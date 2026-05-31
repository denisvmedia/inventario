import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Upload } from "lucide-react"

import { FileCard, type FileCardCoverState } from "@/components/files/FileCard"
import { FilePreviewDialog } from "@/components/files/FilePreviewDialog"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import type { ListedFile } from "@/features/files/api"
import { useFiles } from "@/features/files/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"

// Files panel for entity-detail pages (commodity / location / area).
// Renders files attached to the given linked entity via the unified
// `GET /files?linked_entity_type=…&linked_entity_id=…` endpoint
// introduced for #1411 AC #4 (area linkage added under #1531 item 1).
//
// Clicking a card opens the file in place via FilePreviewDialog (image →
// fullscreen viewer, PDF → fullscreen reader, other → download card),
// keeping the user on the entity-detail page instead of routing away to
// the global `/files/:id` surface.
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
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  // Open the clicked file in place (#1963 follow-up) — image → fullscreen
  // viewer, PDF → fullscreen reader, other → download card — instead of
  // navigating away to the global /files/:id route and losing the entity
  // context (the symptom the maintainer hit from a location's photo).
  const [previewFile, setPreviewFile] = useState<ListedFile | null>(null)

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

  return (
    <Card data-testid="entity-files-panel">
      <CardHeader className="flex flex-row items-start justify-between gap-3">
        <div className="min-w-0">
          <CardTitle className="text-base">{t("files:entityPanel.title")}</CardTitle>
          <CardDescription>{t("files:entityPanel.description", { count: total })}</CardDescription>
        </div>
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
          <div
            className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3"
            data-testid="entity-files-panel-grid"
          >
            {files.map(({ file, signedUrl }) => (
              <FileCard
                key={file.id}
                file={file}
                signedUrl={signedUrl}
                onOpen={() => setPreviewFile({ file, signedUrl })}
                coverState={coverState}
                onSetCover={onSetCover}
                coverBusy={coverBusy}
              />
            ))}
          </div>
        )}
      </CardContent>
      <FilePreviewDialog file={previewFile} onClose={() => setPreviewFile(null)} />
    </Card>
  )
}
