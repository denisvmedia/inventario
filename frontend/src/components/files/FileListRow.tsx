import { useTranslation } from "react-i18next"
import { File as FileIcon, FileImage, FileText } from "lucide-react"

import { Checkbox } from "@/components/ui/checkbox"
import type { FileEntity } from "@/features/files/api"
import { FILE_CATEGORY_TILES, FILE_TAG_PILLS, getFileVisualMeta } from "@/features/files/constants"
import { useCategoryLabel, useTagPillLabel } from "@/features/files/labels"
import { formatBytes, formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

// One row in the Files list-view (the table-style alternate to the
// FileCard grid). Mirrors design-mocks/src/views/FileBrowserView.tsx
// rows ~700–765: 5-column desktop layout (icon / Name+meta / Category
// badge / Uploaded / Size) and a mobile card-style collapse.
//
// Activating the row (click or Enter/Space) opens the detail sheet via
// `onOpen`. The checkbox sits behind a `data-row-checkbox` boundary so
// its clicks don't bubble into the row activation handler.
export interface FileListRowProps {
  file: FileEntity & { id: string }
  // Selection is optional: when `onToggleSelect` is omitted the row
  // renders without a checkbox (read-only), mirroring FileCard — so the
  // same row works on the bulk-select Files page and on the
  // checkbox-free entity-detail surfaces (#1966).
  selected?: boolean
  onToggleSelect?: (id: string) => void
  onOpen: (id: string) => void
}

export function FileListRow({ file, selected = false, onToggleSelect, onOpen }: FileListRowProps) {
  const { t } = useTranslation()
  const labelOf = useCategoryLabel()
  const tagLabelOf = useTagPillLabel()
  const title = file.title?.trim() || file.path?.trim() || file.id
  const tile = FILE_CATEGORY_TILES.find((c) => c.key === file.category)
  const categoryIconClass = cn("size-3 shrink-0", tile?.activeColor ?? "text-muted-foreground")
  const categoryLabel = labelOf((tile?.key ?? "other") as Parameters<typeof labelOf>[0])
  const tags = file.tags ?? []
  // The curated tag pills carry colour metadata; for any tag the user
  // tagged with (lowercase match) we fall back to the standard text
  // colour so unknown tags still surface on the row.
  const matchedTags = tags.map((tag) => {
    const pill = FILE_TAG_PILLS.find((p) => p.id === tag.toLowerCase())
    return {
      id: tag,
      label: pill ? tagLabelOf(pill.id) : tag,
      colorClass: pill?.colorClass ?? "text-muted-foreground",
    }
  })
  const dateStr = file.created_at ? formatDate(file.created_at) : ""
  const sizeStr = file.size_bytes ? formatBytes(file.size_bytes) : ""
  // Per-MIME / per-category palette via shared helper so the leading
  // icon picks up the same accent as the FileCard fallback and the
  // mock's mimeIconAndColor (lines 152-157).
  const visual = getFileVisualMeta(file)
  const LeadingIcon = visual.icon
  const openLabel = t("files:list.openDetail", { title, defaultValue: `Open ${title}` })
  // When no selection handler is wired, drop the checkbox column (and
  // its leading grid track) so the row collapses cleanly to 5 columns.
  const showCheckbox = !!onToggleSelect

  return (
    <li>
      {/* Desktop row (sm+). The Checkbox lives in column 1 OUTSIDE the
          activation button so axe doesn't see nested interactive
          elements; the button covers columns 2–6 which is everything
          else (icon / Name+meta / Category badge / Uploaded / Size). */}
      <div
        className={cn(
          "hidden items-center gap-4 px-4 transition-colors sm:grid",
          showCheckbox
            ? "grid-cols-[auto_auto_1fr_auto_auto_auto]"
            : "grid-cols-[auto_1fr_auto_auto_auto]",
          selected ? "bg-accent" : "hover:bg-muted/40"
        )}
        data-testid={`file-row-${file.id}`}
        data-category={file.category}
        data-mime-group={visual.group}
      >
        {showCheckbox ? (
          <Checkbox
            checked={selected}
            onCheckedChange={() => onToggleSelect?.(file.id)}
            aria-label={t("files:list.selectFile", { title, defaultValue: `Select ${title}` })}
            data-testid={`file-row-checkbox-${file.id}`}
          />
        ) : null}
        <button
          type="button"
          onClick={() => onOpen(file.id)}
          aria-label={openLabel}
          className={cn(
            "col-span-5 grid cursor-pointer grid-cols-subgrid items-center gap-4 py-3 text-left",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          )}
        >
          <LeadingIcon className={cn("size-4 shrink-0", visual.colorClass)} aria-hidden="true" />
          <div className="min-w-0">
            <p className="truncate text-sm font-medium" title={title}>
              {title}
            </p>
            {(file.linked_entity_id || matchedTags.length > 0) && (
              <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5">
                {file.linked_entity_id ? (
                  <span className="truncate text-xs text-muted-foreground">
                    {file.linked_entity_type ?? ""}
                  </span>
                ) : null}
                {matchedTags.slice(0, 4).map((tag) => (
                  <span
                    key={tag.id}
                    className={cn("text-[10px] font-medium", tag.colorClass)}
                    data-testid={`file-row-tag-${file.id}-${tag.id.toLowerCase()}`}
                  >
                    #{tag.label}
                  </span>
                ))}
              </div>
            )}
          </div>
          <div
            className={cn(
              "flex w-24 items-center justify-center gap-1 rounded-full px-2 py-0.5",
              tile?.activeBg ?? "bg-muted"
            )}
          >
            {renderCategoryIcon(file.category, categoryIconClass)}
            <span
              className={cn(
                "text-[10px] font-medium",
                tile?.activeColor ?? "text-muted-foreground"
              )}
            >
              {categoryLabel}
            </span>
          </div>
          <span className="w-28 text-right text-xs text-muted-foreground tabular-nums">
            {dateStr}
          </span>
          <span className="w-16 text-right text-xs text-muted-foreground tabular-nums">
            {sizeStr}
          </span>
        </button>
      </div>

      {/* Mobile row — card-style collapse. Same Checkbox-outside-button
          structure as the desktop row, just stacked. */}
      <div
        className={cn(
          "flex items-start gap-3 px-4 py-3 transition-colors sm:hidden",
          selected ? "bg-accent" : "active:bg-muted/40"
        )}
      >
        {showCheckbox ? (
          <Checkbox
            checked={selected}
            onCheckedChange={() => onToggleSelect?.(file.id)}
            aria-label={t("files:list.selectFile", { title, defaultValue: `Select ${title}` })}
            className="mt-0.5"
          />
        ) : null}
        <button
          type="button"
          onClick={() => onOpen(file.id)}
          aria-label={openLabel}
          className={cn(
            "flex flex-1 cursor-pointer items-start gap-3 text-left",
            "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          )}
        >
          <div
            className={cn(
              "mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-lg",
              visual.bgClass
            )}
          >
            <LeadingIcon className={cn("size-4", visual.colorClass)} aria-hidden="true" />
          </div>
          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-medium leading-tight" title={title}>
              {title}
            </p>
            <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-0.5">
              <span
                className={cn(
                  "flex items-center gap-1 rounded-full px-2 py-0.5",
                  tile?.activeBg ?? "bg-muted"
                )}
              >
                {renderCategoryIcon(
                  file.category,
                  cn("size-2.5 shrink-0", tile?.activeColor ?? "text-muted-foreground")
                )}
                <span
                  className={cn(
                    "text-[10px] font-medium",
                    tile?.activeColor ?? "text-muted-foreground"
                  )}
                >
                  {categoryLabel}
                </span>
              </span>
              {matchedTags.slice(0, 3).map((tag) => (
                <span key={tag.id} className={cn("text-[10px] font-medium", tag.colorClass)}>
                  #{tag.label}
                </span>
              ))}
            </div>
          </div>
          <div className="mt-0.5 flex shrink-0 flex-col items-end gap-0.5">
            <span className="text-xs text-muted-foreground tabular-nums">{sizeStr}</span>
            <span className="text-[10px] whitespace-nowrap text-muted-foreground tabular-nums">
              {dateStr}
            </span>
          </div>
        </button>
      </div>
    </li>
  )
}

function renderCategoryIcon(
  category: FileEntity["category"],
  className = "size-3 shrink-0 text-muted-foreground"
) {
  switch (category) {
    case "images":
      return <FileImage className={className} aria-hidden="true" />
    case "documents":
      return <FileText className={className} aria-hidden="true" />
    default:
      return <FileIcon className={className} aria-hidden="true" />
  }
}
