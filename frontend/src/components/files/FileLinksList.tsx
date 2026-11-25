import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { Box, FilePen, MapPin, Package, X } from "lucide-react"

import { Button } from "@/components/ui/button"
import { ordinaryLinks, type FileEntity, type FileLink } from "@/features/files/api"
import { useDetachFile } from "@/features/files/hooks"
import { useLinkName, useLinkTypeLabel } from "@/features/files/labels"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"

// entityPath is the detail page of a linked entity inside the group.
function entityPath(slug: string, link: FileLink): string | null {
  const id = link.entity_id
  if (!slug || !id) return null
  const base = `/g/${encodeURIComponent(slug)}`
  switch (link.entity_type) {
    case "commodity":
      return `${base}/commodities/${encodeURIComponent(id)}`
    case "location":
      return `${base}/locations/${encodeURIComponent(id)}`
    case "area":
      return `${base}/areas/${encodeURIComponent(id)}`
    case "draft":
      return `${base}/commodities/new?draft=${encodeURIComponent(id)}`
    default:
      return null
  }
}

function LinkIcon({ entityType }: { entityType?: string }) {
  const className = "size-3.5 shrink-0 text-muted-foreground"
  switch (entityType) {
    case "location":
      return <MapPin className={className} aria-hidden="true" />
    case "area":
      return <Box className={className} aria-hidden="true" />
    case "draft":
      return <FilePen className={className} aria-hidden="true" />
    default:
      return <Package className={className} aria-hidden="true" />
  }
}

export interface FileLinksListProps {
  file: FileEntity & { id: string }
}

// FileLinksList is the "Linked to" list of the file detail sheet: every item,
// location, area and open draft the file is attached to, each with a link to
// its page (a draft's opens it in the Add item dialog) and a "Remove
// attachment" action. Removing an attachment never deletes the file;
// after the last one it stays as a standalone file on the Files page.
export function FileLinksList({ file }: FileLinksListProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const { currentGroup } = useCurrentGroup()
  const typeLabel = useLinkTypeLabel()
  const linkName = useLinkName()
  const detach = useDetachFile()
  const links = ordinaryLinks(file)
  const slug = currentGroup?.slug ?? ""

  if (links.length === 0) {
    return (
      <p className="text-muted-foreground" data-testid="file-links-standalone">
        {t("files:links.standalone", { defaultValue: "Not attached to anything" })}
      </p>
    )
  }

  async function remove(link: FileLink) {
    if (!link.id) return
    const name = linkName(link)
    try {
      const updated = await detach.mutateAsync({ fileId: file.id, linkId: link.id })
      const left = ordinaryLinks(updated).length
      toast.success(
        left === 0
          ? t("files:links.removedLast", {
              name,
              defaultValue: "Removed from {{name}}. The file stays on the Files page.",
            })
          : t("files:links.removed", { name, defaultValue: "Removed from {{name}}" })
      )
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <ul className="flex flex-col gap-1" data-testid="file-links">
      {links.map((link) => {
        const name = linkName(link)
        const path = entityPath(slug, link)
        return (
          <li
            key={link.id}
            className="flex min-w-0 items-center gap-1.5"
            data-testid={`file-link-${link.entity_type}-${link.entity_id}`}
          >
            <LinkIcon entityType={link.entity_type} />
            {link.entity_type !== "draft" && (
              // A draft's name already says what it is.
              <span className="sr-only">{typeLabel(link.entity_type)}:</span>
            )}
            {path ? (
              <Link to={path} className="min-w-0 truncate underline-offset-4 hover:underline">
                {name}
              </Link>
            ) : (
              <span className="min-w-0 truncate">{name}</span>
            )}
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="ml-auto size-6 shrink-0"
              onClick={() => void remove(link)}
              disabled={detach.isPending}
              aria-label={t("files:links.remove", {
                name,
                defaultValue: "Remove attachment to {{name}}",
              })}
              title={t("files:links.removeShort", { defaultValue: "Remove attachment" })}
              data-testid={`file-link-remove-${link.entity_type}-${link.entity_id}`}
            >
              <X className="size-3.5" aria-hidden="true" />
            </Button>
          </li>
        )
      })}
    </ul>
  )
}

// useDeleteFileDescription is the body of the "Delete file" confirmation. A
// file attached to several entities goes to Trash and leaves all of them, so
// the confirmation names them and points at "Remove attachment" for the case
// where the user wants to take it off one entity only.
export function useDeleteFileDescription(): (file: FileEntity, title: string) => string {
  const { t } = useTranslation()
  const linkName = useLinkName()
  return (file, title) => {
    const links = ordinaryLinks(file)
    if (links.length === 0) {
      return t("files:detail.deleteConfirm.description", { title })
    }
    const names = links.map(linkName).join(", ")
    return t("files:detail.deleteConfirm.descriptionLinked", {
      title,
      names,
      count: links.length,
      defaultValue:
        "{{title}} moves to the trash and leaves {{names}}. Restoring it puts it back. To take it off one of them only, remove that attachment instead.",
    })
  }
}
