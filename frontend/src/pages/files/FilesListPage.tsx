import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams, useSearchParams } from "react-router-dom"
import { ChevronLeft, ChevronRight, Search, Tag, Trash2, Upload, X } from "lucide-react"

import { CategoryTiles } from "@/components/files/CategoryTiles"
import { FileCollection } from "@/components/files/FileCollection"
import { FileDetailSheet } from "@/components/files/FileDetailSheet"
import { FileViewToggle } from "@/components/files/FileViewToggle"
import type { GalleryImage } from "@/components/files/ImageViewer"
import { UploadFilesDialog } from "@/components/files/UploadFilesDialog"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { BulkActionBar } from "@/components/ui/bulk-action-bar"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { NativeSelect } from "@/components/ui/native-select"
import { Page, PageHeader } from "@/components/ui/page"
import { Skeleton } from "@/components/ui/skeleton"
import {
  FILE_CATEGORY_TILES,
  FILE_TAG_PILLS,
  type FileCategoryTile,
} from "@/features/files/constants"
import { useCategoryDescription, useTagPillLabel } from "@/features/files/labels"
import {
  FILES_PAGE_SIZE,
  fileReviewHref,
  filesListOptions,
  filesListPath,
  parseFilesListParams,
} from "@/features/files/listContext"
import type { FileCategory, FileCategoryCounts } from "@/features/files/api"
import { useTagIndex } from "@/features/tags/hooks"
import { BulkTagsDialog, type BulkTagsChange } from "@/components/tags/BulkTagsDialog"
import { TagFilterMenu } from "@/components/tags/TagFilterMenu"
import {
  useBulkDeleteFiles,
  useBulkReclassifyFiles,
  useBulkUpdateFileTags,
  useFileCategoryCounts,
  useFiles,
} from "@/features/files/hooks"
import { useFilesViewMode, type FilesViewMode } from "@/features/files/useFilesViewMode"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useGroupPermissions } from "@/features/group/roles"
import { useAppToast } from "@/hooks/useAppToast"
import { toastMutationError } from "@/lib/mutation-error-toast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatBytes } from "@/lib/intl"
import { listContextFromLocation } from "@/lib/list-context"
import { cn } from "@/lib/utils"

const PAGE_SIZE = FILES_PAGE_SIZE
const VIEW_MODE_KEY = "files:viewMode"

