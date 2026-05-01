import { useTranslation } from "react-i18next"
import { File as FileIcon, FileImage, FileText, Receipt } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Card } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import type { FileEntity, URLData } from "@/features/files/api"
import { isImageMime } from "@/features/files/constants"
import { formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

// One row in the Files grid. Renders a thumbnail (image MIME) or a
// category icon, the title (falls back to the path), upload date, and
// the tag pills. Click anywhere outside the checkbox opens the detail
// sheet. The checkbox toggles bulk selection.
export interface FileCardProps {
  file: FileEntity & { id: string }
  signedUrl?: URLData
  selected: boolean
  onToggleSelect: (id: string) => void
  onOpen: (id: string) => void
}

export function FileCard({ file, signedUrl, selected, onToggleSelect, onOpen }: FileCardProps) {
  const { t } = useTranslation()
  const title = file.title?.trim() || file.path?.trim() || file.id
  const Icon = iconForCategory(file.category)
  // Thumbnail keys come from services/file_signing_service.go — the BE
  // emits `small` / `medium` / `large`. Falling back through the sizes
  // means we always pick the smallest available (best for the
  // grid-card cell), and finally drop to the full-resolution signed
  // URL if no thumbnails were generated yet.
  const thumbUrl =
    signedUrl?.thumbnails?.small ?? signedUrl?.thumbnails?.medium ?? signedUrl?.thumbnails?.large
  const renderImage = isImageMime(file.mime_type) && (thumbUrl || signedUrl?.url)
  const tags = file.tags ?? []

  return (
    <Card
      data-testid={`file-card-${file.id}`}
      data-category={file.category}
      className={cn(
        "group relative flex h-full flex-col overflow-hidden focus-within:ring-2 focus-within:ring-ring",
        selected && "ring-2 ring-primary"
      )}
    >
      <div className="absolute left-2 top-2 z-10">
        <Checkbox
          checked={selected}
          onCheckedChange={() => onToggleSelect(file.id)}
          aria-label={t("files:list.selectFile", { title, defaultValue: `Select ${title}` })}
          data-testid={`file-card-checkbox-${file.id}`}
          className="bg-background"
        />
      </div>
      <button
        type="button"
        onClick={() => onOpen(file.id)}
        aria-label={t("files:list.openDetail", { title, defaultValue: `Open ${title}` })}
        data-testid={`file-card-open-${file.id}`}
        className="flex flex-1 flex-col text-left focus-visible:outline-none"
      >
        <div className="aspect-[4/3] w-full overflow-hidden bg-muted">
          {renderImage ? (
            <img
              src={thumbUrl ?? signedUrl?.url}
              alt={title}
              loading="lazy"
              className="size-full object-cover"
            />
          ) : (
            <div className="flex size-full items-center justify-center">
              <Icon className="size-12 text-muted-foreground" aria-hidden="true" />
            </div>
          )}
        </div>
        <div className="flex flex-1 flex-col gap-2 p-3">
          <div className="line-clamp-2 text-sm font-medium" title={title}>
            {title}
          </div>
          <div className="text-xs text-muted-foreground">
            {file.created_at
              ? t("files:list.uploadDate", {
                  date: formatDate(file.created_at),
                  defaultValue: `Uploaded ${formatDate(file.created_at)}`,
                })
              : null}
          </div>
          {tags.length > 0 && (
            <div className="flex flex-wrap gap-1">
              {tags.slice(0, 3).map((tag) => (
                <Badge key={tag} variant="secondary" className="text-[10px]">
                  {tag}
                </Badge>
              ))}
              {tags.length > 3 && (
                <Badge variant="outline" className="text-[10px]">
                  +{tags.length - 3}
                </Badge>
              )}
            </div>
          )}
        </div>
      </button>
    </Card>
  )
}

function iconForCategory(category: FileEntity["category"]) {
  switch (category) {
    case "photos":
      return FileImage
    case "invoices":
      return Receipt
    case "documents":
      return FileText
    default:
      return FileIcon
  }
}
