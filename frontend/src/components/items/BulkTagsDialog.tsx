import { useState } from "react"
import { useTranslation } from "react-i18next"

import { TagPalette } from "@/components/tags/TagPalette"
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

export interface BulkTagsChange {
  /** Tag slugs, or the text of a tag the group does not have yet. */
  add: string[]
  /** Tag slugs. */
  remove: string[]
}

export interface BulkTagsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** How many items the change applies to. */
  count: number
  onSubmit: (change: BulkTagsChange) => Promise<void>
  isPending?: boolean
}

// Adds tags to and removes tags from several items. Tags the user picks in
// neither palette stay on each item as they are; a tag cannot be in both.
export function BulkTagsDialog(props: BulkTagsDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      {/* Mounted per open, so each opening starts with empty lists. */}
      {props.open ? <BulkTagsForm {...props} /> : null}
    </Dialog>
  )
}

function BulkTagsForm({ onOpenChange, count, onSubmit, isPending = false }: BulkTagsDialogProps) {
  const { t } = useTranslation()
  const [add, setAdd] = useState<string[]>([])
  const [remove, setRemove] = useState<string[]>([])

  // Picking a tag on one side takes it off the other.
  function changeAdd(next: string[]) {
    setAdd(next)
    setRemove((r) => r.filter((slug) => !next.includes(slug)))
  }
  function changeRemove(next: string[]) {
    setRemove(next)
    setAdd((a) => a.filter((slug) => !next.includes(slug)))
  }

  return (
    <DialogContent className="sm:max-w-lg" data-testid="bulk-tags-dialog">
      <DialogHeader>
        <DialogTitle>{t("commodities:bulk.tagsTitle", { count })}</DialogTitle>
        <DialogDescription>{t("commodities:bulk.tagsDescription")}</DialogDescription>
      </DialogHeader>
      <form
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault()
          if (add.length === 0 && remove.length === 0) return
          void onSubmit({ add, remove })
        }}
      >
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="bulk-tags-add-input">{t("commodities:bulk.tagsAdd")}</Label>
          <TagPalette
            kind="commodity"
            values={add}
            onChange={changeAdd}
            inputId="bulk-tags-add-input"
            testId="bulk-tags-add"
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <span className="text-sm leading-none font-medium">
            {t("commodities:bulk.tagsRemove")}
          </span>
          <TagPalette
            kind="commodity"
            values={remove}
            onChange={changeRemove}
            inputId="bulk-tags-remove-input"
            allowCreate={false}
            testId="bulk-tags-remove"
          />
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => onOpenChange(false)}>
            {t("common:actions.cancel")}
          </Button>
          <Button
            type="submit"
            disabled={isPending || (add.length === 0 && remove.length === 0)}
            data-testid="bulk-tags-confirm"
          >
            {t("commodities:bulk.tagsConfirm")}
          </Button>
        </DialogFooter>
      </form>
    </DialogContent>
  )
}
