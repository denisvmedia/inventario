import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import {
  ArrowLeft,
  Laptop,
  LogOut,
  Monitor,
  Shield,
  Smartphone,
  TabletSmartphone,
} from "lucide-react"

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
import { RouteTitle } from "@/components/routing/RouteTitle"
import type { SessionView } from "@/features/sessions/api"
import { useCurrentGroup } from "@/features/group/GroupContext"
import {
  useRevokeAllOtherSessions,
  useRevokeSession,
  useSessionsList,
} from "@/features/sessions/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { withGroupQuery } from "@/lib/group-aware-url"

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
    revokeAllOthersMutation.mutate(undefined, {
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
      <div className="mx-auto flex w-full max-w-2xl flex-col gap-6" data-testid="sessions-page">
        <div className="space-y-1">
          <Link
            to={withGroupQuery("/settings", currentGroup?.slug)}
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            {t("settings:privacy.title")}
          </Link>
          <h1 className="text-2xl font-semibold tracking-tight">{t("settings:sessions.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("settings:sessions.subtitle")}</p>
        </div>

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
      </div>

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
  const { t, i18n } = useTranslation()
  const ua = parseUserAgent(session.user_agent ?? "")
  const DeviceIcon = ua.deviceIcon
  const locale = i18n.resolvedLanguage ?? "en"
  const lastUsedRelative = formatRelative(session.last_used_at ?? session.created_at ?? "", locale)
  const createdAbsolute = formatAbsolute(session.created_at ?? "", locale)
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

// ParsedUA is what parseUserAgent returns. Keys are intentionally
// non-localized — the consumer pairs them with i18n keys at render time
// (so the unit test can stay i18n-free and the component-level fallback
// can localize "Unknown" labels without re-deriving them here).
interface ParsedUA {
  deviceIcon: typeof Laptop
  browser: string | null
  os: string | null
  isUnknown: boolean
}

// parseUserAgent runs in the browser per #1378 option 2 — keeps the DB
// free of UA strings that age poorly. Cheap regex-based heuristics are
// enough for the FE label: detailed parsing would require a library and
// the cost isn't worth it for a side panel. Returns a structured shape
// so the caller can decide which strings to localize (vs. mixing English
// fallbacks into the parser, which is what review #1674 flagged).
function parseUserAgent(ua: string): ParsedUA {
  if (!ua) return { deviceIcon: Monitor, browser: null, os: null, isUnknown: true }
  const isMobile = /iPhone|Android.*Mobile|Mobile/i.test(ua)
  const isTablet = /iPad|Android(?!.*Mobile)/i.test(ua)
  const browser = matchFirst(ua, [
    [/Edg\/(\d+)/, "Edge"],
    [/OPR\/(\d+)/, "Opera"],
    [/Chrome\/(\d+)/, "Chrome"],
    [/Safari\/(\d+)/, "Safari"],
    [/Firefox\/(\d+)/, "Firefox"],
  ])
  const os = matchFirst(ua, [
    [/Windows NT (\d+\.\d+)/, "Windows"],
    [/Mac OS X (\d+[._]\d+)/, "macOS"],
    [/iPhone OS (\d+[._]\d+)/, "iOS"],
    [/Android (\d+\.\d+)/, "Android"],
    [/Linux/, "Linux"],
  ])
  let icon = Laptop as typeof Laptop
  if (isMobile) icon = Smartphone
  else if (isTablet) icon = TabletSmartphone
  return { deviceIcon: icon, browser, os, isUnknown: !browser && !os }
}

// matchFirst returns the label of the first matching regex; null when
// no pattern matched.
function matchFirst(ua: string, table: Array<[RegExp, string]>): string | null {
  for (const [re, label] of table) {
    if (re.test(ua)) return label
  }
  return null
}

// formatRelative is a tiny Intl.RelativeTimeFormat wrapper. We
// deliberately don't pull in a date-fns dependency here — Intl is
// good enough for this surface.
function formatRelative(iso: string, locale: string): string {
  if (!iso) return ""
  const target = new Date(iso).getTime()
  if (!Number.isFinite(target)) return ""
  const diff = target - Date.now()
  const abs = Math.abs(diff)
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: "auto" })
  const min = 60 * 1000
  const hour = 60 * min
  const day = 24 * hour
  if (abs < min) return rtf.format(Math.round(diff / 1000), "second")
  if (abs < hour) return rtf.format(Math.round(diff / min), "minute")
  if (abs < day) return rtf.format(Math.round(diff / hour), "hour")
  return rtf.format(Math.round(diff / day), "day")
}

function formatAbsolute(iso: string, locale: string): string {
  if (!iso) return ""
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ""
  return new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeStyle: "short" }).format(d)
}
