import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Navigate, useNavigate } from "react-router-dom"
import { AlertTriangle, Building2, Loader2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { createCommodity } from "@/features/commodities/api"
import { clearDraft, readDraft, toRequest, uploadPendingFiles } from "@/features/commodities/draft"
import {
  clearPendingFirstItem,
  consumePendingFirstItem,
  savePendingFirstItem,
  type PendingFirstItem,
} from "@/features/auth/firstItemHandoff"
import { useCreateGroup, useGroups } from "@/features/group/hooks"
import type { LocationGroup } from "@/features/group/api"
import { clearPendingFiles, loadPendingFiles } from "@/lib/pending-files-store"
import { setCurrentGroupSlug } from "@/lib/group-context"

// FirstItemResolver runs after an anonymous visitor logs in, replaying the
// item they drafted on the landing page into a real group (#1988).
//
// The hard requirement is NO DATA LOSS. The flow:
//
//   1. consume the pending-first-item marker once (ref-stable so a
//      re-render can't double-consume). No marker → bounce to "/".
//   2. resolve a target group:
//        - >1 groups → in-page picker (the user chooses where it lands)
//        - exactly 1 → use it silently
//        - 0 groups   → create a "Main" group seeded with the stashed
//          currency
//   3. with the target slug pinned on the http client, read the stashed
//      draft, POST the commodity, upload its IndexedDB pending files,
//      and ONLY THEN clear the draft + files + marker.
//   4. navigate to the created item's detail page.
//
// Failure handling: any error in step 3 re-saves the marker (it was
// already consumed) and surfaces a Retry button — the draft + pending
// files are never cleared on a failed attempt, so nothing is lost. A
// "marker present but draft empty/missing" (stale — e.g. the user cleared
// site data) falls through to "/" without an error.
type Phase = "resolving" | "picking" | "submitting" | "error"

export function FirstItemResolver() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { data: groups, isLoading: groupsLoading, isError: groupsError } = useGroups()
  const createGroup = useCreateGroup()

  // Consume the marker exactly once via a lazy state initializer — runs on
  // the first render only, so a re-render (groups query settling, etc.)
  // never re-reads/re-clears it. `setPending` is used by the retry button
  // to re-consume the marker it re-saved on failure.
  const [pending, setPending] = useState<PendingFirstItem | null>(() => consumePendingFirstItem())

  const [phase, setPhase] = useState<Phase>("resolving")
  // Guards the replay so it fires exactly once even though `groups`
  // settling re-renders the component. A ref (read/written only inside the
  // effect, never during render) is the right tool — it doesn't need to
  // trigger a re-render. Retry resets it.
  const startedRef = useRef(false)
  const [errorMessage, setErrorMessage] = useState<string | null>(null)

  // replay performs steps 3-4 against the chosen group. Pulled out so both
  // the auto-resolve path (0/1 group) and the picker path call into it.
  const replay = useCallback(
    async (group: LocationGroup) => {
      if (!pending) return
      if (!group.slug) {
        // A slug-less group can't be a POST target (would build "/g/").
        // Treat as a soft failure with retry rather than losing data.
        savePendingFirstItem(pending)
        setErrorMessage(t("landing:resolver.errorGeneric"))
        setPhase("error")
        return
      }
      setPhase("submitting")
      // Pin the slug on the http client so createCommodity's "/commodities"
      // POST rewrites to "/g/{slug}/commodities".
      setCurrentGroupSlug(group.slug)
      try {
        const draft = readDraft(pending.draftKey)
        // Stale marker: the draft was cleared between sessions (site-data
        // wipe). Nothing to replay — clear leftovers and fall through.
        if (!draft || !draft.name) {
          clearDraft(pending.draftKey)
          void clearPendingFiles(pending.draftKey)
          clearPendingFirstItem()
          navigate("/", { replace: true })
          return
        }
        const request = toRequest(
          // readDraft returns Partial<CommodityFormInput>; the dialog only
          // ever persists a full input, so the cast is safe. A genuinely
          // partial draft still produces a request — the BE validates it
          // and any 422 surfaces via the catch below (data preserved).
          draft as Parameters<typeof toRequest>[0],
          group.group_currency ?? pending.currency
        )
        const created = await createCommodity(request)
        // The generated Commodity type marks `id` optional; createCommodity
        // sets it from data.id, but guard so a malformed response is treated
        // as a recoverable failure (data preserved) rather than building a
        // broken "/commodities/undefined" URL.
        const createdId = created.id
        if (!createdId) {
          savePendingFirstItem(pending)
          setErrorMessage(t("landing:resolver.errorGeneric"))
          setPhase("error")
          return
        }
        // Upload + link the staged files. Per-file failures are tolerated
        // (the commodity already exists) — we log them but don't abort the
        // success path, mirroring the dialog's fire-and-forget upload.
        const files = await loadPendingFiles(pending.draftKey)
        if (files.length > 0) {
          await uploadPendingFiles(files, createdId, (entry, err) => {
            console.error("first-item file attach failed", entry.file.name, err)
          })
        }
        // Success — only now is it safe to drop the stash.
        clearDraft(pending.draftKey)
        void clearPendingFiles(pending.draftKey)
        clearPendingFirstItem()
        navigate(
          `/g/${encodeURIComponent(group.slug)}/commodities/${encodeURIComponent(createdId)}`,
          { replace: true }
        )
      } catch (err) {
        // Never lose data: re-stash the marker (the draft + files were
        // never cleared) and offer a retry.
        savePendingFirstItem(pending)
        setErrorMessage(t("landing:resolver.errorGeneric"))
        setPhase("error")
        console.error("first-item replay failed", err)
      }
    },
    [pending, navigate, t]
  )

  // Auto-resolve when the groups query settles. Picker path is entered
  // when there's more than one group. The whole body runs inside an async
  // IIFE so the phase transitions are never synchronous setState in the
  // effect body (which cascades renders) — they all happen after at least
  // one microtask boundary.
  useEffect(() => {
    if (!pending) return
    if (startedRef.current) return
    if (groupsLoading) return
    if (groupsError || !groups) return
    startedRef.current = true
    void (async () => {
      if (groups.length > 1) {
        setPhase("picking")
        return
      }
      if (groups.length === 1) {
        await replay(groups[0])
        return
      }
      // Zero groups — create "Main" seeded with the stashed currency, then
      // replay into it.
      setPhase("submitting")
      try {
        const created = await createGroup.mutateAsync({
          name: t("landing:resolver.defaultGroupName"),
          group_currency: pending.currency,
        })
        await replay(created)
      } catch (err) {
        savePendingFirstItem(pending)
        setErrorMessage(t("landing:resolver.errorGeneric"))
        setPhase("error")
        console.error("first-item group create failed", err)
      }
    })()
  }, [pending, groups, groupsLoading, groupsError, replay, createGroup, t])

  // No marker — nothing to do. Render a redirect (not a side effect) so
  // it commits synchronously without a flash.
  if (!pending) {
    return <Navigate to="/" replace />
  }

  if (phase === "picking" && groups) {
    return (
      <ResolverShell>
        <RouteTitle title={t("landing:resolver.title")} />
        <div className="space-y-4" data-testid="first-item-resolver-picker">
          <div className="space-y-1.5 text-center">
            <h1 className="text-xl font-semibold tracking-tight">
              {t("landing:resolver.pickGroupTitle")}
            </h1>
            <p className="text-sm text-muted-foreground">
              {t("landing:resolver.pickGroupSubtitle")}
            </p>
          </div>
          <div className="space-y-2">
            {groups.map((g) => (
              <button
                key={g.id}
                type="button"
                onClick={() => void replay(g)}
                data-testid="first-item-resolver-group"
                className="flex w-full items-center gap-3 rounded-lg border border-border bg-card px-4 py-3 text-left transition-colors hover:border-primary/40 hover:bg-muted/30"
              >
                <div className="flex size-8 items-center justify-center rounded-lg bg-muted">
                  <Building2 className="size-4 text-muted-foreground" aria-hidden="true" />
                </div>
                <span className="text-sm font-medium">{g.name}</span>
              </button>
            ))}
          </div>
        </div>
      </ResolverShell>
    )
  }

  if (phase === "error") {
    return (
      <ResolverShell>
        <RouteTitle title={t("landing:resolver.title")} />
        <div className="space-y-4 text-center" data-testid="first-item-resolver-error">
          <div className="flex justify-center">
            <div className="flex size-12 items-center justify-center rounded-full bg-destructive/10">
              <AlertTriangle className="size-6 text-destructive" aria-hidden="true" />
            </div>
          </div>
          <div className="space-y-1.5">
            <h1 className="text-xl font-semibold tracking-tight">
              {t("landing:resolver.errorTitle")}
            </h1>
            <p className="text-sm text-muted-foreground">
              {errorMessage ?? t("landing:resolver.errorGeneric")}
            </p>
          </div>
          <div className="flex justify-center gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => navigate("/", { replace: true })}
              data-testid="first-item-resolver-skip"
            >
              {t("landing:resolver.skip")}
            </Button>
            <Button
              type="button"
              onClick={() => {
                // Retry: re-consume the (re-saved) marker and restart the
                // auto-resolve. Reset the once-guard so the effect re-runs.
                setErrorMessage(null)
                startedRef.current = false
                setPending(consumePendingFirstItem())
                setPhase("resolving")
              }}
              data-testid="first-item-resolver-retry"
            >
              {t("landing:resolver.retry")}
            </Button>
          </div>
        </div>
      </ResolverShell>
    )
  }

  // resolving / submitting — minimal spinner.
  return (
    <ResolverShell>
      <RouteTitle title={t("landing:resolver.title")} />
      <div
        className="flex flex-col items-center gap-4 text-center"
        data-testid="first-item-resolver-loading"
      >
        <Loader2 className="size-8 animate-spin text-muted-foreground" aria-hidden="true" />
        <p className="text-sm text-muted-foreground">{t("landing:resolver.settingUp")}</p>
      </div>
    </ResolverShell>
  )
}

// ResolverShell centers the resolver's transient surfaces. The resolver
// renders inside the authenticated Shell (group-exempt route), so this is
// just a centered content block, not a full-screen takeover.
function ResolverShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-1 flex-col items-center justify-center px-6 py-16">
      <div className="w-full max-w-md">{children}</div>
    </div>
  )
}
