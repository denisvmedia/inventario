import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { RotateCcw, Trash2 } from "lucide-react"

import { ListLoadError } from "@/components/common/ListLoadError"
import { Pagination, pageWithin } from "@/components/common/Pagination"
import { BulkActionBar } from "@/components/ui/bulk-action-bar"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useCurrentGroup } from "@/features/group/GroupContext"
import type { TrashedCommodity } from "@/features/trash/api"
import {
  usePurgeCommodities,
  usePurgeCommodity,
  useRestoreCommodities,
  useRestoreCommodity,
  useTrashedCommodities,
  type TrashPermissions,
} from "@/features/trash/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatBytes } from "@/lib/intl"

import {
  DeletedCell,
  ExpiresCell,
  RowActions,
  SelectCell,
  TableShell,
  TrashEmptyState,
  headerCheckState,
} from "./TrashTable"

const PAGE_SIZE = 50

// TrashedItemsTab lists the items in Trash with restore and delete-for-good
// actions, one at a time or for a selection.
export function TrashedItemsTab({ permissions }: { permissions: TrashPermissions }) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const confirm = useConfirm()
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""

  const [requestedPage, setRequestedPage] = useState(1)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const list = useTrashedCommodities({ page: requestedPage, perPage: PAGE_SIZE })
  const { page, totalPages } = pageWithin(
    requestedPage,
    setRequestedPage,
    list.data?.total,
    PAGE_SIZE
  )
  const entries = list.data?.entries ?? []
  // Rows that left the list (restored elsewhere, purged by the worker) drop
  // out of the selection on their own.
  const selectedIds = entries.filter((e) => selected.has(e.id)).map((e) => e.id)

  const restoreOne = useRestoreCommodity()
  const restoreMany = useRestoreCommodities()
  const purgeOne = usePurgeCommodity()
  const purgeMany = usePurgeCommodities()
  const busy =
    restoreOne.isPending || restoreMany.isPending || purgeOne.isPending || purgeMany.isPending
  const selectable = permissions.canRestore || permissions.canPurge

  function setPage(next: number) {
    setRequestedPage(next)
    setSelected(new Set())
  }

  function toggle(id: string, checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (checked) next.add(id)
      else next.delete(id)
      return next
    })
  }

  function toggleAll(checked: boolean) {
    setSelected(checked ? new Set(entries.map((e) => e.id)) : new Set())
  }

  function itemHref(id: string) {
    return `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}`
  }

  async function onRestore(entry: TrashedCommodity) {
    const name = entry.commodity.name ?? ""
    try {
      const result = await restoreOne.mutateAsync(entry.id)
      const open = { label: t("trash:toast.openItem"), onClick: () => navigate(itemHref(entry.id)) }
      if (result.areaRemoved) {
        toast.warning(t("trash:toast.itemRestoredUnassigned", { name }), {
          description: t("trash:toast.itemRestoredUnassignedBody"),
          action: open,
        })
      } else {
        toast.success(t("trash:toast.itemRestored", { name }), { action: open })
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  async function onPurge(entry: TrashedCommodity) {
    const ok = await confirm({
      title: t("trash:confirm.purgeItemTitle", { name: entry.commodity.name ?? "" }),
      description:
        entry.fileCount > 0
          ? t("trash:confirm.purgeItemBodyFiles", { count: entry.fileCount })
          : t("trash:confirm.purgeItemBody"),
      confirmLabel: t("trash:actions.purge"),
      destructive: true,
    })
    if (!ok) return
    try {
      await purgeOne.mutateAsync(entry.id)
      toast.success(t("trash:toast.itemPurged", { name: entry.commodity.name ?? "" }))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  async function onBulkRestore() {
    const ids = selectedIds
    if (!ids.length) return
    try {
      const result = await restoreMany.mutateAsync(ids)
      setSelected(new Set())
      if (result.failed.length > 0 || result.unknown.length > 0) {
        toast.warning(
          t("trash:toast.bulkPartial", {
            succeeded: result.succeeded.length,
            failed: result.failed.length,
          }),
          result.unknown.length > 0
            ? { description: t("trash:toast.restoreUnknown", { count: result.unknown.length }) }
            : undefined
        )
      } else if (result.areaRemoved.length > 0) {
        toast.warning(t("trash:toast.itemsRestored", { count: result.succeeded.length }), {
          description: t("trash:toast.itemsRestoredUnassigned", {
            count: result.areaRemoved.length,
          }),
        })
      } else {
        toast.success(t("trash:toast.itemsRestored", { count: result.succeeded.length }))
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  async function onBulkPurge() {
    const ids = selectedIds
    if (!ids.length) return
    const ok = await confirm({
      title: t("trash:confirm.purgeItemsTitle", { count: ids.length }),
      description: t("trash:confirm.purgeItemsBody", { count: ids.length }),
      confirmLabel: t("trash:actions.purge"),
      destructive: true,
    })
    if (!ok) return
    try {
      const result = await purgeMany.mutateAsync(ids)
      setSelected(new Set())
      if (result.failed.length > 0) {
        toast.warning(
          t("trash:toast.bulkPartial", {
            succeeded: result.succeeded.length,
            failed: result.failed.length,
          })
        )
      } else {
        toast.success(t("trash:toast.itemsPurged", { count: result.succeeded.length }))
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  if (list.isLoading) {
    return (
      <div className="flex flex-col gap-2" data-testid="trash-items-loading">
        <Skeleton className="h-10" />
        <Skeleton className="h-10" />
        <Skeleton className="h-10" />
      </div>
    )
  }
  if (list.isError) {
    return <ListLoadError testId="trash-items-error" onRetry={() => void list.refetch()} />
  }
  if (entries.length === 0) {
    return <TrashEmptyState testId="trash-items-empty" />
  }

  return (
    <div className="flex flex-col gap-4">
      <TableShell>
        <Table data-testid="trash-items-table">
          <TableHeader>
            <TableRow>
              {selectable ? (
                <TableHead className="w-10">
                  <SelectCell
                    checked={headerCheckState(selectedIds.length, entries.length) === true}
                    onChange={toggleAll}
                    label={t("trash:table.selectAll")}
                    testId="trash-items-select-all"
                  />
                </TableHead>
              ) : null}
              <TableHead>{t("trash:table.item")}</TableHead>
              <TableHead>{t("trash:table.files")}</TableHead>
              <TableHead>{t("trash:table.deleted")}</TableHead>
              <TableHead>{t("trash:table.expires")}</TableHead>
              <TableHead className="text-right">
                <span className="sr-only">{t("trash:table.actions")}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map((entry) => {
              const name = entry.commodity.name ?? ""
              return (
                <TableRow key={entry.id} data-testid={`trash-item-${entry.id}`}>
                  {selectable ? (
                    <TableCell>
                      <SelectCell
                        checked={selected.has(entry.id)}
                        onChange={(checked) => toggle(entry.id, checked)}
                        label={t("trash:table.selectRow", { name })}
                        testId={`trash-item-${entry.id}-select`}
                      />
                    </TableCell>
                  ) : null}
                  <TableCell>
                    <p className="font-medium">{name}</p>
                    {entry.commodity.short_name ? (
                      <p className="text-xs text-muted-foreground">{entry.commodity.short_name}</p>
                    ) : null}
                  </TableCell>
                  <TableCell className="text-muted-foreground tabular-nums">
                    {entry.fileCount > 0
                      ? t("trash:table.fileSummary", {
                          count: entry.fileCount,
                          size: formatBytes(entry.fileBytes),
                        })
                      : "—"}
                  </TableCell>
                  <TableCell>
                    <DeletedCell at={entry.commodity.deleted_at} by={entry.deletedByName} />
                  </TableCell>
                  <TableCell>
                    <ExpiresCell at={entry.expiresAt} />
                  </TableCell>
                  <TableCell>
                    <RowActions
                      name={name}
                      canRestore={permissions.canRestore}
                      canPurge={permissions.canPurge}
                      busy={busy}
                      onRestore={() => void onRestore(entry)}
                      onPurge={() => void onPurge(entry)}
                      testId={`trash-item-${entry.id}`}
                    />
                  </TableCell>
                </TableRow>
              )
            })}
          </TableBody>
        </Table>
      </TableShell>

      {totalPages > 1 ? (
        <Pagination
          page={page}
          totalPages={totalPages}
          onChange={setPage}
          testId="trash-items-pagination"
        />
      ) : null}

      {selectedIds.length > 0 ? (
        <BulkActionBar
          label={t("trash:bulk.selectedItems", { count: selectedIds.length })}
          selectAll={{
            checked: headerCheckState(selectedIds.length, entries.length),
            onCheckedChange: (next) => toggleAll(next === true),
            label: t("trash:table.selectAll"),
            "data-testid": "trash-items-bulk-select-all",
          }}
          data-testid="trash-items-bulk-bar"
        >
          {permissions.canRestore ? (
            <Button
              size="sm"
              variant="outline"
              onClick={() => void onBulkRestore()}
              disabled={busy}
              data-testid="trash-items-bulk-restore"
            >
              <RotateCcw className="size-4" aria-hidden="true" />
              {t("trash:actions.restore")}
            </Button>
          ) : null}
          {permissions.canPurge ? (
            <Button
              size="sm"
              variant="destructive"
              onClick={() => void onBulkPurge()}
              disabled={busy}
              data-testid="trash-items-bulk-purge"
            >
              <Trash2 className="size-4" aria-hidden="true" />
              {t("trash:actions.purge")}
            </Button>
          ) : null}
        </BulkActionBar>
      ) : null}
    </div>
  )
}
