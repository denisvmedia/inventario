import { zodResolver } from "@hookform/resolvers/zod"
import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { FieldError } from "@/components/FieldError"
import { lendFormSchema, type LendFormInput } from "@/features/loans/schemas"
import { applyServerFieldErrors } from "@/lib/form-errors"

// LendSubmitValues normalises the form output to the shape callers
// actually want — every field non-undefined string — so they don't
// have to thread `?? ""` through every call site. The schema's
// `.optional().default("")` already coerces undefined → "" at parse
// time; this type just narrows the surface.
export interface LendSubmitValues {
  borrower_name: string
  borrower_contact: string
  borrower_note: string
  lent_at: string
  due_back_at: string
}

export interface LendDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  // The dialog is single-purpose: open a brand-new loan. Editing an
  // existing loan uses a separate flow (per-row "edit" buttons in the
  // history table) — this keeps the dialog code small and the
  // happy-path UX (lend, fill borrower, submit) free of mode toggles.
  onSubmit: (values: LendSubmitValues) => Promise<void>
  isPending?: boolean
}

function todayISO(): string {
  // Local-date YYYY-MM-DD — Lend defaults to "today as the user sees
  // it" rather than UTC. Matches the BE's MarkReturned default which
  // also reads server-local time.
  const d = new Date()
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, "0")
  const day = String(d.getDate()).padStart(2, "0")
  return `${y}-${m}-${day}`
}

const buildDefaults = (): LendFormInput => ({
  borrower_name: "",
  borrower_contact: "",
  borrower_note: "",
  lent_at: todayISO(),
  due_back_at: "",
})

export function LendDialog({ open, onOpenChange, onSubmit, isPending = false }: LendDialogProps) {
  const { t } = useTranslation(["loans", "common"])
  const {
    formState: { errors, isSubmitting },
    handleSubmit,
    register,
    reset,
    setError,
  } = useForm<LendFormInput>({
    resolver: zodResolver(lendFormSchema),
    defaultValues: buildDefaults(),
  })

  // Reset on open so a previous "lend" submission doesn't leave stale
  // values in the form when the user re-opens the dialog later.
  useEffect(() => {
    if (open) reset(buildDefaults())
  }, [open, reset])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent variant="fullscreen-mobile" className="sm:max-w-md" data-testid="lend-dialog">
        <DialogHeader>
          <DialogTitle>{t("loans:dialog.title")}</DialogTitle>
          <DialogDescription>{t("loans:dialog.description")}</DialogDescription>
        </DialogHeader>

        <form
          className="flex flex-col gap-4"
          // noValidate: zod owns validation; matches EditLoanDialog so
          // webkit's HTML5 validator can't silently block submission
          // when a <input type="date"> has an empty/edge value.
          noValidate
          onSubmit={handleSubmit(async (values) => {
            try {
              await onSubmit({
                borrower_name: values.borrower_name,
                borrower_contact: values.borrower_contact ?? "",
                borrower_note: values.borrower_note ?? "",
                lent_at: values.lent_at,
                due_back_at: values.due_back_at ?? "",
              })
            } catch (err) {
              // Host toasts a summary and re-throws; map field-level 422s.
              applyServerFieldErrors(err, setError, {
                fields: [
                  "borrower_name",
                  "borrower_contact",
                  "borrower_note",
                  "lent_at",
                  "due_back_at",
                ],
              })
            }
          })}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lend-borrower-name">{t("loans:dialog.borrowerName")}</Label>
            <Input
              id="lend-borrower-name"
              data-testid="lend-borrower-name"
              placeholder={t("loans:dialog.borrowerNamePlaceholder")}
              autoComplete="off"
              aria-invalid={!!errors.borrower_name}
              aria-describedby={
                errors.borrower_name?.message ? "lend-borrower-name-error" : undefined
              }
              {...register("borrower_name")}
            />
            <FieldError
              id="lend-borrower-name-error"
              testId="lend-borrower-name-error"
              message={errors.borrower_name?.message}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lend-borrower-contact">{t("loans:dialog.borrowerContact")}</Label>
            <Input
              id="lend-borrower-contact"
              data-testid="lend-borrower-contact"
              placeholder={t("loans:dialog.borrowerContactPlaceholder")}
              autoComplete="off"
              aria-invalid={!!errors.borrower_contact}
              aria-describedby={
                errors.borrower_contact?.message ? "lend-borrower-contact-error" : undefined
              }
              {...register("borrower_contact")}
            />
            <p className="text-xs text-muted-foreground">{t("loans:dialog.borrowerContactHint")}</p>
            <FieldError
              id="lend-borrower-contact-error"
              testId="lend-borrower-contact-error"
              message={errors.borrower_contact?.message}
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="lend-lent-at">{t("loans:dialog.lentAt")}</Label>
              <Input
                id="lend-lent-at"
                type="date"
                data-testid="lend-lent-at"
                aria-invalid={!!errors.lent_at}
                aria-describedby={errors.lent_at?.message ? "lend-lent-at-error" : undefined}
                {...register("lent_at")}
              />
              <FieldError
                id="lend-lent-at-error"
                testId="lend-lent-at-error"
                message={errors.lent_at?.message}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="lend-due-back-at">{t("loans:dialog.dueBackAt")}</Label>
              <Input
                id="lend-due-back-at"
                type="date"
                data-testid="lend-due-back-at"
                aria-invalid={!!errors.due_back_at}
                aria-describedby={
                  errors.due_back_at?.message ? "lend-due-back-at-error" : undefined
                }
                {...register("due_back_at")}
              />
              <FieldError
                id="lend-due-back-at-error"
                testId="lend-due-back-at-error"
                message={errors.due_back_at?.message}
              />
            </div>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lend-borrower-note">{t("loans:dialog.borrowerNote")}</Label>
            <Input
              id="lend-borrower-note"
              data-testid="lend-borrower-note"
              placeholder={t("loans:dialog.borrowerNotePlaceholder")}
              autoComplete="off"
              aria-invalid={!!errors.borrower_note}
              aria-describedby={
                errors.borrower_note?.message ? "lend-borrower-note-error" : undefined
              }
              {...register("borrower_note")}
            />
            <FieldError
              id="lend-borrower-note-error"
              testId="lend-borrower-note-error"
              message={errors.borrower_note?.message}
            />
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isSubmitting || isPending}
              data-testid="lend-cancel"
            >
              {t("loans:dialog.cancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting || isPending} data-testid="lend-submit">
              {t("loans:dialog.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
