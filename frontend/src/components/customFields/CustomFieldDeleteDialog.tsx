import { useState } from "react"
import { useTranslation } from "react-i18next"

import { ServerErrorBanner } from "@/components/ServerErrorBanner"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"
import { CUSTOM_FIELD_IN_USE, type CustomField } from "@/features/customFields/api"
import { useDeleteCustomField } from "@/features/customFields/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import {
  classifyServerError,
  getServerErrorCode,
  getServerErrorMeta,
  type ClassifiedServerError,
} from "@/lib/server-error"

export interface CustomFieldDeleteDialogProps {
  // The field to delete; null closes the dialog.
  field: CustomField | null
  onClose: () => void
}

interface Usage {
  live: number
  trashed: number
}

interface DialogState {
  open: boolean
  field: CustomField | null
  // The member ticked "Delete the values too".
  confirmed: boolean
  // What the server reported when it refused the delete: values were
  // written after the page loaded its counts.
  refusedUsage: Usage | null
  serverError: ClassifiedServerError | null
}

function freshState(field: CustomField | null): DialogState {
  return { open: field !== null, field, confirmed: false, refusedUsage: null, serverError: null }
}

// CustomFieldDeleteDialog deletes a custom field (#2803). A field that
// items have values for, in Trash included, is deleted only once the member
// confirms that the values go too; the dialog says how many items lose one.
// The mock has no custom fields; the dialog follows ItemTypeDeleteDialog
// with a checkbox in place of the type picker.
export function CustomFieldDeleteDialog({ field, onClose }: CustomFieldDeleteDialogProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const deleteMutation = useDeleteCustomField()
  // The state belongs to one opening, and is kept while the dialog closes
  // so the closing animation still shows the field's name.
  const [state, setState] = useState<DialogState>(() => freshState(field))
  if (field && (!state.open || field.id !== state.field?.id)) {
    setState(freshState(field))
  } else if (!field && state.open) {
    setState((prev) => ({ ...prev, open: false }))
  }
  const { field: shown, confirmed, refusedUsage, serverError } = state
  const usage = refusedUsage ?? {
    live: shown?.item_count ?? 0,
    trashed: shown?.trashed_item_count ?? 0,
  }
  const count = usage.live + usage.trashed
  const inUse = count > 0
  const name = shown?.name ?? ""
  const pending = deleteMutation.isPending

  async function confirm() {
    if (!shown || (inUse && !confirmed)) return
    setState((prev) => ({ ...prev, serverError: null }))
    try {
      // The count the dialog shows: the server refuses the delete when
      // another number of items has a value by now.
      await deleteMutation.mutateAsync({
        id: shown.id,
        deleteValues: inUse,
        expectedValues: inUse ? count : undefined,
      })
      toast.success(t("customFields:toast.deleted"))
      onClose()
    } catch (err) {
      if (getServerErrorCode(err) === CUSTOM_FIELD_IN_USE) {
        const meta = getServerErrorMeta(err)
        setState((prev) => ({
          ...prev,
          confirmed: false,
          refusedUsage: {
            live: Number(meta?.item_count ?? 0) || 0,
            trashed: Number(meta?.trashed_item_count ?? 0) || 0,
          },
        }))
        return
      }
      const error = classifyServerError(err, t("customFields:delete.error"))
      setState((prev) => ({ ...prev, serverError: error }))
    }
  }

  return (
    <Dialog
      open={field !== null}
      onOpenChange={(open) => {
        if (!open && !pending) onClose()
      }}
    >
      <DialogContent className="sm:max-w-md" data-testid="custom-field-delete-dialog">
        <DialogHeader>
          <DialogTitle>{t("customFields:delete.title", { name })}</DialogTitle>
          <DialogDescription data-testid="custom-field-delete-description">
            {inUse
              ? t("customFields:delete.inUse", { count })
              : t("customFields:delete.description")}
            {usage.trashed > 0
              ? ` ${t("customFields:delete.trashed", { count: usage.trashed })}`
              : ""}
          </DialogDescription>
        </DialogHeader>

        {refusedUsage ? (
          <p className="text-sm text-muted-foreground" data-testid="custom-field-delete-refused">
            {t("customFields:delete.changed")}
          </p>
        ) : null}

        {inUse ? (
          <div className="flex items-center gap-2">
            <Checkbox
              id="custom-field-delete-confirm"
              checked={confirmed}
              onCheckedChange={(checked) =>
                setState((prev) => ({ ...prev, confirmed: checked === true }))
              }
              disabled={pending}
              data-testid="custom-field-delete-values"
            />
            <Label htmlFor="custom-field-delete-confirm">
              {t("customFields:delete.confirmLabel")}
            </Label>
          </div>
        ) : null}

        <ServerErrorBanner error={serverError} testId="custom-field-delete-error" />

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            disabled={pending}
            data-testid="custom-field-delete-cancel"
          >
            {t("common:actions.cancel")}
          </Button>
          <Button
            type="button"
            variant="destructive"
            onClick={() => void confirm()}
            disabled={pending || (inUse && !confirmed)}
            data-testid="custom-field-delete-confirm"
          >
            {pending ? t("customFields:delete.deleting") : t("customFields:delete.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
