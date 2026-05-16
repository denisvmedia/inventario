import { createContext, useContext, useEffect, useMemo, type ReactNode } from "react"
import { useLocation, useNavigate, useParams, useSearchParams } from "react-router-dom"

import { useOptionalAuth } from "@/features/auth/AuthContext"
import { setCurrentGroupSlug } from "@/lib/group-context"

import { useGroups } from "./hooks"
import type { LocationGroup } from "./api"

// Query param that carries the active group on routes whose path doesn't
// already encode it (e.g. /profile, /settings, /groups/new). The path
// shape stays untouched — group context piggybacks on the URL via this
// query key. When the path already encodes the group (/g/:slug or
// /groups/:groupId), the path wins and `?g=` is redundant / ignored.
export const GROUP_QUERY_PARAM = "g"

// Stable non-group routes where the user lingers and where the sidebar
// needs an active-group context to render its inventory rows. Limited to
// pathnames whose route element actually renders content (not a
// `<Navigate />` / `<UngroupedRedirect />` stub) — adding ?g= to a
// redirect-source URL would leak into the redirected URL since
// UngroupedRedirect deliberately preserves search/hash on rewrite.
//
// `/no-group` is intentionally NOT on this list: it's the zero-group
// onboarding state, so by definition there's no active group to pin.
// The transient case (a fresh group just created via the inline form,
// where a refetch surfaces groups while the page is still on /no-group
// before NoGroupPage's `navigate("/")` fires) ALSO doesn't want ?g=:
// the auto-set's setSearchParams replace can race the post-create
// navigation in subtle browser-scheduler-dependent ways and leave the
// user stuck on /no-group?g=<slug> instead of bouncing to /g/<slug>
// (firefox + webkit reproduce, chromium doesn't — see e2e
// no-group-redirect.spec.ts > NoGroupView drives group creation).
const PATHS_WITH_GROUP_QUERY = new Set([
  "/profile",
  "/profile/edit",
  "/settings",
  "/groups/new",
  "/plans",
  "/help",
  "/whats-new",
])

const Context = createContext<GroupContextValue | undefined>(undefined)

interface GroupContextValue {
  // Every group the user belongs to. `undefined` while the list is still
  // loading; `[]` for a user with zero groups.
  groups: LocationGroup[] | undefined
  // The active group resolved from the URL. Resolution order:
  //   1. /g/:groupSlug path param (canonical for inventory pages)
  //   2. /groups/:groupId path param (canonical for admin pages)
  //   3. ?g=<slug> query param (carries context on path-shape-clean
  //      routes like /profile, /settings)
  // `null` until groups are loaded or when the user has zero groups.
  currentGroup: LocationGroup | null
  // True while `useGroups()` is fetching for the first time.
  isLoading: boolean
  // True when the groups query errored out.
  isError: boolean
}

interface GroupProviderProps {
  children: ReactNode
}

