import { useState } from "react"
import { useTranslation } from "react-i18next"
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
import { ordinaryLinks } from "@/features/files/api"
import { useLinkTypeLabel } from "@/features/files/labels"
import type { TrashedFile } from "@/features/trash/api"
import {
  usePurgeFile,
  usePurgeFiles,
  useRestoreFile,
  useRestoreFiles,
  useTrashedFiles,
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

function fileTitle(entry: TrashedFile): string {
  return entry.file.title?.trim() || entry.file.path?.trim() || entry.id
}

// TrashedFilesTab lists the files deleted on their own. Files that went to
// Trash with an item are counted on that item's row and come back with it.
export function TrashedFilesTab({ permissions }: { permissions: TrashPermissions }) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const confirm = useConfirm()
  const typeLabel = useLinkTypeLabel()

  const [requestedPage, setRequestedPage] = useState(1)
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const list = useTrashedFiles({ page: requestedPage, perPage: PAGE_SIZE })
  const { page, totalPages } = pageWithin(
    requestedPage,
    setRequestedPage,
    list.data?.total,
    PAGE_SIZE
  )
  const entries = list.data?.entries ?? []
  const selectedIds = entries.filter((e) => selected.has(e.id)).map((e) => e.id)

  const restoreOne = useRestoreFile()
  const restoreMany = useRestoreFiles()
  const purgeOne = usePurgeFile()
  const purgeMany = usePurgeFiles()
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

  function attachedTo(entry: TrashedFile): string {
    const links = ordinaryLinks(entry.file)
    if (links.length === 0) return t("trash:table.notAttached")
    return links.map((l) => l.entity_name || typeLabel(l.entity_type)).join(", ")
  }

  async function onRestore(entry: TrashedFile) {
    try {
      await restoreOne.mutateAsync(entry.id)
      toast.success(t("trash:toast.fileRestored", { name: fileTitle(entry) }))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  async function onPurge(entry: TrashedFile) {
    const ok = await confirm({
      title: t("trash:confirm.purgeFileTitle", { name: fileTitle(entry) }),
      description: t("trash:confirm.purgeFileBody"),
      confirmLabel: t("trash:actions.purge"),
      destructive: true,
    })
    if (!ok) return
    try {
      await purgeOne.mutateAsync(entry.id)
      toast.success(t("trash:toast.filePurged", { name: fileTitle(entry) }))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  async function onBulk(kind: "restore" | "purge") {
    const ids = selectedIds
    if (!ids.length) return
    if (kind === "purge") {
      const ok = await confirm({
        title: t("trash:confirm.purgeFilesTitle", { count: ids.length }),
        description: t("trash:confirm.purgeFilesBody", { count: ids.length }),
        confirmLabel: t("trash:actions.purge"),
        destructive: true,
      })
      if (!ok) return
    }
    try {
      const result =
        kind === "restore" ? await restoreMany.mutateAsync(ids) : await purgeMany.mutateAsync(ids)
      setSelected(new Set())
      if (result.failed.length > 0) {
        toast.warning(
          t("trash:toast.bulkPartial", {
            succeeded: result.succeeded.length,
            failed: result.failed.length,
          })
        )
      } else if (kind === "restore") {
        toast.success(t("trash:toast.filesRestored", { count: result.succeeded.length }))
      } else {
        toast.success(t("trash:toast.filesPurged", { count: result.succeeded.length }))
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  if (list.isLoading) {
    return (
      <div className="flex flex-col gap-2" data-testid="trash-files-loading">
        <Skeleton className="h-10" />
        <Skeleton className="h-10" />
        <Skeleton className="h-10" />
      </div>
    )
  }
  if (list.isError) {
    return <ListLoadError testId="trash-files-error" onRetry={() => void list.refetch()} />
  }
  if (entries.length === 0) {
    return <TrashEmptyState testId="trash-files-empty" />
  }

  return (
    <div className="flex flex-col gap-4">
      <TableShell>
        <Table data-testid="trash-files-table">
          <TableHeader>
            <TableRow>
              {selectable ? (
                <TableHead className="w-10">
                  <SelectCell
                    checked={headerCheckState(selectedIds.length, entries.length) === true}
                    onChange={toggleAll}
                    label={t("trash:table.selectAll")}
                    testId="trash-files-select-all"
                  />
                </TableHead>
              ) : null}
              <TableHead>{t("trash:table.file")}</TableHead>
              <TableHead>{t("trash:table.attachedTo")}</TableHead>
              <TableHead>{t("trash:table.deleted")}</TableHead>
              <TableHead>{t("trash:table.expires")}</TableHead>
              <TableHead className="text-right">
                <span className="sr-only">{t("trash:table.actions")}</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {entries.map((entry) => {
              const name = fileTitle(entry)
              return (
                <TableRow key={entry.id} data-testid={`trash-file-${entry.id}`}>
                  {selectable ? (
                    <TableCell>
                      <SelectCell
                        checked={selected.has(entry.id)}
                        onChange={(checked) => toggle(entry.id, checked)}
                        label={t("trash:table.selectRow", { name })}
                        testId={`trash-file-${entry.id}-select`}
                      />
                    </TableCell>
                  ) : null}
                  <TableCell>
                    <p className="font-medium">{name}</p>
                    <p className="text-xs text-muted-foreground tabular-nums">
                      {formatBytes(entry.file.size_bytes ?? 0)}
                    </p>
                  </TableCell>
                  <TableCell className="text-muted-foreground">{attachedTo(entry)}</TableCell>
                  <TableCell>
                    <DeletedCell at={entry.file.deleted_at} by={entry.deletedByName} />
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
                      testId={`trash-file-${entry.id}`}
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
          testId="trash-files-pagination"
        />
      ) : null}

      {selectedIds.length > 0 ? (
        <BulkActionBar
          label={t("trash:bulk.selectedFiles", { count: selectedIds.length })}
          selectAll={{
            checked: headerCheckState(selectedIds.length, entries.length),
            onCheckedChange: (next) => toggleAll(next === true),
            label: t("trash:table.selectAll"),
            "data-testid": "trash-files-bulk-select-all",
          }}
          data-testid="trash-files-bulk-bar"
        >
          {permissions.canRestore ? (
            <Button
              size="sm"
              variant="outline"
              onClick={() => void onBulk("restore")}
              disabled={busy}
              data-testid="trash-files-bulk-restore"
            >
              <RotateCcw className="size-4" aria-hidden="true" />
              {t("trash:actions.restore")}
            </Button>
          ) : null}
          {permissions.canPurge ? (
            <Button
              size="sm"
              variant="destructive"
              onClick={() => void onBulk("purge")}
              disabled={busy}
              data-testid="trash-files-bulk-purge"
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
