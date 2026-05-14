import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import {
  AlertTriangle,
  ArrowLeft,
  CheckCircle2,
  KeyRound,
  Lock,
  Mail,
  ShieldAlert,
  XCircle,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { RouteTitle } from "@/components/routing/RouteTitle"
import type { LoginEventView } from "@/features/login-history/api"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useLoginHistory } from "@/features/login-history/hooks"
import { cn } from "@/lib/utils"
import { withGroupQuery } from "@/lib/group-aware-url"

// LoginHistoryPage renders /profile/login-history (issue #1379) — a
// reverse-chronological list of credential-check attempts with an
// optional "we noticed N failed attempts" banner if the BE reports
// more than three failures in the last seven days.
const FAILED_ATTEMPTS_BANNER_THRESHOLD = 3

export function LoginHistoryPage() {
  const { t, i18n } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const query = useLoginHistory(100)
  const locale = i18n.resolvedLanguage ?? "en"

  const events = query.data?.events ?? []
  const failedLast7d = query.data?.failed_last_7d ?? 0

  return (
    <>
      <RouteTitle title={t("settings:loginHistory.title")} />
      <div
        className="mx-auto flex w-full max-w-2xl flex-col gap-6"
        data-testid="login-history-page"
      >
        <div className="space-y-1">
          <Link
            to={withGroupQuery("/settings", currentGroup?.slug)}
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            {t("settings:privacy.title")}
          </Link>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("settings:loginHistory.title")}
          </h1>
          <p className="text-sm text-muted-foreground">{t("settings:loginHistory.subtitle")}</p>
        </div>

        {failedLast7d > FAILED_ATTEMPTS_BANNER_THRESHOLD ? (
          <div
            className="flex items-start gap-3 rounded-xl border border-destructive/30 bg-destructive/5 p-4"
            data-testid="login-history-failed-banner"
          >
            <ShieldAlert className="size-5 text-destructive shrink-0 mt-0.5" aria-hidden="true" />
            <div>
              <p className="text-sm font-medium text-destructive">
                {t("settings:loginHistory.failedBanner.title", { count: failedLast7d })}
              </p>
              <p className="text-xs text-muted-foreground mt-0.5">
                {t("settings:loginHistory.failedBanner.description")}
              </p>
            </div>
          </div>
        ) : null}

        {query.isLoading ? (
          <div className="rounded-xl border border-border p-6 text-sm text-muted-foreground">
            {t("settings:loginHistory.loading")}
          </div>
        ) : events.length === 0 ? (
          <div className="rounded-xl border border-border p-6 text-sm text-muted-foreground">
            {t("settings:loginHistory.empty")}
          </div>
        ) : (
          <ul
            className="rounded-xl border border-border divide-y divide-border bg-card"
            data-testid="login-history-list"
          >
            {events.map((event) => (
              <LoginEventRow key={event.id} event={event} locale={locale} />
            ))}
          </ul>
        )}
      </div>
    </>
  )
}

interface LoginEventRowProps {
  event: LoginEventView
  locale: string
}

function LoginEventRow({ event, locale }: LoginEventRowProps) {
  const { t } = useTranslation()
  const outcome = event.outcome ?? "ok"
  const cfg = OUTCOME_CONFIG[outcome] ?? OUTCOME_CONFIG.ok
  const OutcomeIcon = cfg.icon
  const relative = formatRelative(event.created_at ?? "", locale)
  const absolute = formatAbsolute(event.created_at ?? "", locale)
  const ua = parseUserAgent(event.user_agent ?? "")
  // Resolve the badge + method labels via a static lookup map (rather
  // than `t(\`settings:loginHistory.outcomes.${outcome}\`)` template
  // literals) so the i18next-cli extractor sees every key statically.
  // Falls back to the "unknown" key whenever the BE introduces a new
  // enum variant the FE hasn't been deployed for yet.
  const outcomeLabel = OUTCOME_I18N_KEY[outcome]
    ? t(OUTCOME_I18N_KEY[outcome])
    : t("settings:loginHistory.outcomes.unknown")
  const methodLabel = event.method && METHOD_I18N_KEY[event.method]
    ? t(METHOD_I18N_KEY[event.method])
    : (event.method ?? null)
  // Suffix the UA label only when the parser actually recognised
  // something. `ua.isUnknown` keeps the conditional locale-agnostic —
  // a string compare against "Unknown device" would break the moment
  // the page is rendered in cs/ru.
  const uaLabel = ua.isUnknown
    ? null
    : `${ua.browser ?? t("settings:loginHistory.ua.unknownBrowser")} · ${ua.os ?? t("settings:loginHistory.ua.unknownOs")}`

  return (
    <li
      className="flex items-start gap-3 p-4"
      data-testid="login-history-row"
      data-outcome={outcome}
    >
      <div
        className={cn(
          "flex size-9 items-center justify-center rounded-lg shrink-0",
          cfg.bg
        )}
      >
        <OutcomeIcon className={cn("size-4", cfg.color)} aria-hidden="true" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <Badge
            variant="outline"
            className={cn("text-xs", cfg.color, cfg.bg, "border-current/20 font-medium")}
          >
            {outcomeLabel}
          </Badge>
          {methodLabel ? (
            <span className="text-xs text-muted-foreground">{methodLabel}</span>
          ) : null}
        </div>
        <p className="mt-1 text-sm" title={absolute}>
          {relative}
          <span className="text-xs text-muted-foreground"> · {absolute}</span>
        </p>
        <p className="text-xs text-muted-foreground">
          {event.ip_address ? event.ip_address : t("settings:loginHistory.unknownIp")}
          {uaLabel ? ` · ${uaLabel}` : ""}
        </p>
      </div>
    </li>
  )
}

