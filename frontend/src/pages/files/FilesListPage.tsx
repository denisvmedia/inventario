import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams, useSearchParams } from "react-router-dom"
import {
  ChevronLeft,
  ChevronRight,
  LayoutGrid,
  List,
  Search,
  Trash2,
  Upload,
  X,
} from "lucide-react"

import { CategoryTiles } from "@/components/files/CategoryTiles"
import { FileCard } from "@/components/files/FileCard"
import { FileDetailSheet } from "@/components/files/FileDetailSheet"
import type { GalleryImage } from "@/components/files/ImageViewer"
import { FileListRow } from "@/components/files/FileListRow"
import { UploadFilesDialog } from "@/components/files/UploadFilesDialog"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import {
  FILE_CATEGORY_TILES,
  FILE_TAG_PILLS,
  type FileCategoryTile,
} from "@/features/files/constants"
import { useCategoryDescription, useTagPillLabel } from "@/features/files/labels"
import type { FileCategory, FileCategoryCounts, ListFilesOptions } from "@/features/files/api"
import {
  useBulkDeleteFiles,
  useBulkReclassifyFiles,
  useFileCategoryCounts,
  useFiles,
} from "@/features/files/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatBytes } from "@/lib/intl"
import { cn } from "@/lib/utils"

const PAGE_SIZE = 24
const VIEW_MODE_KEY = "files:viewMode"

type ViewMode = "list" | "grid"

