import { useEffect, useState } from "react"
import { LogOut, UserCog } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { useOptionalImpersonation } from "@/features/admin/impersonation/ImpersonationContext"
import { cn } from "@/lib/utils"

// Seconds remaining until `expiresAt`, clamped at zero. Returns null when
// there is no expiry timestamp (the countdown is then simply not shown).
function secondsUntil(expiresAt: string | null): number | null {
  if (!expiresAt) return null
  const diffMs = new Date(expiresAt).getTime() - Date.now()
  if (Number.isNaN(diffMs)) return null
  return Math.max(0, Math.floor(diffMs / 1000))
}

function formatMMSS(totalSeconds: number): string {
  const safe = Math.max(0, totalSeconds)
  const mm = String(Math.floor(safe / 60)).padStart(2, "0")
  const ss = String(safe % 60).padStart(2, "0")
  return `${mm}:${ss}`
}

// ImpersonationBanner is the persistent, non-dismissible top banner shown
// while the current browser is inside an admin impersonation session. It
// is mounted at the top of Shell and renders nothing unless the
// ImpersonationContext flag is `active`.
//
// Replicates design-mocks/src/views/admin/ImpersonationBannerMock.tsx.
// The mock drove its countdown from a `durationSeconds` prop; here the
// countdown is derived from the session's `expires_at` so it stays
// truthful across reloads. The "End impersonation" button is a shell —
// full BE wiring of POST /admin/impersonation/end lands in a later
// sub-issue (#1750 shipped the BE primitive).
export function ImpersonationBanner() {
  const { t } = useTranslation("admin")
  const impersonation = useOptionalImpersonation()
  const expiresAt = impersonation?.expiresAt ?? null

  // A once-per-second tick drives a re-render; `remaining` itself is
  // derived at render time from the absolute expiry timestamp (a
  // wall-clock diff, not a decrementing counter) so a backgrounded tab or
  // a reload never drifts the displayed time. Deriving rather than
  // storing keeps setState out of the effect body.
  const [, setTick] = useState(0)
  useEffect(() => {
    if (!expiresAt) return
    const interval = setInterval(() => setTick((value) => value + 1), 1000)
    return () => clearInterval(interval)
  }, [expiresAt])

  if (!impersonation?.active) return null

  const remaining = secondsUntil(expiresAt)

  const name = impersonation.targetUser?.name?.trim() || impersonation.targetUser?.email || ""
  const email = impersonation.targetUser?.email ?? ""
  const low = remaining !== null && remaining <= 60

  return (
    <div
      data-testid="impersonation-banner"
      className="flex h-10 items-center gap-3 border-b border-accent/40 bg-accent/15 px-4 text-sm"
    >
      <div className="flex size-6 items-center justify-center rounded-md bg-accent text-accent-foreground shrink-0">
        <UserCog className="size-3.5" />
      </div>
      <div className="flex min-w-0 flex-1 items-baseline gap-2">
        <span className="font-semibold text-foreground truncate">
          {t("impersonation.banner.label", { name })}
        </span>
        {email ? (
          <span className="hidden text-xs text-muted-foreground truncate sm:inline">{email}</span>
        ) : null}
      </div>
      {remaining !== null ? (
        <div className="flex items-center gap-1.5 shrink-0">
          <span className="hidden text-xs text-muted-foreground sm:inline">
            {t("impersonation.banner.sessionEndsIn")}
          </span>
          <span
            className={cn(
              "font-mono text-xs font-semibold tabular-nums",
              low ? "text-status-expired" : "text-foreground"
            )}
          >
            {formatMMSS(remaining)}
          </span>
        </div>
      ) : null}
      {/* "End impersonation" is a shell — POST /admin/impersonation/end
          wiring lands in a later sub-issue. The button is rendered
          disabled so the chrome is complete without claiming a working
          action it can't yet perform. */}
      <Button
        size="xs"
        variant="outline"
        disabled
        className="gap-1.5 shrink-0 border-accent-foreground/20 bg-background"
      >
        <LogOut className="size-3" />
        {t("impersonation.banner.end")}
      </Button>
    </div>
  )
}
