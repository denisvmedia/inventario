import { useState } from "react"
import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { ChevronDown, QrCode, Trash2 } from "lucide-react"

import { BulkResultDialog } from "@/components/items/BulkResultDialog"
import { BulkTypeDialog } from "@/components/items/BulkTypeDialog"
import { StatusTransitionDialog } from "@/components/items/StatusTransitionDialog"
import { BulkTagsDialog, type BulkTagsChange } from "@/components/tags/BulkTagsDialog"
import { BulkActionBar } from "@/components/ui/bulk-action-bar"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Label } from "@/components/ui/label"
import type { Area } from "@/features/areas/api"
import {
  MAX_BULK_QUERY_ITEMS,
  type Commodity,
  type CommodityBulkPatch,
  type CommodityBulkResult,
  type CommodityBulkStatusValue,
  type CommoditySelection,
} from "@/features/commodities/api"
import {
  useBulkDeleteCommodities,
  useBulkMoveCommodities,
  useBulkUpdateCommodities,
} from "@/features/commodities/hooks"
import { commodityHref } from "@/features/commodities/listContext"
import { commodityQueryParam, type CommodityQuery } from "@/features/commodities/query"
import type { CommoditySelectionControls } from "@/features/commodities/selection"
import { labelsPageHref } from "@/features/labels/links"
import { useUndoItemDelete } from "@/features/trash/useUndoItemDelete"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import type { ListContext } from "@/lib/list-context"
import { getServerErrorCode, getServerErrorMeta } from "@/lib/server-error"
import { cn } from "@/lib/utils"
import { withId } from "@/lib/with-id"

// The statuses a bulk change offers. Sold is left out: it needs a sale
// price of its own for each item.
const BULK_STATUSES = ["in_use", "lost", "disposed", "written_off"] as const
type TerminalBulkStatus = Exclude<CommodityBulkStatusValue, "in_use">

type BulkAction =
  | { kind: "update"; patch: CommodityBulkPatch }
  | { kind: "move"; areaId: string }
  | { kind: "delete" }

// The items an action was confirmed for, captured when its dialog opens so
// a list refresh while the dialog is open does not change them.
interface Target {
  selection: CommoditySelection
  count: number
}

type DialogKind = "move" | "tags" | "type" | "status"

export interface CommodityBulkActionsProps {
  selection: CommoditySelectionControls
  /** The rows of the current page. */
  rows: Commodity[]
  /** How many items the query matches, on every page. */
  total: number
  query: CommodityQuery
  slug?: string
  areas: Area[]
  listContext: ListContext | null
  /** A currency migration holds group writes. */
  locked: boolean
}

