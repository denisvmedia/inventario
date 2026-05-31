import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { Upload } from "lucide-react"

import type { FileCardCoverState } from "@/components/files/FileCard"
import { FileCollection } from "@/components/files/FileCollection"
import { FileDetailSheet } from "@/components/files/FileDetailSheet"
import { FileViewToggle } from "@/components/files/FileViewToggle"
import type { GalleryImage } from "@/components/files/ImageViewer"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { isImageMime } from "@/features/files/constants"
import { useFiles } from "@/features/files/hooks"
import { useFilesViewMode } from "@/features/files/useFilesViewMode"
import { useCurrentGroup } from "@/features/group/GroupContext"

// Files panel for entity-detail pages (commodity / location / area).
// Renders files attached to the given linked entity via the unified
// `GET /files?linked_entity_type=…&linked_entity_id=…` endpoint
// introduced for #1411 AC #4 (area linkage added under #1531 item 1).
//
// Files render through the shared `FileCollection` (FileCard grid /
// FileListRow list) with a grid/list toggle, consistent with the global
// Files page and the commodity Files tab (#1966).
//
// Clicking a card opens the same right-side FileDetailSheet the /files page
// uses, but *in place* — driven by local state instead of navigating to
// `/files/:id` — so the user keeps the entity-detail context (metadata +
// inline preview, with expand-to-fullscreen from there). Image siblings are
// passed through so the fullscreen viewer's gallery cycles this entity's
// photos.
//
// `onAttachClick` (#1448): when provided, renders an "Attach files"
// button in the header that opens an upload affordance with this
// entity's linkage preselected. Without it, the panel stays read-only
// — a generic "Add files" CTA would link to the global upload and
// create an orphan file (no linked_entity_*), which would NOT show
// up here. The detail page owns the dialog state because it also
// hosts the page-level drag-drop catcher.
export interface EntityFilesPanelProps {
  linkedEntityType: "commodity" | "location" | "area"
  linkedEntityId: string
  pageSize?: number
  onAttachClick?: () => void
  // Cover-photo wiring (issue #1451 option B). `coverState` flags which
  // file is the resolved cover today (explicit override or first-photo
  // auto-pick); `onSetCover` is the mutation entry-point — it must
  // return synchronously, callers wrap their mutation with optimistic
  // cache updates. Both must be supplied to render the star button on
  // each photo.
  coverState?: FileCardCoverState
  onSetCover?: (fileId: string | null) => void
  coverBusy?: boolean
}

const DEFAULT_PAGE_SIZE = 24

export function EntityFilesPanel({
  linkedEntityType,
  linkedEntityId,
  pageSize = DEFAULT_PAGE_SIZE,
  onAttachClick,
  coverState,
  onSetCover,
  coverBusy,
}: EntityFilesPanelProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  // Grid/list toggle, shared with the commodity Files tab via the same
  // localStorage key (entity-detail surfaces default to grid).
  const [viewMode, setViewMode] = useFilesViewMode("files:entityViewMode", "grid")
  // The file whose detail sheet is open, kept locally so opening it never
  // leaves the entity-detail page (#1963 follow-up).
  const [selectedId, setSelectedId] = useState<string | null>(null)

  const filesQuery = useFiles(
    {
      linkedEntityType,
      linkedEntityId,
      perPage: pageSize,
    },
    { enabled: !!linkedEntityId && !!slug }
  )

  const files = filesQuery.data?.files ?? []
  const total = filesQuery.data?.total ?? 0

  // This entity's photos, in grid order, for the fullscreen viewer's gallery.
  const imageSiblings: GalleryImage[] = files
    .filter(({ file, signedUrl }) => isImageMime(file.mime_type) && !!signedUrl?.url)
    .map(({ file, signedUrl }) => ({
      id: file.id,
      url: signedUrl?.url ?? "",
      alt: file.title?.trim() || file.path?.trim() || file.id,
    }))

  return (
    <Card data-testid="entity-files-panel">
      <CardHeader className="flex flex-row items-start justify-between gap-3">
        <div className="min-w-0">
          <CardTitle className="text-base">{t("files:entityPanel.title")}</CardTitle>
          <CardDescription>{t("files:entityPanel.description", { count: total })}</CardDescription>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          {files.length > 0 ? (
            <FileViewToggle
              value={viewMode}
              onChange={setViewMode}
              testIdPrefix="entity-files-panel-view"
            />
          ) : null}
          {onAttachClick ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={onAttachClick}
              data-testid="entity-files-panel-attach"
              className="gap-1.5 shrink-0"
            >
              <Upload className="size-3.5" aria-hidden="true" />
              {t("files:entityPanel.attach")}
            </Button>
          ) : null}
        </div>
      </CardHeader>
      <CardContent>
        {filesQuery.isError ? (
          <Alert variant="destructive" data-testid="entity-files-panel-error">
            <AlertTitle>{t("files:entityPanel.errorTitle")}</AlertTitle>
            <AlertDescription>{t("files:entityPanel.errorDescription")}</AlertDescription>
          </Alert>
        ) : filesQuery.isLoading ? (
          <div
            className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
            data-testid="entity-files-panel-loading"
          >
            {Array.from({ length: 3 }).map((_, i) => (
              <Skeleton key={i} className="aspect-[4/3] w-full" />
            ))}
          </div>
        ) : files.length === 0 ? (
          <p className="text-sm text-muted-foreground" data-testid="entity-files-panel-empty">
            {t("files:entityPanel.empty")}
          </p>
        ) : (
          <FileCollection
            items={files}
            viewMode={viewMode}
            onOpen={(id) => setSelectedId(id)}
            coverState={coverState}
            onSetCover={onSetCover}
            coverBusy={coverBusy}
            idPrefix="entity-files-panel"
            gridClassName="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
          />
        )}
      </CardContent>
      <FileDetailSheet
        fileId={selectedId}
        open={!!selectedId}
        onOpenChange={(next) => {
          if (!next) setSelectedId(null)
        }}
        onEdit={(id) =>
          navigate(`/g/${encodeURIComponent(slug)}/files/${encodeURIComponent(id)}/edit`)
        }
        imageSiblings={imageSiblings}
        onSelectSibling={(id) => setSelectedId(id)}
      />
    </Card>
  )
}
