import { useEffect, useRef, useState } from "react"
import { UserCog, LogOut } from "lucide-react"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

interface ImpersonationBannerMockProps {
  /** Display name of the user currently being impersonated. */
  userName: string
  /** Email of the impersonated user — shown as secondary context. */
  userEmail: string
  /** Total countdown duration in seconds. Defaults to 30 minutes. */
  durationSeconds?: number
  /** Called when the admin ends the impersonated session (button or expiry). */
  onEnd: () => void
}

function formatMMSS(totalSeconds: number): string {
  const safe = Math.max(0, totalSeconds)
  const mm = String(Math.floor(safe / 60)).padStart(2, "0")
  const ss = String(safe % 60).padStart(2, "0")
  return `${mm}:${ss}`
}

/**
 * Persistent, non-dismissible top banner shown while an admin is impersonating
 * a user. Sits above the topbar and counts down a session timer. The only way
 * to dismiss it is the "End impersonation" button (or the timer reaching zero).
 */
export function ImpersonationBannerMock({
  userName,
  userEmail,
  durationSeconds = 30 * 60,
  onEnd,
}: ImpersonationBannerMockProps) {
  const [remaining, setRemaining] = useState(durationSeconds)

  // Keep the latest onEnd in a ref so the mount-once interval below never
  // tears down/recreates when App re-renders (which would drift the timer).
  const onEndRef = useRef(onEnd)
  useEffect(() => {
    onEndRef.current = onEnd
  }, [onEnd])

  // Reset the countdown when a new impersonation session starts.
  useEffect(() => {
    setRemaining(durationSeconds)
  }, [durationSeconds, userName])

  // Mount-once ticking interval — never depends on changing props.
  useEffect(() => {
    const interval = setInterval(() => {
      setRemaining((prev) => (prev <= 0 ? 0 : prev - 1))
    }, 1000)
    return () => clearInterval(interval)
  }, [])

  // End the session when the timer hits zero — done in a separate effect so we
  // never trigger a cross-component update from inside the setState updater.
  useEffect(() => {
    if (remaining === 0) onEndRef.current()
  }, [remaining])

  const low = remaining <= 60

  return (
    <div className="flex h-10 items-center gap-3 border-b border-accent/40 bg-accent/15 px-4 text-sm">
      <div className="flex size-6 items-center justify-center rounded-md bg-accent text-accent-foreground shrink-0">
        <UserCog className="size-3.5" />
      </div>
      <div className="flex min-w-0 flex-1 items-baseline gap-2">
        <span className="font-semibold text-foreground truncate">
          Impersonating {userName}
        </span>
        <span className="hidden text-xs text-muted-foreground truncate sm:inline">
          {userEmail}
        </span>
      </div>
      <div className="flex items-center gap-1.5 shrink-0">
        <span className="text-xs text-muted-foreground hidden sm:inline">Session ends in</span>
        <span
          className={cn(
            "font-mono text-xs font-semibold tabular-nums",
            low ? "text-status-expired" : "text-foreground"
          )}
        >
          {formatMMSS(remaining)}
        </span>
      </div>
      <Button
        size="xs"
        variant="outline"
        className="gap-1.5 shrink-0 border-accent-foreground/20 bg-background"
        onClick={onEnd}
      >
        <LogOut className="size-3" />
        End impersonation
      </Button>
    </div>
  )
}
