import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"

interface FieldErrorProps {
  // Field error message. By convention this is an i18n key produced by a
  // zod schema (e.g. "auth:validation.emailRequired") — FieldError resolves
  // it through `t()` so call sites don't repeat the translation. Renders
  // nothing when undefined / null / empty, so you can pass
  // `errors.<field>?.message` straight through.
  message?: string | null
  // Element id so the matching input can wire `aria-describedby={id}` and
  // assistive tech announces this message when the field gets focus. RHF's
  // default `shouldFocusError` moves focus to the first invalid field on
  // submit, so the described-by text is read out without a `role="alert"`
  // (which would otherwise double-announce on mount).
  id?: string
  // Stable test hook (e.g. "profile-name-error"). Pair it with the input's
  // `aria-describedby` for a11y and with e2e selectors that prefer ids over
  // copy matching.
  testId?: string
  className?: string
}

// FieldError — the single blessed rendering for an inline form-field
// validation error. Replaces the ad-hoc
// `<p className="text-xs text-destructive">{t(error.message)}</p>` repeated
// across every form so the color token, sizing, and the `.field-error`
// hook stay identical app-wide. The `field-error` class is intentionally
// part of the base: the e2e suite (`e2e/tests/profile.spec.ts`) locates
// field errors by that class, so baking it in gives every adopting form
// the same selector for free. See `devdocs/frontend/forms.md` for the
// field shape this slots into.
export function FieldError({ message, id, testId, className }: FieldErrorProps) {
  const { t } = useTranslation()
  if (!message) return null
  return (
    <p
      id={id}
      data-testid={testId}
      className={cn("field-error text-xs text-destructive", className)}
    >
      {t(message)}
    </p>
  )
}
