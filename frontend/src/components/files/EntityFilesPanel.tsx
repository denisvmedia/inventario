import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"

import { FileCard } from "@/components/files/FileCard"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useFiles } from "@/features/files/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"

// Read-only Files panel for entity-detail pages (commodity / location).
// Replaces the legacy `<ComingSoonBanner surface="filesUnification" />`
// placeholder by rendering files attached to the given linked entity
// via the unified `GET /files?linked_entity_type=…&linked_entity_id=…`
// endpoint introduced for #1411 AC #4.
//
// Click on a card deep-links to `/files/:id` which mounts the same
// FileDetailSheet the main /files page uses, so detail / download /
// edit / delete all reuse the validated path.
//
// No "Add files" CTA: a button that linked to the global /files
// upload would create an orphan file (no linked_entity_*) which would
// then NOT show up here — confusing UX. The proper affordance is
// drag-drop on the entity detail with the linked_entity_* preselected,
// which is the explicit scope of #1448.
export interface EntityFilesPanelProps {
  linkedEntityType: "commodity" | "location"
  linkedEntityId: string
  pageSize?: number
}

const DEFAULT_PAGE_SIZE = 24

export function EntityFilesPanel({
  linkedEntityType,
  linkedEntityId,
  pageSize = DEFAULT_PAGE_SIZE,
}: EntityFilesPanelProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""

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
      <CardHeader>
        <CardTitle className="text-base">{t("files:entityPanel.title")}</CardTitle>
        <CardDescription>{t("files:entityPanel.description", { count: total })}</CardDescription>
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
              <FileCard key={file.id} file={file} signedUrl={signedUrl} onOpen={handleOpen} />
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
