import { useId, useState } from "react"
import { useTranslation } from "react-i18next"
import { Trash2, Unlink } from "lucide-react"

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import type { DeleteStrategy } from "@/features/areas/api"

export type DeleteContainerKind = "area" | "location"

interface DeleteWithItemsDialogProps {
  open: boolean
  // Which container is being deleted — drives all the copy (the keys
  // live in the `locations` namespace under `deleteWithItems.*`).
  kind: DeleteContainerKind
  // Display name of the container, shown in the title.
  name: string
  // How many items live inside (directly for an area; transitively for a
  // location). Used to size the "delete everything" warning.
  itemCount: number
  // For a location only: how many areas it holds. Cascade deletes them
  // outright; unlink also removes them (the items survive, un-located).
  areaCount?: number
  isPending?: boolean
  // Resolves with the chosen strategy, or `null` when the user cancels
  // (button, Escape, click-outside). The host kicks off the matching
  // delete mutation on a non-null value and closes the dialog itself.
  onResolve: (strategy: DeleteStrategy | null) => void
}

// #2137 — the dedicated three-way delete prompt shown when a NON-EMPTY
// area/location is deleted. Unlike `useConfirm()` (binary), this offers a
// real choice between cascade (delete the items + their files) and unlink
// (keep the items, un-assign them, drop just the container). No upstream
// design mock exists for this surface — logged in
// `devdocs/frontend/design-deviations.md`. It reuses the shadcn `Dialog`
// + `RadioGroup` primitives so the look matches the rest of the app.
export function DeleteWithItemsDialog({
  open,
  kind,
  name,
  itemCount,
  areaCount = 0,
  isPending = false,
  onResolve,
}: DeleteWithItemsDialogProps) {
  const { t } = useTranslation()
  const groupId = useId()
  const [strategy, setStrategy] = useState<DeleteStrategy>("unlink")

  // Reset to the safe (unlink) default each time the dialog re-opens so a
  // previous "cascade" pick doesn't pre-arm the destructive option for
  // the next container. Done as a render-phase "adjust state on prop
  // change" (the React-recommended alternative to a setState-in-effect)
  // keyed off the open edge.
  const [wasOpen, setWasOpen] = useState(open)
  if (open !== wasOpen) {
    setWasOpen(open)
    if (open) setStrategy("unlink")
  }

  const cascadeId = `${groupId}-cascade`
  const unlinkId = `${groupId}-unlink`

  return (
    <Dialog
      open={open}
      onOpenChange={(next) => {
        if (!next && !isPending) onResolve(null)
      }}
    >
      <DialogContent data-testid="delete-with-items-dialog">
        <DialogHeader>
          <DialogTitle>{t(`locations:deleteWithItems.${kind}Title`, { name })}</DialogTitle>
          <DialogDescription>
            {t(`locations:deleteWithItems.${kind}Description`, { count: itemCount })}
          </DialogDescription>
        </DialogHeader>

        <RadioGroup
          value={strategy}
          onValueChange={(value) => setStrategy(value as DeleteStrategy)}
          className="gap-3"
          data-testid="delete-with-items-strategy"
        >
          <Label
            htmlFor={unlinkId}
            className="flex cursor-pointer items-start gap-3 rounded-lg border border-input p-3 transition-colors has-[[data-state=checked]]:border-primary has-[[data-state=checked]]:bg-primary/5"
          >
            <RadioGroupItem
              id={unlinkId}
              value="unlink"
              className="mt-0.5"
              data-testid="delete-with-items-unlink"
            />
            <span className="flex flex-1 flex-col gap-0.5">
              <span className="flex items-center gap-2 text-sm font-medium">
                <Unlink className="size-4 text-muted-foreground" aria-hidden="true" />
                {t(`locations:deleteWithItems.${kind}UnlinkLabel`)}
              </span>
              <span className="text-sm text-muted-foreground">
                {t(`locations:deleteWithItems.${kind}UnlinkHelp`, { count: areaCount })}
              </span>
            </span>
          </Label>

          <Label
            htmlFor={cascadeId}
            className="flex cursor-pointer items-start gap-3 rounded-lg border border-input p-3 transition-colors has-[[data-state=checked]]:border-destructive has-[[data-state=checked]]:bg-destructive/5"
          >
            <RadioGroupItem
              id={cascadeId}
              value="cascade"
              className="mt-0.5"
              data-testid="delete-with-items-cascade"
            />
            <span className="flex flex-1 flex-col gap-0.5">
              <span className="flex items-center gap-2 text-sm font-medium">
                <Trash2 className="size-4 text-destructive" aria-hidden="true" />
                {t("locations:deleteWithItems.cascadeLabel")}
              </span>
              <span className="text-sm text-muted-foreground">
                {t("locations:deleteWithItems.cascadeHelp", { count: itemCount })}
              </span>
            </span>
          </Label>
        </RadioGroup>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => onResolve(null)}
            disabled={isPending}
            data-testid="delete-with-items-cancel"
          >
            {t("common:actions.cancel")}
          </Button>
          <Button
            type="button"
            variant={strategy === "cascade" ? "destructive" : "default"}
            onClick={() => onResolve(strategy)}
            disabled={isPending}
            data-testid="delete-with-items-confirm"
          >
            {t("common:actions.delete")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
