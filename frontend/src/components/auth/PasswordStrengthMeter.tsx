import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"

interface PasswordStrengthMeterProps {
  password: string
  // Hint copy under the meter. Defaults to a generic length+complexity nudge.
  hintKey?: string
  // data-testid passthrough; root only — the bars don't get individual ids.
  testId?: string
}

// Returns 0..4. Heuristic only: length is the dominant signal, with extra
// points for character-class mix (lower/upper/digit/symbol). Real entropy
// scoring (zxcvbn) is tracked separately in #1381 — this stub is good
// enough to drive the visual meter without pulling in a 700kb dictionary.
export function scorePassword(password: string): 0 | 1 | 2 | 3 | 4 {
  if (!password) return 0
  let score = 0
  if (password.length >= 8) score++
  if (password.length >= 12) score++
  const classes =
    Number(/[a-z]/.test(password)) +
    Number(/[A-Z]/.test(password)) +
    Number(/[0-9]/.test(password)) +
    Number(/[^a-zA-Z0-9]/.test(password))
  if (classes >= 2) score++
  if (classes >= 3 && password.length >= 10) score++
  return Math.min(score, 4) as 0 | 1 | 2 | 3 | 4
}

const STRENGTH_LABELS = [
  "auth:passwordStrength.empty",
  "auth:passwordStrength.weak",
  "auth:passwordStrength.fair",
  "auth:passwordStrength.good",
  "auth:passwordStrength.strong",
] as const

const BAR_COLORS = [
  "bg-muted",
  "bg-destructive/70",
  "bg-amber-500/70",
  "bg-emerald-500/70",
  "bg-emerald-500",
] as const

export function PasswordStrengthMeter({
  password,
  hintKey = "auth:passwordStrength.hint",
  testId,
}: PasswordStrengthMeterProps) {
  const { t } = useTranslation()
  const score = scorePassword(password)
  const labelText = t(STRENGTH_LABELS[score])
  return (
    <div className="space-y-1.5" data-testid={testId}>
      <div
        className="flex gap-1"
        role="meter"
        aria-label={t("auth:passwordStrength.label")}
        aria-valuemin={0}
        aria-valuemax={4}
        aria-valuenow={score}
        aria-valuetext={labelText}
      >
        {[1, 2, 3, 4].map((i) => (
          <div
            key={i}
            className={cn(
              "h-1 flex-1 rounded-full transition-colors",
              i <= score ? BAR_COLORS[score] : "bg-muted"
            )}
          />
        ))}
      </div>
      <p className="text-xs text-muted-foreground">{password ? labelText : t(hintKey)}</p>
    </div>
  )
}
