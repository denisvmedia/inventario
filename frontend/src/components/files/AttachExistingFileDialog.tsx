import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { Check, File as FileIcon, FileImage, FileText, Search } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { ordinaryLinks, type FileEntity, type FileLinkEntityType } from "@/features/files/api"
import { useAttachFile, useFiles } from "@/features/files/hooks"
import { useLinkSummary } from "@/features/files/labels"
import { useAppToast } from "@/hooks/useAppToast"
import { useDebouncedValue } from "@/hooks/useDebouncedValue"

const PAGE_SIZE = 20

export interface AttachExistingFileDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  entity: { type: FileLinkEntityType; id: string; name?: string }
}

function RowIcon({ file }: { file: FileEntity }) {
  const className = "size-4 shrink-0 text-muted-foreground"
  if (file.category === "images") return <FileImage className={className} aria-hidden="true" />
  if (file.category === "documents") return <FileText className={className} aria-hidden="true" />
  return <FileIcon className={className} aria-hidden="true" />
}

// AttachExistingFileDialog lets the user attach a file the group already holds
// to an item, a location or an area: one receipt for three items, one manual
// for every unit of a model. Attaching adds a link only; nothing is uploaded
// and no storage is used.
export function AttachExistingFileDialog({
  open,
  onOpenChange,
  entity,
}: AttachExistingFileDialogProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const [search, setSearch] = useState("")
  const debounced = useDebouncedValue(search)
  const linkSummary = useLinkSummary()
  const attach = useAttachFile()
  const [pendingId, setPendingId] = useState<string | null>(null)

  useEffect(() => {
    // Start from an empty search every time the dialog opens.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (!open) setSearch("")
  }, [open])

  const query = useFiles({ search: debounced, perPage: PAGE_SIZE }, { enabled: open })
  const rows = query.data?.files ?? []
  const total = query.data?.total ?? 0

  async function onAttach(file: FileEntity & { id: string }) {
    setPendingId(file.id)
    try {
      await attach.mutateAsync({ fileId: file.id, entityType: entity.type, entityId: entity.id })
      const title = file.title || file.path || file.id
      toast.success(
        entity.name
          ? t("files:attachExisting.attachedTo", {
              title,
              name: entity.name,
              defaultValue: "Attached {{title}} to {{name}}",
            })
          : t("files:attachExisting.attached", { title, defaultValue: "Attached {{title}}" })
      )
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    } finally {
      setPendingId(null)
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[80vh] sm:max-w-xl" data-testid="attach-existing-dialog">
        <DialogHeader>
          <DialogTitle>
            {entity.name
              ? t("files:attachExisting.titleWithName", {
                  name: entity.name,
                  defaultValue: "Attach an existing file to {{name}}",
                })
              : t("files:attachExisting.title", { defaultValue: "Attach an existing file" })}
          </DialogTitle>
          <DialogDescription>
            {t("files:attachExisting.description", {
              defaultValue:
                "The file is linked, not copied: it stays one file wherever it is attached and uses no extra storage.",
            })}
          </DialogDescription>
        </DialogHeader>

        <div className="relative">
          <Search
            className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            type="search"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t("files:attachExisting.searchPlaceholder", {
              defaultValue: "Search files…",
            })}
            aria-label={t("files:attachExisting.searchPlaceholder", {
              defaultValue: "Search files…",
            })}
            className="pl-8"
            data-testid="attach-existing-search"
          />
        </div>

        <div className="max-h-80 min-h-24 overflow-y-auto rounded-md border">
          {query.isLoading ? (
            <div className="flex flex-col gap-2 p-3">
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
              <Skeleton className="h-8 w-full" />
            </div>
          ) : rows.length === 0 ? (
            <p
              className="p-6 text-center text-sm text-muted-foreground"
              data-testid="attach-existing-empty"
            >
              {t("files:attachExisting.empty", { defaultValue: "No files match." })}
            </p>
          ) : (
            <ul className="divide-y">
              {rows.map(({ file }) => {
                const links = ordinaryLinks(file)
                const already = links.some(
                  (l) => l.entity_type === entity.type && l.entity_id === entity.id
                )
                const summary = linkSummary(links)
                const title = file.title || file.path || file.id
                return (
                  <li
                    key={file.id}
                    className="flex items-center gap-3 px-3 py-2 text-sm"
                    data-testid={`attach-existing-row-${file.id}`}
                  >
                    <RowIcon file={file} />
                    <div className="min-w-0 flex-1">
                      <p className="truncate font-medium" title={title}>
                        {title}
                      </p>
                      <p className="truncate text-xs text-muted-foreground">
                        {summary ||
                          t("files:links.standalone", { defaultValue: "Not attached to anything" })}
                      </p>
                    </div>
                    {already ? (
                      <Badge variant="secondary" className="gap-1">
                        <Check className="size-3" aria-hidden="true" />
                        {t("files:attachExisting.alreadyAttached", { defaultValue: "Attached" })}
                      </Badge>
                    ) : (
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => void onAttach(file)}
                        disabled={pendingId !== null}
                        aria-label={t("files:attachExisting.attachLabel", {
                          title,
                          defaultValue: "Attach {{title}}",
                        })}
                        data-testid={`attach-existing-attach-${file.id}`}
                      >
                        {t("files:attachExisting.attach", { defaultValue: "Attach" })}
                      </Button>
                    )}
                  </li>
                )
              })}
            </ul>
          )}
        </div>
        {total > rows.length ? (
          <p className="text-xs text-muted-foreground">
            {t("files:attachExisting.more", {
              count: total - rows.length,
              defaultValue: "{{count}} more files — refine the search to find them.",
            })}
          </p>
        ) : null}

        <DialogFooter>
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            data-testid="attach-existing-done"
          >
            {t("files:attachExisting.done", { defaultValue: "Done" })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
