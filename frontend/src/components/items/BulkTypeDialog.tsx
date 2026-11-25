import { useState } from "react"
import { useTranslation } from "react-i18next"

import { FieldError } from "@/components/FieldError"
import { ItemTypeField } from "@/components/itemTypes/ItemTypeField"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Label } from "@/components/ui/label"

// The type the server refused for a bulk change, with the reason to show.
export interface BulkTypeError {
  slug: string
  message: string
}

export interface BulkTypeDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** How many items the change applies to. */
  count: number
  /** An item type slug of the group. */
  onSubmit: (type: string) => Promise<void>
  isPending?: boolean
  // Shown on the field while the refused type stays selected.
  error?: BulkTypeError | null
}

// Sets the type of several items, picked from the group's item types with
// the edit form's type field.
export function BulkTypeDialog(props: BulkTypeDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      {props.open ? <BulkTypeForm {...props} /> : null}
    </Dialog>
  )
}

function BulkTypeForm({
  onOpenChange,
  count,
  onSubmit,
  isPending = false,
  error,
}: BulkTypeDialogProps) {
  const { t } = useTranslation()
  const [type, setType] = useState("")
  const errorMessage = error && error.slug === type ? error.message : undefined

  return (
    <DialogContent className="sm:max-w-md" data-testid="bulk-type-dialog">
      <DialogHeader>
        <DialogTitle>{t("commodities:bulk.typeTitle", { count })}</DialogTitle>
        <DialogDescription>{t("commodities:bulk.typeDescription")}</DialogDescription>
      </DialogHeader>
      <form
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault()
          if (type) void onSubmit(type)
        }}
      >
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="bulk-type-select">{t("commodities:fields.type")}</Label>
          <ItemTypeField
            id="bulk-type-select"
            value={type}
            onChange={setType}
            invalid={!!errorMessage}
            errorMessage={errorMessage}
            ariaDescribedBy={errorMessage ? "bulk-type-error" : undefined}
          />
          <FieldError id="bulk-type-error" testId="bulk-type-error" message={errorMessage} />
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t("common:actions.cancel")}
          </Button>
          <Button type="submit" disabled={isPending || !type} data-testid="bulk-type-confirm">
            {t("commodities:bulk.typeConfirm")}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  )
}