// Static lookup so the i18next-cli extractor can see each key. Keep in
// sync with go/models/login_event.go LoginOutcome and LoginMethod enums.
const OUTCOME_I18N_KEY: Record<string, string> = {
  ok: "settings:loginHistory.outcomes.ok",
  bad_password: "settings:loginHistory.outcomes.bad_password",
  account_locked: "settings:loginHistory.outcomes.account_locked",
  account_disabled: "settings:loginHistory.outcomes.account_disabled",
  email_not_verified: "settings:loginHistory.outcomes.email_not_verified",
}

const METHOD_I18N_KEY: Record<string, string> = {
  password: "settings:loginHistory.methods.password",
  oauth_google: "settings:loginHistory.methods.oauth_google",
  oauth_github: "settings:loginHistory.methods.oauth_github",
}

type OutcomeKey =
  | "ok"
  | "bad_password"
  | "account_locked"
  | "account_disabled"
  | "email_not_verified"

interface OutcomeConfig {
  icon: typeof CheckCircle2
  color: string
  bg: string
}

const OUTCOME_CONFIG: Record<OutcomeKey, OutcomeConfig> = {
  ok: { icon: CheckCircle2, color: "text-status-active", bg: "bg-status-active/10" },
  bad_password: { icon: XCircle, color: "text-destructive", bg: "bg-destructive/10" },
  account_locked: { icon: Lock, color: "text-destructive", bg: "bg-destructive/10" },
  account_disabled: { icon: AlertTriangle, color: "text-status-expiring", bg: "bg-status-expiring/10" },
  email_not_verified: { icon: Mail, color: "text-status-expiring", bg: "bg-status-expiring/10" },
}

// Minimal UA parse — same shape as SessionsPage uses but pared down to
// just the browser + os pair (the row doesn't need a device icon).
// Returns a structured shape so the caller localizes the fallback
// strings (see review #1674); the parser stays i18n-free for testing.
interface ParsedUA {
  browser: string | null
  os: string | null
  isUnknown: boolean
}

function parseUserAgent(ua: string): ParsedUA {
  if (!ua) return { browser: null, os: null, isUnknown: true }
  const browser = matchFirst(ua, [
    [/Edg\/\d+/, "Edge"],
    [/OPR\/\d+/, "Opera"],
    [/Chrome\/\d+/, "Chrome"],
    [/Safari\/\d+/, "Safari"],
    [/Firefox\/\d+/, "Firefox"],
  ])
  const os = matchFirst(ua, [
    [/Windows NT/, "Windows"],
    [/Mac OS X/, "macOS"],
    [/iPhone OS/, "iOS"],
    [/Android/, "Android"],
    [/Linux/, "Linux"],
  ])
  return { browser, os, isUnknown: !browser && !os }
}

function matchFirst(ua: string, table: Array<[RegExp, string]>): string | null {
  for (const [re, label] of table) {
    if (re.test(ua)) return label
  }
  return null
}

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

// Avoid unused-icon warnings for icons that may be needed when new
// outcome values are added (e.g. KeyRound is reserved for "oauth_*" methods
// shipping with #1394). Keeping the import while the enum stabilizes.
void KeyRound
