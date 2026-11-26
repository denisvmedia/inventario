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
import type { ItemSetFieldView, ItemSetItemTypeView, ItemSetView } from "@/features/group/api"

// CollectionSetRemoveDialog confirms removing the Collection set (#1607).
// It lists the types and fields the removal deletes, and those that stay
// with the reason: a member made it, items use it, or it is the group's
// last type. The card checks the status again when the dialog opens, and
// Confirm waits for that check. Confirming sends the rows listed under
// Deleted, and the server deletes no other; one an item started to use
// meanwhile stays too.
interface CollectionSetRemoveDialogProps {
  open: boolean
  view: ItemSetView
  // checking is true while the status is fetched again; checkFailed when
  // that fetch failed. Either keeps Confirm disabled.
  checking: boolean
  checkFailed: boolean
  pending: boolean
  onConfirm: () => void
  onClose: () => void
}

type TFn = ReturnType<typeof useTranslation>["t"]

interface Row {
  id: string
  name: string
  kind: "type" | "field"
  reason?: string
}

export function CollectionSetRemoveDialog({
  open,
  view,
  checking,
  checkFailed,
  pending,
  onConfirm,
  onClose,
}: CollectionSetRemoveDialogProps) {
  const { t } = useTranslation()
  const goes: Row[] = []
  const stays: Row[] = []
  for (const row of view.item_types ?? []) {
    if (!row.item_type_id) continue
    const entry: Row = { id: `type-${row.key}`, name: row.name ?? "", kind: "type" }
    if (row.removable) goes.push(entry)
    else stays.push({ ...entry, reason: typeKeepReason(t, row) })
  }
  for (const row of view.custom_fields ?? []) {
    if (!row.custom_field_id) continue
    const entry: Row = { id: `field-${row.key}`, name: row.name ?? "", kind: "field" }
    if (row.removable) goes.push(entry)
    else stays.push({ ...entry, reason: fieldKeepReason(t, row) })
  }

  return (
    <Dialog open={open} onOpenChange={(next) => (!next && !pending ? onClose() : undefined)}>
      <DialogContent className="sm:max-w-lg" data-testid="collection-set-remove-dialog">
        <DialogHeader>
          <DialogTitle>{t("groups:settings.collectionSet.removal.title")}</DialogTitle>
          <DialogDescription>
            {goes.length > 0
              ? t("groups:settings.collectionSet.removal.description")
              : t("groups:settings.collectionSet.removal.nothing")}
          </DialogDescription>
        </DialogHeader>

        {checking ? (
          <p
            className="text-xs text-muted-foreground"
            role="status"
            data-testid="collection-set-remove-checking"
          >
            {t("groups:settings.collectionSet.removal.checking")}
          </p>
        ) : checkFailed ? (
          <p
            className="text-xs text-destructive"
            role="alert"
            data-testid="collection-set-remove-check-failed"
          >
            {t("groups:settings.collectionSet.removal.checkFailed")}
          </p>
        ) : null}

        <div className="max-h-[50vh] space-y-4 overflow-y-auto">
          {goes.length > 0 ? (
            <section>
              <h3 className="text-sm font-medium">
                {t("groups:settings.collectionSet.removal.goes", { total: goes.length })}
              </h3>
              <ul className="mt-1.5 space-y-1" data-testid="collection-set-remove-goes">
                {goes.map((row) => (
                  <li key={row.id} className="flex items-baseline justify-between gap-3 text-sm">
                    <span className="truncate">{row.name}</span>
                    <span className="shrink-0 text-xs text-muted-foreground">
                      {kindLabel(t, row.kind)}
                    </span>
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          {stays.length > 0 ? (
            <section>
              <h3 className="text-sm font-medium">
                {t("groups:settings.collectionSet.removal.stays", { total: stays.length })}
              </h3>
              <ul className="mt-1.5 space-y-1.5" data-testid="collection-set-remove-stays">
                {stays.map((row) => (
                  <li
                    key={row.id}
                    className="text-sm"
                    data-testid={`collection-set-stays-${row.id}`}
                  >
                    <div className="flex items-baseline justify-between gap-3">
                      <span className="truncate">{row.name}</span>
                      <span className="shrink-0 text-xs text-muted-foreground">
                        {kindLabel(t, row.kind)}
                      </span>
                    </div>
                    <p className="text-xs text-muted-foreground">{row.reason}</p>
                  </li>
                ))}
              </ul>
            </section>
          ) : null}
        </div>

        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            disabled={pending}
            data-testid="collection-set-remove-cancel"
          >
            {goes.length > 0 ? t("common:actions.cancel") : t("common:actions.close")}
          </Button>
          {goes.length > 0 ? (
            <Button
              type="button"
              variant="destructive"
              onClick={onConfirm}
              disabled={pending || checking || checkFailed}
              data-testid="collection-set-remove-confirm"
            >
              {pending
                ? t("groups:settings.collectionSet.removal.removing")
                : t("groups:settings.collectionSet.removal.confirm")}
            </Button>
          ) : null}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function kindLabel(t: TFn, kind: Row["kind"]): string {
  return kind === "type"
    ? t("groups:settings.collectionSet.removal.kindType")
    : t("groups:settings.collectionSet.removal.kindField")
}

function typeKeepReason(t: TFn, row: ItemSetItemTypeView): string {
  switch (row.keep_reason) {
    case "own":
      return t("groups:settings.collectionSet.removal.keepOwnType")
    case "last_type":
      return t("groups:settings.collectionSet.removal.keepLastType")
    case "in_use":
    default:
      return t("groups:settings.collectionSet.removal.keepTypeInUse", {
        count: (row.item_count ?? 0) + (row.trashed_item_count ?? 0),
      })
  }
}

function fieldKeepReason(t: TFn, row: ItemSetFieldView): string {
  switch (row.keep_reason) {
    case "own":
      return t("groups:settings.collectionSet.removal.keepOwnField")
    case "in_use":
    default:
      return t("groups:settings.collectionSet.removal.keepFieldInUse", {
        count: (row.item_count ?? 0) + (row.trashed_item_count ?? 0),
      })
  }
}