// The bulk action bar of the Items list and the dialogs behind it: move,
// edit tags, type and status, print labels and delete. Every action runs on
// the server, item by item, with the checks and history of a single change.
// Items that fail are listed with the reason, stay selected, and can be
// retried.
export function CommodityBulkActions({
  selection,
  rows,
  total,
  query,
  slug,
  areas,
  listContext,
  locked,
}: CommodityBulkActionsProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const confirm = useConfirm()
  const undoDelete = useUndoItemDelete()
  const bulkDelete = useBulkDeleteCommodities()
  const bulkMove = useBulkMoveCommodities()
  const bulkUpdate = useBulkUpdateCommodities()

  const [dialog, setDialog] = useState<DialogKind | null>(null)
  const [target, setTarget] = useState<Target | null>(null)
  const [statusTarget, setStatusTarget] = useState<TerminalBulkStatus | null>(null)
  const [moveArea, setMoveArea] = useState("")
  const [editOpen, setEditOpen] = useState(false)
  const [outcome, setOutcome] = useState<{
    action: BulkAction
    result: CommodityBulkResult
  } | null>(null)

  const pageIds = rows.map((r) => r.id ?? "").filter(Boolean)
  const checked = selection.checkedOn(pageIds)
  const pageSelected = pageIds.length > 0 && pageIds.every((id) => checked.has(id))
  const count = selection.count(total)
  const pending = bulkDelete.isPending || bulkMove.isPending || bulkUpdate.isPending

  function currentTarget(): Target {
    return { selection: selection.toRequest(commodityQueryParam(query), total), count }
  }

  function openDialog(kind: DialogKind) {
    setTarget(currentTarget())
    setDialog(kind)
  }

  function execute(action: BulkAction, sel: CommoditySelection): Promise<CommodityBulkResult> {
    switch (action.kind) {
      case "update":
        return bulkUpdate.mutateAsync({ selection: sel, patch: action.patch })
      case "move":
        return bulkMove.mutateAsync({ selection: sel, areaId: action.areaId })
      case "delete":
        return bulkDelete.mutateAsync(sel)
    }
  }

  // run sends the action and reports what it did. It returns false when the
  // request failed as a whole, so the dialog that sent it can stay open.
  async function run(action: BulkAction, sel: CommoditySelection): Promise<boolean> {
    let result: CommodityBulkResult
    try {
      result = await execute(action, sel)
    } catch (err) {
      reportError(action, err)
      return false
    }
    report(action, result)
    return true
  }

  function report(action: BulkAction, result: CommodityBulkResult) {
    const done = result.succeeded.length
    if (action.kind === "delete" && done > 0) {
      toast.success(t("commodities:toast.bulkDeleted", { count: done }), {
        action: undoDelete(result.succeeded),
      })
    }
    if (result.failed.length > 0) {
      setOutcome({ action, result })
      // The failed items stay selected, to retry them or fix them another way.
      selection.replace(result.failed.map((f) => f.id))
      return
    }
    setOutcome(null)
    selection.clear()
    if (action.kind === "delete") return
    if (done === 0) {
      toast.success(t("commodities:toast.bulkUnchanged"))
      return
    }
    const description =
      result.skipped.length > 0
        ? t("commodities:toast.bulkSkipped", { count: result.skipped.length })
        : undefined
    const message =
      action.kind === "move"
        ? t("commodities:toast.bulkMoved", { count: done })
        : t("commodities:toast.bulkUpdated", { count: done })
    toast.success(message, { description })
  }

  function reportError(action: BulkAction, err: unknown) {
    const code = getServerErrorCode(err)
    const meta = getServerErrorMeta(err) ?? {}
    if (code === "bulk.selection_changed") {
      // Sending the same request again would fail the same way: close the
      // dialog so the user looks at the refreshed list first.
      setDialog(null)
      toast.error(
        t("commodities:toast.bulkSelectionChanged", {
          expected: meta.expected ?? "",
          matched: meta.matched ?? "",
        })
      )
      return
    }
    if (code === "bulk.too_many") {
      toast.error(t("commodities:toast.bulkTooMany", { max: meta.max ?? MAX_BULK_QUERY_ITEMS }))
      return
    }
    if (action.kind === "move") toast.error(t("commodities:toast.bulkMoveError"))
    else if (action.kind === "delete") toast.error(t("commodities:toast.bulkDeleteError"))
    else toast.error(t("commodities:toast.bulkUpdateError"))
  }

  async function submit(action: BulkAction) {
    if (!target) return
    if (await run(action, target.selection)) setDialog(null)
  }

  async function handleDelete() {
    const tgt = currentTarget()
    const ok = await confirm({
      title: t("commodities:bulk.deleteTitle", { count: tgt.count }),
      description: t("commodities:bulk.deleteDescription", { count: tgt.count }),
      confirmLabel: t("common:actions.delete"),
      destructive: true,
    })
    if (ok) await run({ kind: "delete" }, tgt.selection)
  }

  async function handleInUse() {
    const tgt = currentTarget()
    const ok = await confirm({
      title: t("commodities:bulk.inUseTitle", { count: tgt.count }),
      description: t("commodities:bulk.inUseDescription"),
      confirmLabel: t("commodities:bulk.inUseConfirm"),
    })
    if (ok) await run({ kind: "update", patch: { status: { value: "in_use" } } }, tgt.selection)
  }

  function openStatus(status: (typeof BULK_STATUSES)[number]) {
    if (status === "in_use") {
      void handleInUse()
      return
    }
    setStatusTarget(status)
    openDialog("status")
  }

  async function retry() {
    if (!outcome) return
    await run(outcome.action, { ids: outcome.result.failed.map((f) => f.id) })
  }

  // A held write keeps its button focusable, with the reason in the title,
  // instead of `disabled`, which would hide the tooltip. The hover class
  // cancels the hover color, which Button keys off the real attribute.
  const lockAttrs = locked
    ? { "aria-disabled": true as const, title: t("errors:lockedDuringMigration") }
    : {}
  const lockedClass = (hoverClass: string) =>
    locked ? cn("cursor-not-allowed opacity-50", hoverClass) : undefined

  const countLabel = selection.allMatching
    ? t("commodities:bulk.allMatchingSelected", { count: total })
    : t("commodities:bulk.selected", { count })
  // "Select all N matching" is offered once the whole page is selected and
  // the query matches more than the page.
  const offerAllMatching = !selection.allMatching && pageSelected && total > pageIds.length

  return (
    <>
      {count > 0 ? (
        <BulkActionBar
          label={
            <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
              <span data-testid="commodities-bulk-count">{countLabel}</span>
              {offerAllMatching && total <= MAX_BULK_QUERY_ITEMS ? (
                <Button
                  type="button"
                  variant="link"
                  size="sm"
                  className="h-auto p-0"
                  onClick={selection.selectAllMatching}
                  data-testid="commodities-select-all-matching"
                >
                  {t("commodities:bulk.selectAllMatching", { count: total })}
                </Button>
              ) : null}
              {offerAllMatching && total > MAX_BULK_QUERY_ITEMS ? (
                <span
                  className="text-xs text-muted-foreground"
                  data-testid="commodities-select-all-too-many"
                >
                  {t("commodities:bulk.selectAllTooMany", { max: MAX_BULK_QUERY_ITEMS })}
                </span>
              ) : null}
              {selection.allMatching ? (
                <Button
                  type="button"
                  variant="link"
                  size="sm"
                  className="h-auto p-0"
                  onClick={selection.clear}
                  data-testid="commodities-clear-selection"
                >
                  {t("commodities:bulk.clearSelection")}
                </Button>
              ) : null}
            </span>
          }
          regionLabel={t("commodities:bulk.regionLabel")}
          selectAll={{
            checked: pageSelected,
            onCheckedChange: () => selection.togglePage(pageIds),
            label: t("commodities:list.selectAll"),
            "data-testid": "commodities-select-all",
          }}
          className="max-w-3xl"
          data-testid="commodities-bulk-bar"
        >
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              if (!locked) openDialog("move")
            }}
            {...lockAttrs}
            className={lockedClass("hover:bg-background hover:text-foreground")}
            data-testid="commodities-bulk-move"
          >
            {t("commodities:bulk.moveButton")}
          </Button>
          <DropdownMenu open={editOpen} onOpenChange={(open) => setEditOpen(open && !locked)}>
            <DropdownMenuTrigger asChild>
              <Button
                variant="outline"
                size="sm"
                {...lockAttrs}
                className={cn("gap-1", lockedClass("hover:bg-background hover:text-foreground"))}
                data-testid="commodities-bulk-edit"
              >
                {t("commodities:bulk.editButton")}
                <ChevronDown className="size-4" aria-hidden="true" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                onSelect={() => openDialog("tags")}
                data-testid="commodities-bulk-edit-tags"
              >
                {t("commodities:bulk.editTags")}
              </DropdownMenuItem>
              <DropdownMenuItem
                onSelect={() => openDialog("type")}
                data-testid="commodities-bulk-edit-type"
              >
                {t("commodities:bulk.editType")}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuLabel>{t("commodities:bulk.statusLabel")}</DropdownMenuLabel>
              {BULK_STATUSES.map((status) => (
                <DropdownMenuItem
                  key={status}
                  onSelect={() => openStatus(status)}
                  data-testid={`commodities-bulk-status-${status}`}
                >
                  {t(`commodities:status.${status}`)}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
          {/* Print labels (#2807) takes ids, so it is offered for a picked selection. */}
          {slug && !selection.allMatching ? (
            <Button asChild variant="outline" size="sm" className="gap-1.5">
              <Link
                to={labelsPageHref(slug, { items: [...selection.ids] })}
                data-testid="commodities-bulk-print-labels"
              >
                <QrCode className="size-4" aria-hidden="true" />
                {t("commodities:bulk.printLabels")}
              </Link>
            </Button>
          ) : null}
          <Button
            variant="destructive"
            size="sm"
            onClick={() => {
              if (!locked) void handleDelete()
            }}
            disabled={bulkDelete.isPending}
            {...lockAttrs}
            className={lockedClass("hover:bg-destructive hover:text-white")}
            data-testid="commodities-bulk-delete"
          >
            <Trash2 className="mr-2 size-4" aria-hidden="true" />
            {t("commodities:bulk.deleteButton")}
          </Button>
        </BulkActionBar>
      ) : null}

      <Dialog
        open={dialog === "move"}
        onOpenChange={(open) => {
          if (!open) setDialog(null)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("commodities:bulk.moveTitle", { count: target?.count ?? 0 })}
            </DialogTitle>
            <DialogDescription>{t("commodities:bulk.moveDescription")}</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            <Label htmlFor="bulk-move-area">{t("commodities:bulk.moveTargetLabel")}</Label>
            {/* eslint-disable-next-line no-restricted-syntax -- bulk move-to-area utility selector; native <select> retained, covered by native-select e2e (commodity-bulk-and-filter.spec.ts) */}
            <select
              id="bulk-move-area"
              value={moveArea}
              onChange={(e) => setMoveArea(e.target.value)}
              className="border-input bg-transparent rounded-md border px-3 py-2 text-sm"
              data-testid="bulk-move-area"
            >
              <option value="">{t("commodities:bulk.moveTargetPlaceholder")}</option>
              {withId(areas).map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name}
                </option>
              ))}
            </select>
          </div>
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDialog(null)}>
              {t("common:actions.cancel")}
            </Button>
            <Button
              onClick={async () => {
                if (!moveArea) return
                await submit({ kind: "move", areaId: moveArea })
                setMoveArea("")
              }}
              disabled={!moveArea || pending}
              data-testid="bulk-move-confirm"
            >
              {t("commodities:bulk.moveConfirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <BulkTagsDialog
        kind="commodity"
        open={dialog === "tags"}
        onOpenChange={(open) => {
          if (!open) setDialog(null)
        }}
        count={target?.count ?? 0}
        isPending={pending}
        onSubmit={(change: BulkTagsChange) => {
          const tags: NonNullable<CommodityBulkPatch["tags"]> = {}
          if (change.add.length > 0) tags.add = change.add
          if (change.remove.length > 0) tags.remove = change.remove
          return submit({ kind: "update", patch: { tags } })
        }}
      />

      <BulkTypeDialog
        open={dialog === "type"}
        onOpenChange={(open) => {
          if (!open) setDialog(null)
        }}
        count={target?.count ?? 0}
        isPending={pending}
        onSubmit={(type) => submit({ kind: "update", patch: { type } })}
      />

      <StatusTransitionDialog
        open={dialog === "status"}
        onOpenChange={(open) => {
          if (!open) setDialog(null)
        }}
        targetStatus={statusTarget}
        title={
          statusTarget
            ? t("commodities:bulk.statusTitle", {
                count: target?.count ?? 0,
                label: t(`commodities:status.${statusTarget}`),
              })
            : undefined
        }
        description={t("commodities:bulk.statusDescription")}
        isPending={pending}
        onSubmit={async (payload) => {
          if (!statusTarget) return
          await submit({
            kind: "update",
            patch: {
              status: {
                value: statusTarget,
                date: payload.status_date,
                note: payload.status_note || undefined,
              },
            },
          })
        }}
      />

      <BulkResultDialog
        open={outcome !== null}
        onOpenChange={(open) => {
          if (!open) setOutcome(null)
        }}
        result={outcome?.result ?? null}
        itemHref={slug ? (id) => commodityHref(slug, id, listContext) : undefined}
        onRetry={() => void retry()}
        isRetrying={pending}
      />
    </>
  )
}
