import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams, useSearchParams } from "react-router-dom"
import { ChevronLeft, ChevronRight, Search, Trash2, Upload } from "lucide-react"

import { CategoryTiles } from "@/components/files/CategoryTiles"
import { FileCard } from "@/components/files/FileCard"
import { FileDetailSheet } from "@/components/files/FileDetailSheet"
import { TagsInput } from "@/components/files/TagsInput"
import { UploadFilesDialog } from "@/components/files/UploadFilesDialog"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import type { FileCategoryTile } from "@/features/files/constants"
import type { FileCategory, ListFilesOptions } from "@/features/files/api"
import {
  useBulkDeleteFiles,
  useBulkReclassifyFiles,
  useFileCategoryCounts,
  useFiles,
} from "@/features/files/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"

const PAGE_SIZE = 24

// Files list page — the centrepiece of #1411. Renders five category
// tiles with live counts (BE: GET /files/category-counts), a search +
// tag filter row, and a paginated grid of file cards. Selecting a file
// opens the detail sheet; selecting the checkbox enables bulk delete.
export function FilesListPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const groupSlug = currentGroup?.slug ?? ""
  const toast = useAppToast()
  const confirm = useConfirm()

  const [searchParams, setSearchParams] = useSearchParams()
  const activeTile = parseTileParam(searchParams.get("category"))
  const search = searchParams.get("q") ?? ""
  const tags = useMemo(() => splitTags(searchParams.get("tags")), [searchParams])
  const page = parsePageParam(searchParams.get("page"))

  const [pendingSearch, setPendingSearch] = useState(search)
  useEffect(() => setPendingSearch(search), [search])
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [uploadOpen, setUploadOpen] = useState(false)
  // /files/:id deep-links open the same sheet as clicking a card; the
  // route param is the source of truth so back-button + browser refresh
  // both work. Closing the sheet navigates back to /files.
  const { id: routeFileId } = useParams<{ id?: string }>()
  const detailId = routeFileId ?? null

  // activeTile is already validated against the closed tile-key set
  // (parseTileParam below), so the cast to FileCategory is sound for
  // every non-"all" branch.
  const listOpts: ListFilesOptions = {
    page,
    perPage: PAGE_SIZE,
    category: activeTile === "all" ? undefined : (activeTile as FileCategory),
    search: search || undefined,
    tags: tags.length ? tags : undefined,
  }
  const filesQuery = useFiles(listOpts, { enabled: !!groupSlug })
  const countsQuery = useFileCategoryCounts(
    { search: search || undefined, tags: tags.length ? tags : undefined },
    { enabled: !!groupSlug }
  )
  const bulkDelete = useBulkDeleteFiles()
  const bulkReclassify = useBulkReclassifyFiles()

  const items = filesQuery.data?.files ?? []
  const total = filesQuery.data?.total ?? 0
  const totalPages = total > 0 ? Math.max(1, Math.ceil(total / PAGE_SIZE)) : 1
  const allSelectedOnPage = items.length > 0 && items.every((it) => selected.has(it.file.id))
  const hasFilters = !!search || tags.length > 0 || activeTile !== "all"

  function patchParams(next: Record<string, string | null>) {
    setSearchParams((prev) => {
      const out = new URLSearchParams(prev)
      for (const [k, v] of Object.entries(next)) {
        if (v === null || v === "") out.delete(k)
        else out.set(k, v)
      }
      // Resetting filters / category implicitly resets pagination.
      if (Object.keys(next).some((k) => k !== "page")) out.delete("page")
      return out
    })
    setSelected(new Set())
  }

  function toggleOne(id: string) {
    setSelected((prev) => {
      const out = new Set(prev)
      if (out.has(id)) {
        out.delete(id)
      } else {
        out.add(id)
      }
      return out
    })
  }

  function toggleAllOnPage() {
    setSelected((prev) => {
      const out = new Set(prev)
      if (allSelectedOnPage) {
        for (const it of items) out.delete(it.file.id)
      } else {
        for (const it of items) out.add(it.file.id)
      }
      return out
    })
  }

  async function onBulkMove(category: FileCategory) {
    const ids = [...selected]
    if (!ids.length) return
    try {
      const result = await bulkReclassify.mutateAsync({ ids, category })
      setSelected(new Set())
      if (result.failed.length === 0) {
        toast.success(
          t("files:bulk.moveSuccess", {
            count: result.succeeded.length,
            defaultValue: "{{count}} files re-categorized",
          })
        )
      } else {
        toast.warning(
          t("files:bulk.movePartial", {
            succeeded: result.succeeded.length,
            failed: result.failed.length,
            defaultValue: "{{succeeded}} re-categorized, {{failed}} failed",
          })
        )
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  async function onBulkDelete() {
    const ids = [...selected]
    if (!ids.length) return
    const ok = await confirm({
      title: t("files:bulk.deleteConfirm.title", {
        count: ids.length,
        defaultValue_one: "Delete {{count}} file?",
        defaultValue_other: "Delete {{count}} files?",
      }),
      description: t("files:bulk.deleteConfirm.description"),
      confirmLabel: t("files:bulk.deleteConfirm.confirm", {
        count: ids.length,
        defaultValue_one: "Delete {{count}} file",
        defaultValue_other: "Delete {{count}} files",
      }),
      destructive: true,
    })
    if (!ok) return
    try {
      const result = await bulkDelete.mutateAsync(ids)
      setSelected(new Set())
      if (result.failed.length === 0) {
        toast.success(t("files:bulk.deleteSuccess", { count: result.succeeded.length }))
      } else {
        toast.warning(
          t("files:bulk.deletePartial", {
            succeeded: result.succeeded.length,
            failed: result.failed.length,
          })
        )
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className="space-y-6" data-testid="page-files">
      <RouteTitle title={t("files:title", { defaultValue: "Files" })} />

      <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight">
            {t("files:title", { defaultValue: "Files" })}
          </h1>
          <p className="max-w-prose text-sm text-muted-foreground">{t("files:subtitle")}</p>
        </div>
        <Button onClick={() => setUploadOpen(true)} data-testid="files-upload-cta">
          <Upload className="mr-2 size-4" aria-hidden="true" />
          {t("files:uploadCta")}
        </Button>
      </header>

      <CategoryTiles
        active={activeTile}
        counts={countsQuery.data}
        loading={countsQuery.isLoading}
        onSelect={(key) => patchParams({ category: key === "all" ? null : key })}
      />

      <div className="flex flex-col gap-3 lg:flex-row lg:items-center">
        <form
          className="flex flex-1 items-center gap-2"
          onSubmit={(e) => {
            e.preventDefault()
            patchParams({ q: pendingSearch.trim() || null })
          }}
        >
          <div className="relative flex-1">
            <Search
              className="absolute left-2 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              aria-hidden="true"
            />
            <Input
              type="search"
              value={pendingSearch}
              onChange={(e) => setPendingSearch(e.target.value)}
              placeholder={t("files:searchPlaceholder")}
              className="pl-8"
              data-testid="files-search-input"
            />
          </div>
          <Button type="submit" variant="outline">
            {t("common:nav.search")}
          </Button>
        </form>
        <div className="lg:w-72">
          <TagsInput
            placeholder={t("files:tagsPlaceholder")}
            values={tags}
            onChange={(next) => patchParams({ tags: next.length ? next.join(",") : null })}
            testId="files-tag-filter"
          />
        </div>
      </div>

      {selected.size > 0 ? (
        <div
          className="flex flex-wrap items-center justify-between gap-3 rounded-md border bg-muted/40 px-3 py-2 text-sm"
          data-testid="files-bulk-bar"
        >
          <div className="flex items-center gap-3">
            <Checkbox
              checked={allSelectedOnPage}
              onCheckedChange={toggleAllOnPage}
              aria-label={t("files:list.selectAll")}
              data-testid="files-select-all"
            />
            <span>{t("files:bulk.selected", { count: selected.size })}</span>
          </div>
          <div className="flex items-center gap-2">
            <Label htmlFor="files-bulk-move" className="sr-only">
              {t("files:bulk.move")}
            </Label>
            <select
              id="files-bulk-move"
              data-testid="files-bulk-move"
              defaultValue=""
              disabled={bulkReclassify.isPending}
              onChange={async (e) => {
                const target = e.target.value as FileCategory | ""
                e.target.value = ""
                if (!target) return
                await onBulkMove(target)
              }}
              className="h-8 rounded-md border border-input bg-transparent px-2 text-sm"
            >
              <option value="" disabled>
                {t("files:bulk.move")}
              </option>
              <option value="photos">
                {t("files:categoryPhotos", { defaultValue: "Photos" })}
              </option>
              <option value="invoices">
                {t("files:categoryInvoices", { defaultValue: "Invoices" })}
              </option>
              <option value="documents">
                {t("files:categoryDocuments", { defaultValue: "Documents" })}
              </option>
              <option value="other">{t("files:categoryOther", { defaultValue: "Other" })}</option>
            </select>
            <Button
              variant="destructive"
              size="sm"
              onClick={onBulkDelete}
              disabled={bulkDelete.isPending}
              data-testid="files-bulk-delete"
            >
              <Trash2 className="mr-2 size-4" aria-hidden="true" />
              {t("files:bulk.delete")}
            </Button>
          </div>
        </div>
      ) : null}

      {filesQuery.error ? (
        <Alert variant="destructive">
          <AlertTitle>
            {t("common:errors.generic", { defaultValue: "Something went wrong" })}
          </AlertTitle>
          <AlertDescription>{(filesQuery.error as Error).message}</AlertDescription>
        </Alert>
      ) : null}

      {filesQuery.isLoading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4">
          {Array.from({ length: 8 }).map((_, i) => (
            <Skeleton key={i} className="aspect-[4/3] w-full" />
          ))}
        </div>
      ) : items.length === 0 ? (
        <div
          className="flex flex-col items-center justify-center gap-3 rounded-md border border-dashed p-12 text-center"
          data-testid="files-empty"
        >
          <p className="text-base font-medium">
            {hasFilters ? t("files:empty.filteredTitle") : t("files:empty.title")}
          </p>
          <p className="max-w-sm text-sm text-muted-foreground">
            {hasFilters ? t("files:empty.filteredSubtitle") : t("files:empty.subtitle")}
          </p>
          {!hasFilters ? (
            <Button onClick={() => setUploadOpen(true)}>
              <Upload className="mr-2 size-4" aria-hidden="true" />
              {t("files:empty.uploadCta")}
            </Button>
          ) : null}
        </div>
      ) : (
        <div
          className="grid grid-cols-1 gap-4 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-4"
          data-testid="files-grid"
        >
          {items.map(({ file, signedUrl }) => (
            <FileCard
              key={file.id}
              file={file}
              signedUrl={signedUrl}
              selected={selected.has(file.id)}
              onToggleSelect={toggleOne}
              onOpen={(id) => navigate(filesUrl(groupSlug, id))}
            />
          ))}
        </div>
      )}

      {total > PAGE_SIZE ? (
        <nav
          className="flex items-center justify-between"
          aria-label="Pagination"
          data-testid="files-pagination"
        >
          <p className="text-sm text-muted-foreground">
            {t("files:list.showingRange", {
              from: items.length === 0 ? 0 : (page - 1) * PAGE_SIZE + 1,
              to: (page - 1) * PAGE_SIZE + items.length,
              total,
            })}
          </p>
          <div className="flex items-center gap-2">
            <Button
              variant="outline"
              size="sm"
              disabled={page <= 1}
              onClick={() => patchParams({ page: String(page - 1) })}
              aria-label={t("files:list.previousPage")}
            >
              <ChevronLeft className="size-4" aria-hidden="true" />
            </Button>
            <span className="text-sm">
              {page} / {totalPages}
            </span>
            <Button
              variant="outline"
              size="sm"
              disabled={page >= totalPages}
              onClick={() => patchParams({ page: String(page + 1) })}
              aria-label={t("files:list.nextPage")}
            >
              <ChevronRight className="size-4" aria-hidden="true" />
            </Button>
          </div>
        </nav>
      ) : null}

      <FileDetailSheet
        fileId={detailId}
        open={!!detailId}
        onOpenChange={(open) => {
          if (!open) {
            navigate(filesUrl(groupSlug))
          }
        }}
        onEdit={(id) => navigate(filesUrl(groupSlug, id, "edit"))}
      />

      <UploadFilesDialog open={uploadOpen} onOpenChange={setUploadOpen} />
    </div>
  )
}

function splitTags(raw: string | null): string[] {
  if (!raw) return []
  return raw
    .split(",")
    .map((t) => t.trim())
    .filter(Boolean)
}

// URL params are untrusted: a typo or a hand-crafted link with a junk
// `category=warranty` would otherwise reach the BE as an invalid filter
// (400) and leave every tile unselected. Fall back to `"all"` for any
// value that isn't part of the closed enum.
function parseTileParam(raw: string | null): FileCategoryTile {
  if (!raw) return "all"
  const allowed: FileCategoryTile[] = ["all", "photos", "invoices", "documents", "other"]
  return (allowed as string[]).includes(raw) ? (raw as FileCategoryTile) : "all"
}

// Same defensiveness for `?page=`. `Number("abc")` is NaN, which would
// poison the list query key and the pagination UI; `parseInt` + a
// finite-and-positive guard collapses any garbage to page 1.
function parsePageParam(raw: string | null): number {
  if (!raw) return 1
  const parsed = Number.parseInt(raw, 10)
  if (!Number.isFinite(parsed) || parsed < 1) return 1
  return parsed
}

function filesUrl(slug: string, id?: string, suffix?: "edit"): string {
  const base = slug ? `/g/${encodeURIComponent(slug)}/files` : "/files"
  if (!id) return base
  const url = `${base}/${encodeURIComponent(id)}`
  return suffix ? `${url}/${suffix}` : url
}