// Files list page — the centrepiece of #1411, polished under #1538.
// Renders five category tiles with live counts (BE: GET
// /files/category-counts), a contextual subtitle row + view-mode
// toggle, search + curated tag pills, and a paginated grid OR table of
// files. Selecting a file opens the detail sheet; selecting checkboxes
// enables bulk delete / move. The grid/list toggle persists per-user
// via localStorage and is also URL-syncable for shareable links.
export function FilesListPage() {
  const { t } = useTranslation()
  // Viewers browse, open and download; uploading, selecting for the bulk
  // actions and the sheet's Edit and Delete take a role that writes files.
  const canEdit = useGroupPermissions().canEditContent
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const groupSlug = currentGroup?.slug ?? ""
  const toast = useAppToast()
  const confirm = useConfirm()
  const descriptionOf = useCategoryDescription()
  const tagPillLabelOf = useTagPillLabel()
  const fileTags = useTagIndex("file")

  const [searchParams, setSearchParams] = useSearchParams()
  // Parsed by the same helper the file review page reads its `?from=` list
  // with, so both send the same query and share the cached pages.
  const listParams = useMemo(() => parseFilesListParams(searchParams), [searchParams])
  const { tile: activeTile, search, tags, page } = listParams
  // View mode: URL `?view=` overrides; otherwise the localStorage-backed
  // pick (shared hook) survives across refreshes; default = list (per the
  // #1538 mock).
  const urlView = searchParams.get("view") as FilesViewMode | null
  const [storedView, setStoredView] = useFilesViewMode(VIEW_MODE_KEY, "list")
  const viewMode: FilesViewMode = urlView === "grid" || urlView === "list" ? urlView : storedView

  const [pendingSearch, setPendingSearch] = useState(search)
  // Re-seed input when URL search param changes (back/forward).
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setPendingSearch(search), [search])
  const [selected, setSelected] = useState<Set<string>>(new Set())
  const [uploadOpen, setUploadOpen] = useState(false)
  // /files/:id deep-links open the same sheet as clicking a card; the
  // route param is the source of truth so back-button + browser refresh
  // both work. Closing the sheet navigates back to /files with the same
  // query.
  const { id: routeFileId } = useParams<{ id?: string }>()
  const detailId = routeFileId ?? null

  const listOpts = filesListOptions(listParams)
  const filesQuery = useFiles(listOpts, { enabled: !!groupSlug })
  const countsQuery = useFileCategoryCounts(
    { search: search || undefined, tags: tags.length ? tags : undefined },
    { enabled: !!groupSlug }
  )
  const bulkDelete = useBulkDeleteFiles()
  const bulkReclassify = useBulkReclassifyFiles()
  const bulkTags = useBulkUpdateFileTags()
  const [tagsDialogOpen, setTagsDialogOpen] = useState(false)

  // useMemo with a stable reference: a `?? []` fallback would mint a
  // fresh array each render and bust the downstream tag/sibling memos.
  const items = useMemo(() => filesQuery.data?.files ?? [], [filesQuery.data?.files])
  const total = filesQuery.data?.total ?? 0
  const totalPages = total > 0 ? Math.max(1, Math.ceil(total / PAGE_SIZE)) : 1
  // A page past the end (a save that took the last file off it, or an old
  // link) goes to the last page that has files; the list would otherwise
  // show an empty page with no pagination to leave it by.
  const outOfRange =
    !!filesQuery.data && !filesQuery.isPlaceholderData && page > 1 && page > totalPages
  useEffect(() => {
    if (!outOfRange) return
    setSearchParams(
      (prev) => {
        const out = new URLSearchParams(prev)
        if (totalPages > 1) out.set("page", String(totalPages))
        else out.delete("page")
        return out
      },
      { replace: true }
    )
  }, [outOfRange, totalPages, setSearchParams])
  const allSelectedOnPage = items.length > 0 && items.every((it) => selected.has(it.file.id))
  const hasFilters = !!search || tags.length > 0 || activeTile !== "all"

  // Image siblings for the fullscreen viewer's gallery navigation. We
  // include only files with a signed URL (no URL = no rendered image)
  // and the matching MIME prefix; the order matches the list grid so
  // ←/→ in the viewer feels like "next card".
  const imageSiblings: GalleryImage[] = useMemo(
    () =>
      items
        .filter((it) => it.signedUrl?.url && it.file.mime_type?.startsWith("image/"))
        .map((it) => ({
          id: it.file.id,
          url: it.signedUrl!.url,
          alt: it.file.title?.trim() || it.file.path?.trim() || it.file.id,
        })),
    [items]
  )

  const activeTileMeta =
    FILE_CATEGORY_TILES.find((c) => c.key === activeTile) ?? FILE_CATEGORY_TILES[0]
  const ActiveIcon = activeTileMeta.icon
  // Cumulative footer numbers: when the user is on the synthetic "All"
  // tile we want the sum across categories (counts.all / counts.bytes.all);
  // otherwise the bucket totals for the active category. Both are
  // already scoped to the active search/tag filters by the BE.
  const counts = countsQuery.data
  const cumulativeBytes = counts ? bytesForKey(activeTile, counts) : undefined
  const cumulativeCount = counts ? countForKey(activeTile, counts) : total

  // Opening, switching and closing the detail panel keep the list's query, so
  // the filters, page and layout behind the panel stay as the user set them.
  function inList(path: string): string {
    const query = searchParams.toString()
    return query ? `${path}?${query}` : path
  }

  function patchParams(next: Record<string, string | null>) {
    setSearchParams((prev) => {
      const out = new URLSearchParams(prev)
      for (const [k, v] of Object.entries(next)) {
        if (v === null || v === "") out.delete(k)
        else out.set(k, v)
      }
      // Resetting filters / category implicitly resets pagination.
      // `view` is presentation-only and never resets the page.
      if (Object.keys(next).some((k) => k !== "page" && k !== "view")) out.delete("page")
      return out
    })
    setSelected(new Set())
  }

  function setViewMode(mode: FilesViewMode) {
    // localStorage persistence is handled by the shared hook; here we
    // additionally mirror the choice into the shareable `?view=` URL.
    setStoredView(mode)
    setSearchParams((prev) => {
      const out = new URLSearchParams(prev)
      out.set("view", mode)
      return out
    })
  }

  function toggleTagPill(tag: string) {
    const next = tags.includes(tag) ? tags.filter((x) => x !== tag) : [...tags, tag]
    patchParams({ tags: next.length ? next.join(",") : null })
  }

  function clearTagPills() {
    patchParams({ tags: null })
  }

  // Selected tags outside the curated pills, shown as pills of their own
  // so every condition in the URL stays visible and removable.
  const curatedTagIds = new Set<string>(FILE_TAG_PILLS.map((pill) => pill.id))
  const otherSelectedTags = tags.filter((tag) => !curatedTagIds.has(tag))

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

  // Adds and removes tags on the selected files. Files that fail stay
  // selected, so opening the dialog again retries just those. The dialog can
  // be dismissed while the request runs, and the user can then change the
  // filters or the selection: the outcome replaces the selection only while
  // it is still the one the request was sent for (the same Set, since every
  // change makes a new one).
  async function onBulkTags(change: BulkTagsChange) {
    const sent = selected
    const ids = [...sent]
    if (!ids.length) return
    try {
      const result = await bulkTags.mutateAsync({ ids, change })
      setTagsDialogOpen(false)
      // Files whose request failed may have changed: they stay selected with
      // the failed ones, to check them and send the change again.
      const keep = [...result.failed.map((f) => f.id), ...result.unknown]
      setSelected((prev) => (prev === sent ? new Set(keep) : prev))
      const unknown =
        result.unknown.length > 0
          ? { description: t("files:bulk.tagsUnknown", { count: result.unknown.length }) }
          : undefined
      if (result.failed.length > 0 || result.unknown.length > 0) {
        toast.warning(
          t("files:bulk.tagsPartial", {
            succeeded: result.succeeded.length,
            failed: result.failed.length,
          }),
          unknown
        )
      } else if (result.succeeded.length === 0) {
        toast.info(tagsUnchangedMessage(change))
      } else {
        toast.success(t("files:bulk.tagsUpdated", { count: result.succeeded.length }))
      }
    } catch (err) {
      toastMutationError(err, t("files:bulk.tagsError"))
    }
  }

  // tagsUnchangedMessage says why a tag change changed no file, naming the
  // part that did nothing: removing a tag no file has is not "already had".
  function tagsUnchangedMessage(change: BulkTagsChange): string {
    const adds = change.add.length > 0
    const removes = change.remove.length > 0
    if (adds && removes) return t("files:bulk.tagsUnchangedBoth")
    if (removes) return t("files:bulk.tagsNotPresent")
    return t("files:bulk.tagsUnchanged")
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
    <Page width="wide" data-testid="page-files">
      <RouteTitle title={t("files:title", { defaultValue: "Files" })} />
      <PageHeader
        title={t("files:title", { defaultValue: "Files" })}
        subtitle={t("files:subtitle")}
        subtitleClassName="max-w-prose"
        actions={
          canEdit ? (
            <Button
              size="sm"
              className="gap-1.5 shrink-0"
              onClick={() => setUploadOpen(true)}
              data-testid="files-upload-cta"
            >
              <Upload className="size-4" aria-hidden="true" />
              {t("files:uploadCta")}
            </Button>
          ) : undefined
        }
      />

      <CategoryTiles
        active={activeTile}
        counts={countsQuery.data}
        loading={countsQuery.isLoading}
        onSelect={(key) => patchParams({ category: key === "all" ? null : key })}
      />

      {/* Subtitle + view toggle + search + tag pills form ONE toolbar
          block (design-mocks/src/views/FileBrowserView.tsx 618-675).
          Grouping them under a single `flex flex-col gap-2` keeps the
          rhythm tight — the prior layout treated each row as a sibling
          of the page wrapper's `space-y-6`, which left a 24px gulf
          between the subtitle and the search input. */}
      <div className="flex flex-col gap-2">
        <div className="flex items-center gap-2" data-testid="files-category-subtitle">
          <div
            className={cn(
              "flex size-5 shrink-0 items-center justify-center rounded-md",
              activeTileMeta.activeBg
            )}
          >
            <ActiveIcon className={cn("size-3", activeTileMeta.activeColor)} aria-hidden="true" />
          </div>
          <p className="flex-1 text-xs text-muted-foreground">{descriptionOf(activeTile)}</p>
          <FileViewToggle value={viewMode} onChange={setViewMode} />
        </div>
        <form
          className="relative"
          onSubmit={(e) => {
            e.preventDefault()
            patchParams({ q: pendingSearch.trim() || null })
          }}
        >
          <Search
            className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
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
          {/* Submit-on-Enter only; the bare input keeps the toolbar
              compact like the mock. */}
          <button type="submit" className="sr-only">
            {t("common:nav.search")}
          </button>
        </form>

        {/* Curated tag-filter pills. Until #1400 lands a proper Tags
            entity these are the canonical taxonomy surfaced in the
            toolbar; arbitrary user-supplied tags still appear on
            individual files but aren't reachable here. */}
        <div className="flex flex-wrap items-center gap-1.5" data-testid="files-tag-pills">
          {FILE_TAG_PILLS.map((pill) => {
            const isActive = tags.includes(pill.id)
            const label = tagPillLabelOf(pill.id)
            return (
              <button
                key={pill.id}
                type="button"
                onClick={() => toggleTagPill(pill.id)}
                aria-pressed={isActive}
                data-testid={`files-tag-pill-${pill.id}`}
                className={cn(
                  "flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs font-medium transition-all",
                  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                  isActive
                    ? "border-primary bg-primary text-primary-foreground"
                    : "border-border bg-card text-muted-foreground hover:border-foreground/30 hover:text-foreground"
                )}
              >
                {label}
                {isActive ? <X className="size-3" aria-hidden="true" /> : null}
              </button>
            )
          })}
          {otherSelectedTags.map((slug) => (
            <button
              key={slug}
              type="button"
              onClick={() => toggleTagPill(slug)}
              aria-pressed
              data-testid={`files-tag-pill-${slug}`}
              className={cn(
                "flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs font-medium transition-all",
                "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
                "border-primary bg-primary text-primary-foreground"
              )}
            >
              {fileTags.resolve(slug).label}
              <X className="size-3" aria-hidden="true" />
            </button>
          ))}
          <TagFilterMenu
            label={t("files:moreTags")}
            testIdPrefix="files-tag-picker"
            tags={fileTags.tags}
            value={{ include: tags, exclude: [], mode: "all", untagged: false }}
            onChange={(next) =>
              patchParams({ tags: next.include.length ? next.include.join(",") : null })
            }
          />
          {tags.length > 0 ? (
            <button
              type="button"
              onClick={clearTagPills}
              className="ml-1 text-xs text-muted-foreground transition-colors hover:text-foreground"
              data-testid="files-tag-clear"
            >
              {t("files:tagsClearAll", { defaultValue: "Clear all" })}
            </button>
          ) : null}
        </div>
      </div>

      {canEdit && selected.size > 0 ? (
        <BulkActionBar
          label={t("files:bulk.selected", { count: selected.size })}
          selectAll={{
            checked: allSelectedOnPage,
            onCheckedChange: toggleAllOnPage,
            label: t("files:list.selectAll"),
            "data-testid": "files-select-all",
          }}
          data-testid="files-bulk-bar"
        >
          <Label htmlFor="files-bulk-move" className="sr-only">
            {t("files:bulk.move")}
          </Label>
          <NativeSelect
            id="files-bulk-move"
            size="sm"
            data-testid="files-bulk-move"
            defaultValue=""
            disabled={bulkReclassify.isPending}
            onChange={async (e) => {
              const target = e.target.value as FileCategory | ""
              e.target.value = ""
              if (!target) return
              await onBulkMove(target)
            }}
          >
            <option value="" disabled>
              {t("files:bulk.move")}
            </option>
            <option value="images">{t("files:categoryImages")}</option>
            <option value="documents">{t("files:categoryDocuments")}</option>
            <option value="other">{t("files:categoryOther")}</option>
          </NativeSelect>
          <Button
            variant="outline"
            size="sm"
            onClick={() => setTagsDialogOpen(true)}
            disabled={bulkTags.isPending}
            data-testid="files-bulk-tags"
          >
            <Tag className="mr-2 size-4" aria-hidden="true" />
            {t("files:bulk.tagsButton")}
          </Button>
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
        </BulkActionBar>
      ) : null}
      <BulkTagsDialog
        kind="file"
        open={tagsDialogOpen}
        onOpenChange={setTagsDialogOpen}
        count={selected.size}
        isPending={bulkTags.isPending}
        onSubmit={onBulkTags}
      />

      {filesQuery.error ? (
        <Alert variant="destructive">
          <AlertTitle>
            {t("common:errors.generic", { defaultValue: "Something went wrong" })}
          </AlertTitle>
          <AlertDescription>{(filesQuery.error as Error).message}</AlertDescription>
        </Alert>
      ) : null}

      {filesQuery.isLoading ? (
        viewMode === "grid" ? (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4">
            {Array.from({ length: 8 }).map((_, i) => (
              <Skeleton key={i} className="aspect-[4/3] w-full" />
            ))}
          </div>
        ) : (
          <div className="space-y-1">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-12 w-full" />
            ))}
          </div>
        )
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
          {!hasFilters && canEdit ? (
            <Button onClick={() => setUploadOpen(true)}>
              <Upload className="mr-2 size-4" aria-hidden="true" />
              {t("files:empty.uploadCta")}
            </Button>
          ) : null}
        </div>
      ) : (
        <FileCollection
          items={items}
          viewMode={viewMode}
          onOpen={(id) => navigate(inList(filesUrl(groupSlug, id)))}
          selectedIds={canEdit ? selected : undefined}
          onToggleSelect={canEdit ? toggleOne : undefined}
          allSelectedOnPage={allSelectedOnPage}
          onToggleAll={canEdit ? toggleAllOnPage : undefined}
          showListHeader
        />
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

      {/* Cumulative footer — "{N} files · {Y} total". Only shown when
          we have at least one file in the active filter set so empty
          buckets don't get a useless "0 files · 0 B total" tail. */}
      {items.length > 0 && counts && cumulativeBytes !== undefined ? (
        <p className="text-xs text-muted-foreground" data-testid="files-cumulative-footer">
          {t("files:list.footerTotal", {
            count: cumulativeCount ?? items.length,
            size: formatBytes(cumulativeBytes),
            defaultValue_one: "{{count}} file · {{size}} total",
            defaultValue_other: "{{count}} files · {{size}} total",
          })}
        </p>
      ) : null}

      <FileDetailSheet
        fileId={detailId}
        open={!!detailId}
        onOpenChange={(open) => {
          if (!open) {
            navigate(inList(filesUrl(groupSlug)))
          }
        }}
        // The review page walks this list and returns to it, so it gets
        // the list's own URL (without the open file) as its context.
        onEdit={(id) =>
          navigate(
            fileReviewHref(
              groupSlug,
              id,
              listContextFromLocation({
                pathname: filesListPath(groupSlug),
                search: searchParams.toString(),
              })
            )
          )
        }
        canEdit={canEdit}
        imageSiblings={imageSiblings}
        onSelectSibling={(id) => navigate(inList(filesUrl(groupSlug, id)))}
      />

      <UploadFilesDialog open={canEdit && uploadOpen} onOpenChange={setUploadOpen} />
    </Page>
  )
}

function filesUrl(slug: string, id?: string): string {
  const base = slug ? `/g/${encodeURIComponent(slug)}/files` : "/files"
  return id ? `${base}/${encodeURIComponent(id)}` : base
}

function countForKey(key: FileCategoryTile, counts: FileCategoryCounts): number {
  switch (key) {
    case "all":
      return counts.all ?? 0
    case "images":
      return counts.images ?? 0
    case "documents":
      return counts.documents ?? 0
    case "other":
      return counts.other ?? 0
  }
}

function bytesForKey(key: FileCategoryTile, counts: FileCategoryCounts): number | undefined {
  // Returning `undefined` (not `0`) when the BE didn't ship a `bytes`
  // sub-object — the footer's render gate checks
  // `cumulativeBytes !== undefined`, so dropping a `0` here would
  // surface a misleading "0 B total" against an unknown total.
  const bytes = counts.bytes
  if (!bytes) return undefined
  switch (key) {
    case "all":
      return bytes.all ?? 0
    case "images":
      return bytes.images ?? 0
    case "documents":
      return bytes.documents ?? 0
    case "other":
      return bytes.other ?? 0
  }
}
