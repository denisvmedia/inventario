import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { Upload } from "lucide-react"

import type { FileCardCoverState } from "@/components/files/FileCard"
import { FileCollection } from "@/components/files/FileCollection"
import { FileViewToggle } from "@/components/files/FileViewToggle"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useFiles } from "@/features/files/hooks"
import { useFilesViewMode } from "@/features/files/useFilesViewMode"
import { useCurrentGroup } from "@/features/group/GroupContext"

// Files panel for entity-detail pages (commodity / location / area).
// Renders files attached to the given linked entity via the unified
// `GET /files?linked_entity_type=…&linked_entity_id=…` endpoint
// introduced for #1411 AC #4 (area linkage added under #1531 item 1).
//
// Click on a card deep-links to `/files/:id` which mounts the same
// FileDetailSheet the main /files page uses, so detail / download /
// edit / delete all reuse the validated path.
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
  // Shared with the commodity Files tab — entity-detail surfaces default
  // to grid (they're photo-first).
  const [viewMode, setViewMode] = useFilesViewMode("files:entityViewMode", "grid")

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

  // Deep-link to the global Files detail sheet — keeps URL shareable
  // and reuses the validated detail / download / delete path.
  const handleOpen = (fileId: string) => {
    if (!slug) return
    navigate(`/g/${encodeURIComponent(slug)}/files/${encodeURIComponent(fileId)}`)
  }

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
            onOpen={handleOpen}
            coverState={coverState}
            onSetCover={onSetCover}
            coverBusy={coverBusy}
            idPrefix="entity-files-panel"
            gridClassName="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
          />
        )}
      </CardContent>
    </Card>
  )
}