// GroupProvider keeps three things in sync:
//
//   1. The /api/v1/g/{slug}/* URL-rewrite slot in `lib/http.ts` — every
//      group-scoped fetch reads the slug from there. Setting it from the
//      URL is what makes two browser tabs at two different groups actually
//      independent (#1289 Gap C).
//
//   2. The "is the URL pointing at a slug I'm a member of?" check — a stale
//      or wrong path slug bounces the user to their first group's home (or
//      to /no-group if they have none).
//
//   3. The active-group hint on path-shape-clean routes (/profile, /settings,
//      /groups/new, …). The path is left untouched; group context is
//      pinned via `?g=<slug>` so the sidebar (and anything else reading
//      currentGroup) keeps a coherent view when the user steps off
//      /g/:slug/*. Auto-filled from the user's default group when missing.
//
// The URL stays the single source of truth: the provider only ever writes
// to the URL via setSearchParams (the replaced-history `?g=` reconciliation
// in case 3) — never to private in-memory state that diverges from it.
export function GroupProvider({ children }: GroupProviderProps) {
  const params = useParams<{ groupSlug?: string; groupId?: string }>()
  const navigate = useNavigate()
  const location = useLocation()
  const [searchParams, setSearchParams] = useSearchParams()
  const auth = useOptionalAuth()
  const user = auth?.user
  const { data: groups, isLoading, isError } = useGroups()

  const slugFromPath = params.groupSlug ?? null
  const idFromPath = params.groupId ?? null
  const slugFromQuery = searchParams.get(GROUP_QUERY_PARAM)

  const currentGroup = useMemo<LocationGroup | null>(() => {
    if (!groups) return null
    if (slugFromPath) return groups.find((g) => g.slug === slugFromPath) ?? null
    if (idFromPath) return groups.find((g) => g.id === idFromPath) ?? null
    if (slugFromQuery) return groups.find((g) => g.slug === slugFromQuery) ?? null
    return null
  }, [slugFromPath, idFromPath, slugFromQuery, groups])

  // Mirror the URL's slug hint into the http client immediately — first
  // paint can dispatch group-scoped queries before `groups` resolves and
  // currentGroup becomes non-null, and an un-rewritten request fails.
  // Path slug wins; query slug fills in for path-clean routes
  // (/profile?g=foo). On /groups/:id/* the URL has no slug, so the slot
  // stays null here and gets corrected below once the id resolves to a
  // group — pages mounted at /groups/:id/* either hit id-keyed endpoints
  // (no rewrite needed) or pre-arm the slot in their own test setup.
  //
  // Note: we deliberately do NOT clear the slot in this effect's cleanup.
  // When the URL slug changes ("household" → "office"), React runs cleanup
  // (slot=null) then the next effect (slot="office"); a request issued in
  // between would skip the rewrite. Idempotent overwrite is race-free.
  const slugHint = slugFromPath ?? slugFromQuery ?? null
  useEffect(() => {
    setCurrentGroupSlug(slugHint)
  }, [slugHint])

  // Once the id-based path resolves a group, correct the slot to the real
  // slug. No-op on slug/query routes — slugHint already covered them.
  useEffect(() => {
    if (!idFromPath || !currentGroup?.slug) return
    setCurrentGroupSlug(currentGroup.slug)
  }, [idFromPath, currentGroup?.slug])

  // Provider-unmount cleanup is the only place where we want a hard clear:
  // if the GroupProvider tree goes away (e.g. logout drops to public routes),
  // future non-group requests should not inherit a stale slug.
  useEffect(() => {
    return () => setCurrentGroupSlug(null)
  }, [])

  // URL-shape reconciliation. Three cases:
  //
  //   1. /g/:slug/* with a slug the user is NOT a member of (revoked
  //      membership, hand-typed, post-rename) → send them to their first
  //      group's home, or to /no-group if they have none. Same fail-safe
  //      as the legacy redirect; preserved in the new scheme.
  //
  //   2. /groups/:id/* with an id the user is not a member of → leave it
  //      alone. The host page (GroupSettingsPage) renders its own 404/error
  //      from the `useGroup(id)` query; we don't second-guess.
  //
  //   3. Path-shape-clean routes (/profile, /settings, /no-group, …) for
  //      a user with ≥1 groups: ensure the active-group context is in the
  //      URL via ?g=<slug>. Without this, the sidebar (which reads
  //      currentGroup) loses its inventory/manage rows the moment the user
  //      navigates off /g/:slug/*. Pick the user's default_group_id when
  //      it resolves; fall back to the first slug-bearing group otherwise.
  //      Replaces the history entry rather than pushing — back-button
  //      shouldn't bounce through a "no-context" version of the page.
  useEffect(() => {
    if (slugFromPath) {
      if (!groups) return
      if (groups.some((g) => g.slug === slugFromPath)) return
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
      return
    }
    if (idFromPath) return
    if (!groups || groups.length === 0) return
    if (slugFromQuery && groups.some((g) => g.slug === slugFromQuery)) return
    if (!PATHS_WITH_GROUP_QUERY.has(location.pathname)) return
    const preferredSlug =
      (user?.default_group_id ? groups.find((g) => g.id === user.default_group_id)?.slug : null) ??
      groups.find((g) => !!g.slug)?.slug
    if (!preferredSlug) return
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        next.set(GROUP_QUERY_PARAM, preferredSlug)
        return next
      },
      { replace: true }
    )
  }, [
    slugFromPath,
    idFromPath,
    slugFromQuery,
    groups,
    user?.default_group_id,
    location.pathname,
    navigate,
    setSearchParams,
  ])

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

// Optional variant that returns undefined when no provider is mounted. Used by
// chrome components (TopBar's GroupRoleCluster) that render in both
// authenticated shells (provider present) and on test surfaces / boot states
// where the provider hasn't been wrapped yet — both cases want the chrome to
// degrade silently rather than crash.
export function useOptionalCurrentGroup(): GroupContextValue | undefined {
  return useContext(Context)
}
