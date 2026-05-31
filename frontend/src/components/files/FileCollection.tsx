import { useTranslation } from "react-i18next"

import { FileCard, type FileCardCoverState } from "@/components/files/FileCard"
import { FileListRow } from "@/components/files/FileListRow"
import { Checkbox } from "@/components/ui/checkbox"
import type { ListedFile } from "@/features/files/api"
import type { FilesViewMode } from "@/features/files/useFilesViewMode"
import { cn } from "@/lib/utils"

// Single source of truth for rendering a set of files as EITHER a
// FileCard grid OR a FileListRow list (#1966). Every file-listing
// surface (the global Files page, the location/area EntityFilesPanel,
// and the commodity detail Files tab) delegates its grid/list body
// here so a file looks and behaves identically wherever it appears.
//
// Bulk selection (checkboxes + the desktop select-all header) is opt-in
// via `onToggleSelect` / `showListHeader` — the Files page wires them,
// the read-only entity surfaces omit them. Cover-star props are
// forwarded straight to FileCard (image cards only).
const DEFAULT_GRID = "grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4"

export interface FileCollectionProps {
  items: ListedFile[]
  viewMode: FilesViewMode
  onOpen: (id: string) => void
  // Bulk-selection (Files page only) — all optional.
  selectedIds?: Set<string>
  onToggleSelect?: (id: string) => void
  allSelectedOnPage?: boolean
  onToggleAll?: () => void
  showListHeader?: boolean
  // Cover-photo wiring — forwarded to FileCard (#1451).
  coverState?: FileCardCoverState
  onSetCover?: (fileId: string | null) => void
  coverBusy?: boolean
  // Surface-scoped container test id: emits `${idPrefix}-grid` /
  // `${idPrefix}-list`. Defaults to the Files-page ids.
  idPrefix?: string
  gridClassName?: string
}

export function FileCollection({
  items,
  viewMode,
  onOpen,
  selectedIds,
  onToggleSelect,
  allSelectedOnPage = false,
  onToggleAll,
  showListHeader = false,
  coverState,
  onSetCover,
  coverBusy,
  idPrefix = "files",
  gridClassName,
}: FileCollectionProps) {
  const { t } = useTranslation()

  if (viewMode === "grid") {
    return (
      <div className={cn(gridClassName ?? DEFAULT_GRID)} data-testid={`${idPrefix}-grid`}>
        {items.map(({ file, signedUrl }) => (
          <FileCard
            key={file.id}
            file={file}
            signedUrl={signedUrl}
            selected={selectedIds?.has(file.id)}
            onToggleSelect={onToggleSelect}
            onOpen={onOpen}
            coverState={coverState}
            onSetCover={onSetCover}
            coverBusy={coverBusy}
          />
        ))}
      </div>
    )
  }

  return (
    <div className="overflow-hidden rounded-xl border bg-card" data-testid={`${idPrefix}-list`}>
      {showListHeader ? (
        <div className="hidden grid-cols-[auto_auto_1fr_auto_auto_auto] gap-4 border-b bg-muted/50 px-4 py-2 sm:grid">
          <div>
            <Checkbox
              checked={allSelectedOnPage}
              onCheckedChange={onToggleAll}
              aria-label={t("files:list.selectAll")}
              data-testid="files-list-select-all"
            />
          </div>
          <span className="size-4" aria-hidden="true" />
          <span className="text-xs font-medium text-muted-foreground">
            {t("files:list.columnName", { defaultValue: "Name" })}
          </span>
          <span className="w-24 text-center text-xs font-medium text-muted-foreground">
            {t("files:list.columnCategory", { defaultValue: "Category" })}
          </span>
          <span className="w-28 text-right text-xs font-medium text-muted-foreground">
            {t("files:list.columnUploaded", { defaultValue: "Uploaded" })}
          </span>
          <span className="w-16 text-right text-xs font-medium text-muted-foreground">
            {t("files:list.columnSize", { defaultValue: "Size" })}
          </span>
        </div>
      ) : null}
      <ul className="divide-y">
        {items.map(({ file }) => (
          <FileListRow
            key={file.id}
            file={file}
            selected={selectedIds?.has(file.id)}
            onToggleSelect={onToggleSelect}
            onOpen={onOpen}
          />
        ))}
      </ul>
    </div>
  )
}
