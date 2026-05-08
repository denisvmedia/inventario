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
import { editLoanFormSchema, type EditLoanFormInput } from "@/features/loans/schemas"
import type { LoanEntity, UpdateLoanRequest } from "@/features/loans/api"
import { formatDate } from "@/lib/intl"

// Edit a loan's mutable fields. The dialog is intentionally separate
// from LendDialog (which is create-only) — sharing the form would
// re-introduce the mode-toggle complexity LendDialog's comment
// explicitly avoids. Issue #1513 added the "clear due date"
// affordance: explicit JSON null on the wire maps to "open-ended
// loan, no return date." `lent_at` is rendered read-only because the
// BE rejects mutations to it (audit-trail confusion — see UpdateLoan
// in commodity_loan_service.go).
export interface EditLoanDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  loan: (LoanEntity & { id: string }) | null
  // The handler diffs the form output against the original loan and
  // builds the tri-state PATCH (absent / null / value) — the dialog
  // doesn't need to know the tri-state semantics, only the diff.
  onSubmit: (patch: UpdateLoanRequest) => Promise<void>
  isPending?: boolean
}

function buildDefaults(loan: (LoanEntity & { id: string }) | null): EditLoanFormInput {
  return {
    borrower_name: loan?.borrower_name ?? "",
    borrower_contact: loan?.borrower_contact ?? "",
    borrower_note: loan?.borrower_note ?? "",
    due_back_at: (loan?.due_back_at as string | undefined) ?? "",
  }
}

// buildPatch turns the form output + original loan into a sparse
// PATCH body. Each field is only included when it differs from the
// original; due_back_at uses `null` to encode an explicit clear,
// matching the BE's presence-aware decoder (issue #1513).
function buildPatch(
  loan: LoanEntity & { id: string },
  values: {
    borrower_name: string
    borrower_contact: string
    borrower_note: string
    due_back_at: string
  }
): UpdateLoanRequest {
  const patch: UpdateLoanRequest = {}
  if (values.borrower_name !== (loan.borrower_name ?? "")) {
    patch.borrower_name = values.borrower_name
  }
  if (values.borrower_contact !== (loan.borrower_contact ?? "")) {
    patch.borrower_contact = values.borrower_contact
  }
  if (values.borrower_note !== (loan.borrower_note ?? "")) {
    patch.borrower_note = values.borrower_note
  }
  const original = (loan.due_back_at as string | undefined) ?? ""
  if (values.due_back_at !== original) {
    patch.due_back_at = values.due_back_at === "" ? null : values.due_back_at
  }
  return patch
}

