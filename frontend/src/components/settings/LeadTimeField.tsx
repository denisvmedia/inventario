import { useRef, useState, type KeyboardEvent } from "react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { formatLeadTimes, validateLeadTimes } from "@/features/settings/leadTimes"

interface LeadTimeFieldProps {
  id: string
  // The values in effect: the stored ones, or the defaults when unset.
  value: number[]
  defaults: number[]
  isCustom: boolean
  multiple: boolean
  max: number
  disabled?: boolean
  // null resets the setting to the default.
  onSave: (value: number[] | null) => void
  testId: string
}

// LeadTimeField is the "days before" input under a reminder row in
// Settings → Notifications (#2805). It saves on blur or Enter; an empty
// field or "Use default" goes back to the default. A saved or rolled-back
// value resets the draft. The field stays mounted through that, so a
// keyboard user keeps focus in it.
export function LeadTimeField({
  id,
  value,
  defaults,
  isCustom,
  multiple,
  max,
  disabled,
  onSave,
  testId,
}: LeadTimeFieldProps) {
  const { t } = useTranslation()
  const current = formatLeadTimes(value)
  const [draft, setDraft] = useState(current)
  const [error, setError] = useState<string | null>(null)
  // The value in effect when the draft was last reset. A save, a rollback or
  // the server default arriving changes it, and the draft follows.
  const [shown, setShown] = useState(current)
  if (shown !== current) {
    setShown(current)
    setDraft(current)
    setError(null)
  }
  const inputRef = useRef<HTMLInputElement>(null)
  const hintId = `${id}-hint`

  function commit() {
    if (draft.trim() === current) {
      setDraft(current)
      setError(null)
      return
    }
    const result = validateLeadTimes(draft, { multiple, max })
    if (!result.ok) {
      setError(t(result.errorKey, result.params))
      return
    }
    setError(null)
    if (result.value === null) {
      setDraft(formatLeadTimes(defaults))
      if (isCustom) onSave(null)
      return
    }
    setDraft(formatLeadTimes(result.value))
    if (formatLeadTimes(result.value) !== current) onSave(result.value)
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter") {
      e.preventDefault()
      commit()
    }
  }

  return (
    <div className="mt-3 space-y-1" data-testid={testId}>
      <div className="flex flex-wrap items-center gap-2">
        <Label htmlFor={id} className="text-xs font-normal text-muted-foreground">
          {t("settings:notifications.leadTimes.label")}
        </Label>
        <Input
          ref={inputRef}
          id={id}
          inputMode="numeric"
          autoComplete="off"
          className="h-8 w-32 text-sm"
          value={draft}
          disabled={disabled}
          aria-invalid={error ? true : undefined}
          aria-describedby={hintId}
          onChange={(e) => {
            setDraft(e.target.value)
            if (error) setError(null)
          }}
          onBlur={commit}
          onKeyDown={onKeyDown}
          data-testid={`${testId}-input`}
        />
        {isCustom && (
          <Button
            type="button"
            variant="link"
            size="sm"
            className="h-auto px-0 text-xs"
            disabled={disabled}
            onClick={() => {
              setError(null)
              setDraft(formatLeadTimes(defaults))
              onSave(null)
              // The button goes away once the setting is back to the
              // default; keep keyboard focus in the field.
              inputRef.current?.focus()
            }}
            data-testid={`${testId}-reset`}
          >
            {t("settings:notifications.leadTimes.reset", { value: formatLeadTimes(defaults) })}
          </Button>
        )}
      </div>
      <p
        id={hintId}
        className={error ? "text-xs text-destructive" : "text-xs text-muted-foreground"}
        role={error ? "alert" : undefined}
      >
        {error ??
          (multiple
            ? t("settings:notifications.leadTimes.hintList", { max })
            : t("settings:notifications.leadTimes.hintSingle", { max }))}
      </p>
    </div>
  )
}
