import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { ArrowLeft, LogOut, Shield } from "lucide-react"

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Page, PageHeader } from "@/components/ui/page"
import { RouteTitle } from "@/components/routing/RouteTitle"
import type { SessionView } from "@/features/sessions/api"
import { useCurrentGroup } from "@/features/group/GroupContext"
import {
  useRevokeAllOtherSessions,
  useRevokeSession,
  useSessionsList,
} from "@/features/sessions/hooks"
import { parseUserAgent } from "@/features/security/ua"
import { useAppToast } from "@/hooks/useAppToast"
import { withGroupQuery } from "@/lib/group-aware-url"
import { formatDateTime, formatRelative } from "@/lib/intl"

// SessionsPage renders the /profile/sessions view from issue #1378.
// Card per session — device icon, browser/OS, last-used, partial IP,
// "This device" pill, Revoke button. Plus a top-level
// "Sign out all other sessions" CTA that confirms before firing.
export function SessionsPage() {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const toast = useAppToast()
  const sessionsQuery = useSessionsList()
  const revokeMutation = useRevokeSession()
  const revokeAllOthersMutation = useRevokeAllOtherSessions()

  // Pending dialogs — we confirm both single revoke and revoke-all so a
  // misclick never wipes the user's other browsers.
  const [pendingRevoke, setPendingRevoke] = useState<SessionView | null>(null)
  const [revokeAllOpen, setRevokeAllOpen] = useState(false)

  // Memoise the array reference so the `otherSessionsCount` dep array
  // stays stable across re-renders that don't change the query data —
  // without this, `sessions` is a fresh `[]` whenever sessionsQuery.data
  // is still undefined and the deps trip exhaustive-deps.
  const sessions = useMemo(() => sessionsQuery.data?.sessions ?? [], [sessionsQuery.data?.sessions])
  const otherSessionsCount = useMemo(() => sessions.filter((s) => !s.is_current).length, [sessions])

  const confirmRevoke = (session: SessionView) => setPendingRevoke(session)

  const doRevoke = () => {
    if (!pendingRevoke?.id) return
    const id = pendingRevoke.id
    revokeMutation.mutate(id, {
      onSuccess: () => {
        toast.success(t("settings:sessions.toasts.revoked"))
        setPendingRevoke(null)
      },
      onError: () => {
        toast.error(t("settings:sessions.toasts.revokeFailed"))
      },
    })
  }

  const doRevokeAllOthers = () => {
    // Resolve the id of the session the BE should preserve. The list
    // endpoint flags exactly one row with `is_current: true`; we hand
    // that id off to the mutation so the BE knows which refresh-token
    // row to keep alive. Without it the BE wipes every session
    // because the refresh cookie is path-scoped to /api/v1/auth and
    // isn't sent on this route (issue surfaced via the
    // sessions-and-login-history e2e cleanup branch).
    const currentSessionId = sessions.find((s) => s.is_current)?.id
    revokeAllOthersMutation.mutate(currentSessionId, {
      onSuccess: () => {
        toast.success(t("settings:sessions.toasts.allRevoked"))
        setRevokeAllOpen(false)
      },
      onError: () => {
        toast.error(t("settings:sessions.toasts.revokeFailed"))
      },
    })
  }

  return (
    <>
      <RouteTitle title={t("settings:sessions.title")} />
      <Page width="narrow" data-testid="sessions-page">
        <PageHeader
          size="detail"
          title={t("settings:sessions.title")}
          subtitle={t("settings:sessions.subtitle")}
          backLink={
            <Link
              to={withGroupQuery("/settings", currentGroup?.slug)}
              className="inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground transition-colors"
            >
              <ArrowLeft className="size-4" aria-hidden="true" />
              {t("settings:privacy.title")}
            </Link>
          }
        />

        {otherSessionsCount > 0 ? (
          <div className="flex items-center justify-between rounded-xl border border-border bg-card p-4">
            <div className="flex items-center gap-3">
              <div className="flex size-9 items-center justify-center rounded-lg bg-primary/10">
                <Shield className="size-4 text-primary" aria-hidden="true" />
              </div>
              <div>
                <p className="text-sm font-medium">{t("settings:sessions.revokeAll.label")}</p>
                <p className="text-xs text-muted-foreground">
                  {t("settings:sessions.revokeAll.description", { count: otherSessionsCount })}
                </p>
              </div>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() => setRevokeAllOpen(true)}
              data-testid="sessions-revoke-all-btn"
              className="gap-1.5"
            >
              <LogOut className="size-3.5" aria-hidden="true" />
              {t("settings:sessions.revokeAll.cta")}
            </Button>
          </div>
        ) : null}

        {sessionsQuery.isLoading ? (
          <div className="rounded-xl border border-border p-6 text-sm text-muted-foreground">
            {t("settings:sessions.loading")}
          </div>
        ) : sessions.length === 0 ? (
          <div className="rounded-xl border border-border p-6 text-sm text-muted-foreground">
            {t("settings:sessions.empty")}
          </div>
        ) : (
          <div className="space-y-3" data-testid="sessions-list">
            {sessions.map((session) => (
              <SessionCard
                key={session.id}
                session={session}
                onRevoke={confirmRevoke}
                disabled={revokeMutation.isPending}
              />
            ))}
          </div>
        )}
      </Page>

      <Dialog open={!!pendingRevoke} onOpenChange={(o) => !o && setPendingRevoke(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("settings:sessions.confirmRevoke.title")}</DialogTitle>
            <DialogDescription>
              {t("settings:sessions.confirmRevoke.description")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="gap-2">
            <Button
              variant="outline"
              onClick={() => setPendingRevoke(null)}
              disabled={revokeMutation.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              onClick={doRevoke}
              disabled={revokeMutation.isPending}
              variant="destructive"
              data-testid="sessions-confirm-revoke-btn"
            >
              {t("settings:sessions.confirmRevoke.cta")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={revokeAllOpen} onOpenChange={setRevokeAllOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("settings:sessions.confirmRevokeAll.title")}</DialogTitle>
            <DialogDescription>
              {t("settings:sessions.confirmRevokeAll.description")}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="gap-2">
            <Button
              variant="outline"
              onClick={() => setRevokeAllOpen(false)}
              disabled={revokeAllOthersMutation.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              onClick={doRevokeAllOthers}
              disabled={revokeAllOthersMutation.isPending}
              variant="destructive"
              data-testid="sessions-confirm-revoke-all-btn"
            >
              {t("settings:sessions.confirmRevokeAll.cta")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

interface SessionCardProps {
  session: SessionView
  onRevoke: (session: SessionView) => void
  disabled: boolean
}

function SessionCard({ session, onRevoke, disabled }: SessionCardProps) {
  const { t } = useTranslation()
  const ua = parseUserAgent(session.user_agent ?? "")
  const DeviceIcon = ua.deviceIcon
  const lastUsedRelative = formatRelative(session.last_used_at ?? session.created_at ?? "")
  const createdAbsolute = formatDateTime(session.created_at ?? "")
  // Label resolution lives here (not inside parseUserAgent) so we can keep
  // the parser pure and i18n-free and still render localized fallbacks.
  // Mixing the two would force the parser to take a `t` argument, which
  // makes it harder to unit-test.
  const label = ua.isUnknown
    ? t("settings:sessions.ua.unknownDevice")
    : `${ua.browser ?? t("settings:sessions.ua.unknownBrowser")} · ${ua.os ?? t("settings:sessions.ua.unknownOs")}`

  return (
    <div
      className="rounded-xl border border-border bg-card p-4"
      data-testid="session-card"
      data-session-id={session.id}
      data-session-current={session.is_current ? "true" : "false"}
    >
      <div className="flex items-start gap-3">
        <div className="flex size-10 items-center justify-center rounded-lg bg-muted shrink-0">
          <DeviceIcon className="size-5 text-muted-foreground" aria-hidden="true" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-sm font-medium">{label}</p>
            {session.is_current ? (
              <Badge variant="secondary" className="text-xs" data-testid="session-current-pill">
                {t("settings:sessions.thisDevice")}
              </Badge>
            ) : null}
          </div>
          <p className="mt-0.5 text-xs text-muted-foreground">
            {t("settings:sessions.lastUsed", { value: lastUsedRelative })}
            {session.ip_address ? ` · ${session.ip_address}` : ""}
          </p>
          <p className="text-xs text-muted-foreground" title={session.created_at ?? ""}>
            {t("settings:sessions.createdAt", { value: createdAbsolute })}
          </p>
        </div>
        {!session.is_current ? (
          <Button
            variant="outline"
            size="sm"
            disabled={disabled || !session.id}
            onClick={() => onRevoke(session)}
            data-testid="session-revoke-btn"
          >
            {t("settings:sessions.revoke")}
          </Button>
        ) : null}
      </div>
    </div>
  )
}
