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
import { lendFormSchema, type LendFormInput } from "@/features/loans/schemas"

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
      <DialogContent className="sm:max-w-md" data-testid="lend-dialog">
        <DialogHeader>
          <DialogTitle>{t("loans:dialog.title")}</DialogTitle>
          <DialogDescription>{t("loans:dialog.description")}</DialogDescription>
        </DialogHeader>

        <form
          className="flex flex-col gap-4"
          onSubmit={handleSubmit(async (values) => {
            await onSubmit({
              borrower_name: values.borrower_name,
              borrower_contact: values.borrower_contact ?? "",
              borrower_note: values.borrower_note ?? "",
              lent_at: values.lent_at,
              due_back_at: values.due_back_at ?? "",
            })
          })}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lend-borrower-name">{t("loans:dialog.borrowerName")}</Label>
            <Input
              id="lend-borrower-name"
              data-testid="lend-borrower-name"
              placeholder={t("loans:dialog.borrowerNamePlaceholder")}
              autoComplete="off"
              {...register("borrower_name")}
            />
            {errors.borrower_name?.message ? (
              <p className="text-xs text-destructive" data-testid="lend-borrower-name-error">
                {t(errors.borrower_name.message)}
              </p>
            ) : null}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lend-borrower-contact">{t("loans:dialog.borrowerContact")}</Label>
            <Input
              id="lend-borrower-contact"
              data-testid="lend-borrower-contact"
              placeholder={t("loans:dialog.borrowerContactPlaceholder")}
              autoComplete="off"
              {...register("borrower_contact")}
            />
            <p className="text-xs text-muted-foreground">{t("loans:dialog.borrowerContactHint")}</p>
            {errors.borrower_contact?.message ? (
              <p className="text-xs text-destructive" data-testid="lend-borrower-contact-error">
                {t(errors.borrower_contact.message)}
              </p>
            ) : null}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="lend-lent-at">{t("loans:dialog.lentAt")}</Label>
              <Input
                id="lend-lent-at"
                type="date"
                data-testid="lend-lent-at"
                {...register("lent_at")}
              />
              {errors.lent_at?.message ? (
                <p className="text-xs text-destructive" data-testid="lend-lent-at-error">
                  {t(errors.lent_at.message)}
                </p>
              ) : null}
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="lend-due-back-at">{t("loans:dialog.dueBackAt")}</Label>
              <Input
                id="lend-due-back-at"
                type="date"
                data-testid="lend-due-back-at"
                {...register("due_back_at")}
              />
              {errors.due_back_at?.message ? (
                <p className="text-xs text-destructive" data-testid="lend-due-back-at-error">
                  {t(errors.due_back_at.message)}
                </p>
              ) : null}
            </div>
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="lend-borrower-note">{t("loans:dialog.borrowerNote")}</Label>
            <Input
              id="lend-borrower-note"
              data-testid="lend-borrower-note"
              placeholder={t("loans:dialog.borrowerNotePlaceholder")}
              autoComplete="off"
              {...register("borrower_note")}
            />
            {errors.borrower_note?.message ? (
              <p className="text-xs text-destructive" data-testid="lend-borrower-note-error">
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