export function EditLoanDialog({
  open,
  onOpenChange,
  loan,
  onSubmit,
  isPending = false,
}: EditLoanDialogProps) {
  const { t } = useTranslation(["loans", "common"])
  const {
    formState: { errors, isSubmitting },
    handleSubmit,
    register,
    reset,
    setValue,
    watch,
  } = useForm<EditLoanFormInput>({
    resolver: zodResolver(editLoanFormSchema),
    defaultValues: buildDefaults(loan),
  })

  // Repopulate when (re)opened so the form mirrors the most recent
  // loan snapshot — list invalidations between edits would otherwise
  // leave a stale value in the dialog.
  useEffect(() => {
    if (open) reset(buildDefaults(loan))
  }, [open, loan, reset])

  const dueBackAt = watch("due_back_at")

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md" data-testid="edit-loan-dialog">
        <DialogHeader>
          <DialogTitle>{t("loans:editDialog.title")}</DialogTitle>
          <DialogDescription>{t("loans:editDialog.description")}</DialogDescription>
        </DialogHeader>

        <form
          className="flex flex-col gap-4"
          // noValidate: zod owns validation. Without this, webkit's
          // native HTML5 validation runs first and silently blocks
          // submission of <input type="date"> after Clear (the empty
          // value trips its constraint), so handleSubmit never fires
          // and the dialog stays open.
          noValidate
          onSubmit={handleSubmit(async (values) => {
            if (!loan) return
            const patch = buildPatch(loan, {
              borrower_name: values.borrower_name,
              borrower_contact: values.borrower_contact ?? "",
              borrower_note: values.borrower_note ?? "",
              due_back_at: values.due_back_at ?? "",
            })
            await onSubmit(patch)
          })}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="edit-loan-borrower-name">{t("loans:dialog.borrowerName")}</Label>
            <Input
              id="edit-loan-borrower-name"
              data-testid="edit-loan-borrower-name"
              autoComplete="off"
              {...register("borrower_name")}
            />
            {errors.borrower_name?.message ? (
              <p className="text-xs text-destructive" data-testid="edit-loan-borrower-name-error">
                {t(errors.borrower_name.message)}
              </p>
            ) : null}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="edit-loan-borrower-contact">{t("loans:dialog.borrowerContact")}</Label>
            <Input
              id="edit-loan-borrower-contact"
              data-testid="edit-loan-borrower-contact"
              autoComplete="off"
              {...register("borrower_contact")}
            />
            <p className="text-xs text-muted-foreground">{t("loans:dialog.borrowerContactHint")}</p>
            {errors.borrower_contact?.message ? (
              <p
                className="text-xs text-destructive"
                data-testid="edit-loan-borrower-contact-error"
              >
                {t(errors.borrower_contact.message)}
              </p>
            ) : null}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <Label>{t("loans:dialog.lentAt")}</Label>
              {/* lent_at is read-only — see comment above. */}
              <p className="text-sm" data-testid="edit-loan-lent-at-readonly">
                {loan?.lent_at ? formatDate(loan.lent_at as string) : "—"}
              </p>
            </div>
            <div className="flex flex-col gap-1.5">
              <div className="flex items-baseline justify-between gap-2">
                <Label htmlFor="edit-loan-due-back-at">{t("loans:dialog.dueBackAt")}</Label>
                {dueBackAt ? (
                  <button
                    type="button"
                    className="text-xs text-muted-foreground underline-offset-2 hover:underline"
                    onClick={(e) => {
                      // Blur first: setValue unmounts this button (gated
                      // on `dueBackAt`). Without blur, webkit slides
                      // focus to the now-empty <input type="date"> as
                      // the next focusable sibling, and the date
                      // input's native focus side-effects swallow the
                      // very next click (notably the Submit button in
                      // e2e: the form's onSubmit never fires and the
                      // dialog stays open). chromium/firefox tolerate
                      // the focus jump; webkit doesn't.
                      ;(e.currentTarget as HTMLButtonElement).blur()
                      setValue("due_back_at", "", { shouldDirty: true })
                    }}
                    data-testid="edit-loan-clear-due-back"
                  >
                    {t("loans:editDialog.clearDueBack")}
                  </button>
                ) : null}
              </div>
              <Input
                id="edit-loan-due-back-at"
                type="date"
                data-testid="edit-loan-due-back-at"
                {...register("due_back_at")}
              />
              {errors.due_back_at?.message ? (
                <p className="text-xs text-destructive" data-testid="edit-loan-due-back-at-error">
                  {t(errors.due_back_at.message)}
                </p>
              ) : null}
            </div>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="edit-loan-borrower-note">{t("loans:dialog.borrowerNote")}</Label>
            <Input
              id="edit-loan-borrower-note"
              data-testid="edit-loan-borrower-note"
              autoComplete="off"
              {...register("borrower_note")}
            />
            {errors.borrower_note?.message ? (
              <p className="text-xs text-destructive" data-testid="edit-loan-borrower-note-error">
                {t(errors.borrower_note.message)}
              </p>
            ) : null}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isSubmitting || isPending}
              data-testid="edit-loan-cancel"
            >
              {t("loans:dialog.cancel")}
            </Button>
            <Button
              type="submit"
              disabled={isSubmitting || isPending}
              data-testid="edit-loan-submit"
            >
              {t("loans:editDialog.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
