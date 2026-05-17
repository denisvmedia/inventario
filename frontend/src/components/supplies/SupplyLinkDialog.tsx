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
import { Textarea } from "@/components/ui/textarea"
import {
  supplyLinkFormSchema,
  type SupplyLinkFormInput,
  type SupplyLinkFormValues,
} from "@/features/supplies/schemas"
import type { SupplyLinkEntity } from "@/features/supplies/api"

export type { SupplyLinkFormValues } from "@/features/supplies/schemas"

interface SupplyLinkDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  // initial is provided when editing an existing link; absent means
  // "create flow" — the form opens empty.
  initial?: SupplyLinkEntity & { id: string }
  onSubmit: (values: SupplyLinkFormValues) => Promise<void> | void
  busy?: boolean
}

const emptyDefaults: SupplyLinkFormInput = {
  label: "",
  url: "",
  notes: "",
}

// SupplyLinkDialog is the shared create/edit modal for a single
// supply link (#1369). The component is intentionally dumb — parent
// owns the mutation, this just collects fields and validates them
// against the same shape the BE enforces.
export function SupplyLinkDialog({
  open,
  onOpenChange,
  title,
  initial,
  onSubmit,
  busy = false,
}: SupplyLinkDialogProps) {
  const { t } = useTranslation(["supplies", "common"])
  const {
    formState: { errors, isSubmitting },
    handleSubmit,
    register,
    reset,
  } = useForm<SupplyLinkFormInput>({
    resolver: zodResolver(supplyLinkFormSchema),
    defaultValues: emptyDefaults,
  })

  // Reset on every open so a previous submission's values don't bleed
  // across re-opens, and so switching from "edit row A" to "edit row
  // B" reloads the right initial data.
  useEffect(() => {
    if (!open) return
    if (initial) {
      reset({
        label: initial.label ?? "",
        url: initial.url ?? "",
        notes: initial.notes ?? "",
      })
    } else {
      reset(emptyDefaults)
    }
  }, [open, initial, reset])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md" data-testid="supply-link-dialog">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          <DialogDescription>
            {t("supplies:dialog.description", {
              defaultValue: "Label the consumable and paste the link where you re-buy it.",
            })}
          </DialogDescription>
        </DialogHeader>

        <form
          className="flex flex-col gap-4"
          noValidate
          onSubmit={handleSubmit(async (values) => {
            await onSubmit({
              label: values.label.trim(),
              url: values.url.trim(),
              notes: values.notes ?? "",
            })
          })}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="supply-label">
              {t("supplies:fields.label", { defaultValue: "Label" })}
            </Label>
            <Input
              id="supply-label"
              autoFocus
              placeholder={t("supplies:fields.labelPlaceholder", {
                defaultValue: "e.g. Water filter",
              })}
              data-testid="supply-link-label-input"
              {...register("label")}
            />
            {errors.label ? (
              <p
                className="text-xs text-destructive"
                data-testid="supply-link-label-error"
              >
                {errors.label.message}
              </p>
            ) : null}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="supply-url">
              {t("supplies:fields.url", { defaultValue: "URL" })}
            </Label>
            <Input
              id="supply-url"
              type="url"
              placeholder="https://example.com/refill"
              data-testid="supply-link-url-input"
              {...register("url")}
            />
            {errors.url ? (
              <p
                className="text-xs text-destructive"
                data-testid="supply-link-url-error"
              >
                {errors.url.message}
              </p>
            ) : null}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="supply-notes">
              {t("supplies:fields.notes", { defaultValue: "Notes (optional)" })}
            </Label>
            <Textarea
              id="supply-notes"
              rows={3}
              placeholder={t("supplies:fields.notesPlaceholder", {
                defaultValue: "Pack size, refill cadence, etc.",
              })}
              data-testid="supply-link-notes-input"
              {...register("notes")}
            />
            {errors.notes ? (
              <p
                className="text-xs text-destructive"
                data-testid="supply-link-notes-error"
              >
                {errors.notes.message}
              </p>
            ) : null}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => onOpenChange(false)}
              disabled={busy || isSubmitting}
            >
              {t("supplies:dialog.cancel", { defaultValue: "Cancel" })}
            </Button>
            <Button
              type="submit"
              disabled={busy || isSubmitting}
              data-testid="supply-link-submit"
            >
              {t("supplies:dialog.save", { defaultValue: "Save" })}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
