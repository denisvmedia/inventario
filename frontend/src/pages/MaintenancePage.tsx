import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate, useSearchParams } from "react-router-dom"
import { Wrench } from "lucide-react"

import { Button } from "@/components/ui/button"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { formatDateTime } from "@/lib/intl"

// Stable status order so the row sequence on the page doesn't shuffle when
// the BE rotates the X-Maintenance-Status header values. The mock
// (design-mocks/src/views/EmptyStatesView.tsx L242-L258) renders API,
// Database, File storage in that exact order.
const COMPONENT_ORDER: ReadonlyArray<"api" | "database" | "storage"> = [
  "api",
  "database",
  "storage",
]

type StatusKey = "operational" | "degraded" | "maintenance"

function parseStatusHeader(raw: string | null): Record<string, StatusKey> {
  // X-Maintenance-Status: "api=degraded,database=maintenance,storage=operational"
  // Unknown component keys are dropped; unknown status values fall through to
  // "maintenance" so the user sees the most conservative read of the
  // operator's intent rather than a deceptive "Operational" pill.
  if (!raw) return {}
  const out: Record<string, StatusKey> = {}
  for (const part of raw.split(",")) {
    const [key, value] = part.split("=").map((s) => s.trim().toLowerCase())
    if (!key || !value) continue
    const normalized: StatusKey =
      value === "operational" || value === "degraded" || value === "maintenance"
        ? value
        : "maintenance"
    out[key] = normalized
  }
  return out
}

function parseRetryAfter(raw: string | null): Date | null {
  // RFC 9110: Retry-After is either an HTTP-date or a delta-seconds value.
  // Parse both. Invalid input → null so the page hides the "expected to
  // resume" line rather than rendering "Invalid Date".
  if (!raw) return null
  const trimmed = raw.trim()
  if (!trimmed) return null
  // delta-seconds is purely digits.
  if (/^\d+$/.test(trimmed)) {
    const seconds = Number.parseInt(trimmed, 10)
    if (!Number.isFinite(seconds)) return null
    return new Date(Date.now() + seconds * 1000)
  }
  const parsed = new Date(trimmed)
  return Number.isNaN(parsed.getTime()) ? null : parsed
}

// MaintenancePage — full-screen 503 state served when the API or storage is
// down for scheduled maintenance. The HTTP client (lib/http.ts) bounces here
// on a 503 with Retry-After + X-Maintenance-Status carried as URL params so
// a refresh (or a deep-link share) keeps showing the page.
//
// Visual matches design-mocks/src/views/EmptyStatesView.tsx::MaintenanceView
// (wrench tile, scheduled-maintenance pill, status card listing API /
// Database / File storage, footer line with the local resume time).
export function MaintenancePage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [params] = useSearchParams()
  const [now, setNow] = useState(() => Date.now())

  // Tick once every 15s so the "expected to resume" countdown stays honest
  // without burning CPU. Stops once the resume time has passed so we don't
  // re-render forever on a forgotten tab.
  useEffect(() => {
    const interval = window.setInterval(() => setNow(Date.now()), 15_000)
    return () => window.clearInterval(interval)
  }, [])

  const retryAt = useMemo(() => parseRetryAfter(params.get("retry_after")), [params])
  const componentStatus = useMemo(() => parseStatusHeader(params.get("status")), [params])
  const hasResumePassed = retryAt ? retryAt.getTime() < now : false

  function handleRetry() {
    // A fresh navigation re-runs the HTTP probes; if the API is back up the
    // 503 handler stops bouncing and the user lands on their previous page.
    navigate("/")
  }

  return (
    <>
      <RouteTitle title={t("errors:maintenance.documentTitle")} />
      <div
        className="flex min-h-screen flex-1 flex-col items-center justify-center gap-6 px-6 py-24 text-center"
        data-testid="page-maintenance"
      >
        <div className="relative flex size-32 items-center justify-center">
          <div className="absolute size-32 rounded-full bg-muted/60" aria-hidden="true" />
          <div className="absolute size-20 rounded-full bg-muted" aria-hidden="true" />
          <Wrench className="relative size-10 text-muted-foreground/60" aria-hidden="true" />
        </div>
        <div className="max-w-sm space-y-2">
          <div
            className="inline-flex items-center gap-1.5 rounded-full border border-border bg-muted px-3 py-1 text-xs font-medium text-muted-foreground"
            data-testid="maintenance-badge"
          >
            <span
              className="size-1.5 animate-pulse rounded-full bg-status-expiring"
              aria-hidden="true"
            />
            {t("errors:maintenance.badge")}
          </div>
          <h1 className="scroll-m-20 text-2xl font-bold tracking-tight">
            {t("errors:maintenance.heading")}
          </h1>
          <p className="text-sm leading-relaxed text-muted-foreground">
            {t("errors:maintenance.description")}
          </p>
        </div>

        <div
          className="w-full max-w-xs space-y-3 rounded-xl border border-border bg-card px-5 py-4 text-left"
          data-testid="maintenance-status-card"
        >
          <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
            {t("errors:maintenance.statusHeading")}
          </p>
          {COMPONENT_ORDER.map((key) => {
            const status: StatusKey = componentStatus[key] ?? "maintenance"
            const statusColor =
              status === "operational"
                ? "text-status-active"
                : status === "degraded"
                  ? "text-status-expiring"
                  : "text-muted-foreground"
            return (
              <div
                key={key}
                className="flex items-center justify-between"
                data-testid={`maintenance-status-${key}`}
              >
                <span className="text-sm">{t(`errors:maintenance.components.${key}`)}</span>
                <span className={`text-xs font-medium ${statusColor}`}>
                  {t(`errors:maintenance.statusLabels.${status}`)}
                </span>
              </div>
            )
          })}
        </div>

        {retryAt && !hasResumePassed ? (
          <p className="text-xs text-muted-foreground" data-testid="maintenance-resume">
            {t("errors:maintenance.resumeAt", { time: formatDateTime(retryAt) })}
          </p>
        ) : null}

        <Button onClick={handleRetry} data-testid="maintenance-retry">
          {t("errors:maintenance.retry")}
        </Button>
      </div>
    </>
  )
}
