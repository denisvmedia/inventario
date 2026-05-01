import { createContext, useContext, useEffect, useMemo, type ReactNode } from "react"
import { useNavigate, useParams } from "react-router-dom"

import { setCurrentGroupSlug } from "@/lib/group-context"

import { useGroups } from "./hooks"
import type { LocationGroup } from "./api"

interface GroupContextValue {
  // Every group the user belongs to. `undefined` while the list is still
  // loading; `[]` for a user with zero groups.
  groups: LocationGroup[] | undefined
  // The group whose slug is on the URL right now. `null` for routes outside
  // /g/:groupSlug/* (e.g. /profile, /no-group, /login).
  currentGroup: LocationGroup | null
  // True while `useGroups()` is fetching for the first time.
  isLoading: boolean
  // True when the groups query errored out.
  isError: boolean
}

const Context = createContext<GroupContextValue | undefined>(undefined)

interface GroupProviderProps {
  children: ReactNode
}

// GroupProvider keeps two things in sync:
//
//   1. The /api/v1/g/{slug}/* URL-rewrite slot in `lib/http.ts` — every
//      group-scoped fetch reads the slug from there. Setting it from the
//      router's :groupSlug param is what makes two browser tabs at two
//      different groups actually independent (#1289 Gap C).
//
//   2. The "is the URL pointing at a slug I'm a member of?" check — a stale
//      or wrong slug bounces the user to their first group's home (or to
//      /no-group if they have none).
//
// The URL is the single source of truth; this provider never writes to it
// from in-memory state.
export function GroupProvider({ children }: GroupProviderProps) {
  const params = useParams<{ groupSlug?: string }>()
  const navigate = useNavigate()
  const { data: groups, isLoading, isError } = useGroups()

  const slugFromUrl = params.groupSlug ?? null
  const currentGroup = useMemo<LocationGroup | null>(() => {
    if (!slugFromUrl || !groups) return null
    return groups.find((g) => g.slug === slugFromUrl) ?? null
  }, [slugFromUrl, groups])

  // Mirror the URL slug into the http client. Note: we deliberately do NOT
  // clear the slot in this effect's cleanup — when the URL slug changes
  // ("household" → "office"), React would run cleanup (slot=null) and then
  // the next effect (slot="office"); a request issued in between would skip
  // the rewrite. Setting the slot directly on every change is idempotent and
  // race-free; navigating to a non-group route already passes
  // slugFromUrl=null which clears the slot the same way.
  useEffect(() => {
    setCurrentGroupSlug(slugFromUrl)
  }, [slugFromUrl])

  // Provider-unmount cleanup is the only place where we want a hard clear:
  // if the GroupProvider tree goes away (e.g. logout drops to public routes),
  // future non-group requests should not inherit a stale slug.
  useEffect(() => {
    return () => setCurrentGroupSlug(null)
  }, [])

  // Stale-slug fallback: the URL names a group the user is not a member of
  // (revoked membership, wrong group, hand-typed). Send them to their first
  // group with a usable slug, or to /no-group. Only kicks in once we know
  // the membership list, and only on /g/:slug/* — non-group routes pass
  // through.
  useEffect(() => {
    if (!slugFromUrl || !groups) return
    const known = groups.some((g) => g.slug === slugFromUrl)
    if (known) return
    // The generated LocationGroup type marks `slug` as optional, so guard
    // against a slug-less group ending up as the navigation target — that
    // would build "/g/" and drop into the 404. Treat "no group has a slug"
    // as functionally equivalent to "no groups".
    const firstWithSlug = groups.find((g): g is LocationGroup & { slug: string } => !!g.slug)
    if (!firstWithSlug) {
      navigate("/no-group", { replace: true })
      return
    }
    navigate(`/g/${encodeURIComponent(firstWithSlug.slug)}`, { replace: true })
  }, [slugFromUrl, groups, navigate])

  const value = useMemo<GroupContextValue>(
    () => ({ groups, currentGroup, isLoading, isError }),
    [groups, currentGroup, isLoading, isError]
  )

  return <Context.Provider value={value}>{children}</Context.Provider>
}

// Throws if used outside <GroupProvider>; that's a programming error, not a
// user-facing one, so we don't try to recover.
export function useCurrentGroup(): GroupContextValue {
  const ctx = useContext(Context)
  if (!ctx) throw new Error("useCurrentGroup must be used inside <GroupProvider>")
  return ctx
}