// Files list page — the centrepiece of #1411, polished under #1538.
// Renders five category tiles with live counts (BE: GET
// /files/category-counts), a contextual subtitle row + view-mode
// toggle, search + curated tag pills, and a paginated grid OR table of
// files. Selecting a file opens the detail sheet; selecting checkboxes
// enables bulk delete / move. The grid/list toggle persists per-user
// via localStorage and is also URL-syncable for shareable links.
export function FilesListPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const groupSlug = currentGroup?.slug ?? ""
  const toast = useAppToast()
  const confirm = useConfirm()
  const descriptionOf = useCategoryDescription()
  const tagPillLabelOf = useTagPillLabel()

  const [searchParams, setSearchParams] = useSearchParams()
  const activeTile = parseTileParam(searchParams.get("category"))
  const search = searchParams.get("q") ?? ""
  const tags = useMemo(() => splitTags(searchParams.get("tags")), [searchParams])
  const page = parsePageParam(searchParams.get("page"))
  // View mode: URL `?view=` overrides; otherwise the user's localStorage
  // pick survives across refreshes; otherwise default = list (per the
  // #1538 mock).
  const urlView = searchParams.get("view") as ViewMode | null
  const [storedView, setStoredView] = useState<ViewMode>(() => {
    if (typeof window === "undefined") return "list"
    const raw = window.localStorage.getItem(VIEW_MODE_KEY)
    return raw === "grid" || raw === "list" ? raw : "list"
  })
  const viewMode: ViewMode = urlView === "grid" || urlView === "list" ? urlView : storedView

  const [pendingSearch, setPendingSearch] = useState(search)
  // Re-seed input when URL search param changes (back/forward).
  // eslint-disable-next-line react-hooks/set-state-in-effect
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

  // useMemo with a stable reference: a `?? []` fallback would mint a
  // fresh array each render and bust the downstream tag/sibling memos.
  const items = useMemo(() => filesQuery.data?.files ?? [], [filesQuery.data?.files])
  const total = filesQuery.data?.total ?? 0
  const totalPages = total > 0 ? Math.max(1, Math.ceil(total / PAGE_SIZE)) : 1
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

  function setViewMode(mode: ViewMode) {
    setStoredView(mode)
    if (typeof window !== "undefined") {
      window.localStorage.setItem(VIEW_MODE_KEY, mode)
    }
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

      {/* Mock-aligned page header (design-mocks/src/views/FileBrowserView.tsx
          lines 531-542): h1 uses the canonical scroll-m-20 text-3xl
          treatment; the lede sits directly under it; the upload button
          is the size="sm" outline-less primary action on the right edge
          of the title row (with `shrink-0` so a long lede never squeezes
          it out of position). */}
      <header className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">
            {t("files:title", { defaultValue: "Files" })}
          </h1>
          <p className="mt-1 max-w-prose text-muted-foreground">{t("files:subtitle")}</p>
        </div>
        <Button
          size="sm"
          className="gap-1.5 shrink-0"
          onClick={() => setUploadOpen(true)}
          data-testid="files-upload-cta"
        >
          <Upload className="size-4" aria-hidden="true" />
          {t("files:uploadCta")}
        </Button>
      </header>

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
          <div className="flex gap-1">
            <Button
              variant={viewMode === "list" ? "secondary" : "ghost"}
              size="icon"
              className="size-8"
              onClick={() => setViewMode("list")}
              aria-label={t("files:view.list", { defaultValue: "List view" })}
              aria-pressed={viewMode === "list"}
              data-testid="files-view-list"
            >
              <List className="size-4" aria-hidden="true" />
            </Button>
            <Button
              variant={viewMode === "grid" ? "secondary" : "ghost"}
              size="icon"
              className="size-8"
              onClick={() => setViewMode("grid")}
              aria-label={t("files:view.grid", { defaultValue: "Grid view" })}
              aria-pressed={viewMode === "grid"}
              data-testid="files-view-grid"
            >
              <LayoutGrid className="size-4" aria-hidden="true" />
            </Button>
          </div>
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

      {/* Bulk-action toolbar — a fixed bottom-centre overlay so toggling
          the first checkbox doesn't reflow the list (no "jolt"). The
          shadcn `popover` token already encodes the floating-surface
          elevation, which keeps us off bespoke `shadow-*` per the design
          rules. Slide-in animation via `tw-animate-css`. The bar is
          preserved as an intentional `mock < reality` divergence
          (BulkBar isn't in design-mocks/src/views/FileBrowserView.tsx) —
          see devdocs/frontend/design-deviations.md. */}
      {selected.size > 0 ? (
        <div
          role="region"
          aria-label={t("files:bulk.selected", { count: selected.size })}
          className={cn(
            "fixed bottom-6 left-1/2 z-40 w-[calc(100vw-2rem)] max-w-xl -translate-x-1/2",
            "flex flex-wrap items-center justify-between gap-3 rounded-xl border bg-popover px-4 py-2.5 text-sm text-popover-foreground",
            "animate-in slide-in-from-bottom-4 fade-in-0 duration-200"
          )}
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
              <option value="images">{t("files:categoryImages")}</option>
              <option value="documents">{t("files:categoryDocuments")}</option>
              <option value="other">{t("files:categoryOther")}</option>
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
          {!hasFilters ? (
            <Button onClick={() => setUploadOpen(true)}>
              <Upload className="mr-2 size-4" aria-hidden="true" />
              {t("files:empty.uploadCta")}
            </Button>
          ) : null}
        </div>
      ) : viewMode === "grid" ? (
        <div
          className="grid grid-cols-1 gap-3 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4"
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
      ) : (
        <div className="overflow-hidden rounded-xl border bg-card" data-testid="files-list">
          {/* Desktop header row — hidden on mobile (rows collapse). The
              gap-4/px-4 rhythm mirrors the mock (FileBrowserView.tsx
              ~line 692) and keeps the header aligned with FileListRow's
              `grid-cols-subgrid` body row. */}
          <div className="hidden grid-cols-[auto_auto_1fr_auto_auto_auto] gap-4 border-b bg-muted/50 px-4 py-2 sm:grid">
            <div>
              <Checkbox
                checked={allSelectedOnPage}
                onCheckedChange={toggleAllOnPage}
                aria-label={t("files:list.selectAll")}
                data-testid="files-list-select-all"
              />
            </div>
            <span className="size-4" aria-hidden="true" />
            <span className="text-xs font-medium text-muted-foreground">
              {t("files:list.columnName", { defaultValue: "Name" })}
            </span>
            <span className="w-24 text-center text-xs font-medium text-muted-foreground">
              {t("files:list.columnCategory", { defaultValue: "Category" })}
            </span>
            <span className="w-28 text-right text-xs font-medium text-muted-foreground">
              {t("files:list.columnUploaded", { defaultValue: "Uploaded" })}
            </span>
            <span className="w-16 text-right text-xs font-medium text-muted-foreground">
              {t("files:list.columnSize", { defaultValue: "Size" })}
            </span>
          </div>
          <ul className="divide-y">
            {items.map(({ file }) => (
              <FileListRow
                key={file.id}
                file={file}
                selected={selected.has(file.id)}
                onToggleSelect={toggleOne}
                onOpen={(id) => navigate(filesUrl(groupSlug, id))}
              />
            ))}
          </ul>
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
            navigate(filesUrl(groupSlug))
          }
        }}
        onEdit={(id) => navigate(filesUrl(groupSlug, id, "edit"))}
        imageSiblings={imageSiblings}
        onSelectSibling={(id) => navigate(filesUrl(groupSlug, id))}
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
  const allowed: FileCategoryTile[] = ["all", "images", "documents", "other"]
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
