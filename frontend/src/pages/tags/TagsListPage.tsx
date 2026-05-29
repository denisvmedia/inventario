import { Plus, Search, Tag as TagIcon, X } from "lucide-react"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router-dom"

import { TagBadge } from "@/components/tags/TagBadge"
import { TagFormDialog } from "@/components/tags/TagFormDialog"
import { TagInlineCreate } from "@/components/tags/TagInlineCreate"
import { TagRow, type TagRowPreviewItem } from "@/components/tags/TagRow"
import { TagsStatsBar } from "@/components/tags/TagsStatsBar"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Page, PageHeader } from "@/components/ui/page"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useCommodities } from "@/features/commodities/hooks"
import {
  type CreateTagRequest,
  type TagColor,
  type TagEntity,
  type TagScope,
  type TagSortField,
  type TagSortOrder,
  type UpdateTagRequest,
} from "@/features/tags/api"
import {
  useCreateTag,
  useDeleteTag,
  useTagStats,
  useTags,
  useUpdateTag,
} from "@/features/tags/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { cn } from "@/lib/utils"

// Search-input debounce keeps the page responsive on every keystroke
// while still reflecting the typed value in the URL (so refreshes /
// shareable links survive). 250ms is the same delay other "as-you-type"
// surfaces in this codebase use.
const SEARCH_DEBOUNCE_MS = 250

// Allowlist for ?sort= / ?order= query params — guards against a
// hand-edited URL or a stale shared link slipping an unsupported value
// through to the BE (which would 4xx) or rendering a blank `<select>`.
const VALID_SORT_FIELDS = [
  "label",
  "created_at",
  "usage",
] as const satisfies readonly TagSortField[]
const VALID_SORT_ORDERS = ["asc", "desc"] as const satisfies readonly TagSortOrder[]

// Cap on commodities pulled for preview-chip aggregation. Each tag row
// surfaces up to 2 sample items + an overflow count from `usage`, so we
// only need enough rows to find a hit for each tag. The BE's
// `parsePagination` (go/apiserver/apiserver.go) caps `per_page` at 100
// — anything higher silently falls back to the default 50, so 100 is
// the practical upper bound. Groups with more than 100 commodities will
// see empty chip strips on rows whose tagged commodities all sit
// outside the first page; the authoritative count still surfaces via
// `usage.commodities` (so "no chips but N items" still tells the truth).
// We send `include_inactive=true` because tags can be attached to draft
// or written-off items too.
const PREVIEW_COMMODITIES_LIMIT = 100
const PREVIEW_ITEMS_PER_TAG = 2

// Cap on the unfiltered-tag companion query. 100 matches the BE's
// per_page ceiling; realistic per-group tag counts (typically < 30)
// sit comfortably below this. Groups with more than 100 tags will see
// the Preview footer / inline duplicate-slug check truncate at the
// hundredth — acceptable for v1; revisit if real groups exceed this.
const ALL_TAGS_FETCH_LIMIT = 100

function parseSort(raw: string | null): TagSortField {
  return (VALID_SORT_FIELDS as readonly string[]).includes(raw ?? "")
    ? (raw as TagSortField)
    : "label"
}

function parseOrder(raw: string | null): TagSortOrder {
  return (VALID_SORT_ORDERS as readonly string[]).includes(raw ?? "")
    ? (raw as TagSortOrder)
    : "asc"
}

// Local tab id — "all" stays in the URL as the explicit no-filter
// sentinel; the data layer turns it into "no scope param" before the
// request. "commodity" / "file" mirror the BE's wire contract.
type TabId = "all" | TagScope
const VALID_TABS = ["all", "commodity", "file"] as const satisfies readonly TabId[]

function parseTab(raw: string | null): TabId {
  return (VALID_TABS as readonly string[]).includes(raw ?? "") ? (raw as TabId) : "all"
}

function scopeForTab(tab: TabId): TagScope | undefined {
  return tab === "all" ? undefined : tab
}

interface DialogState {
  open: boolean
  mode: "create" | "edit"
  tag?: TagEntity & { id: string }
}

export function TagsListPage() {
  const { t } = useTranslation(["tags", "common"])
  const toast = useAppToast()
  const confirm = useConfirm()
  const [searchParams, setSearchParams] = useSearchParams()

  const urlQuery = searchParams.get("q") ?? ""
  const urlSort = parseSort(searchParams.get("sort"))
  const urlOrder = parseOrder(searchParams.get("order"))
  const urlTab = parseTab(searchParams.get("tab"))

  const [pendingSearch, setPendingSearch] = useState(urlQuery)
  // Re-seed the input when the URL query changes via back/forward — this
  // is a controlled-input sync from URL state. The cascading render is
  // bounded by URL changes.
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setPendingSearch(urlQuery), [urlQuery])

  // Debounced URL update — write the typed search into the URL after the
  // user stops typing. The list query is keyed off the URL value (via
  // useTags below), so this also drives refetching.
  useEffect(() => {
    const trimmed = pendingSearch.trim()
    if (trimmed === urlQuery) return
    const timer = window.setTimeout(() => {
      const next = new URLSearchParams(searchParams)
      if (trimmed) {
        next.set("q", trimmed)
      } else {
        next.delete("q")
      }
      setSearchParams(next, { replace: true })
    }, SEARCH_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [pendingSearch, urlQuery, searchParams, setSearchParams])

  const listOpts = useMemo(
    () => ({
      search: urlQuery || undefined,
      sort: urlSort,
      order: urlOrder,
      includeUsage: true,
      // BE caps per_page at 100 (parsePagination); ask for the max so
      // small/medium groups render in one page. Paging arrives in a
      // follow-up if real groups exceed 100 tags.
      perPage: 100,
      scope: scopeForTab(urlTab),
    }),
    [urlQuery, urlSort, urlOrder, urlTab]
  )
  const tagsQuery = useTags(listOpts)
  // Unfiltered companion query — backs the duplicate-slug check inside
  // the inline-create row and the Preview footer pill grid. Without it,
  // both consumers see only the search-filtered (and paginated) slice
  // and would (a) miss collisions for tags outside the current view
  // and (b) shrink the pill grid down to the search results, which
  // defeats the whole "all-tags palette" purpose of the footer.
  const allTagsQuery = useTags(
    useMemo(
      () => ({
        sort: "label" as TagSortField,
        order: "asc" as TagSortOrder,
        perPage: ALL_TAGS_FETCH_LIMIT,
      }),
      []
    )
  )
  const statsQuery = useTagStats()

  // Commodities pull powers the per-row preview chips. We aggregate
  // client-side because the BE doesn't expose a tag-filtered commodities
  // index — same trade-off as #1531 area-count aggregation. The query
  // is non-blocking: chips simply don't render until the data lands.
  const previewCommoditiesQuery = useCommodities({
    perPage: PREVIEW_COMMODITIES_LIMIT,
    includeInactive: true,
  })

  const previewByTag = useMemo(() => {
    const map = new Map<string, TagRowPreviewItem[]>()
    const items = previewCommoditiesQuery.data?.commodities ?? []
    for (const c of items) {
      const slugs = (c.tags as string[] | undefined) ?? []
      const display = c.short_name?.trim() || c.name?.trim() || ""
      if (!display || !c.id) continue
      for (const slug of slugs) {
        if (!slug) continue
        const bucket = map.get(slug)
        if (bucket && bucket.length >= PREVIEW_ITEMS_PER_TAG) continue
        const entry: TagRowPreviewItem = { id: c.id, name: display }
        if (bucket) bucket.push(entry)
        else map.set(slug, [entry])
      }
    }
    return map
  }, [previewCommoditiesQuery.data])

  const createMutation = useCreateTag()
  const updateMutation = useUpdateTag()
  const deleteMutation = useDeleteTag()

  const [dialog, setDialog] = useState<DialogState>({ open: false, mode: "create" })

  const items = useMemo(() => tagsQuery.data?.tags ?? [], [tagsQuery.data])
  const totalMatches = tagsQuery.data?.total ?? items.length
  const isInitialLoading = tagsQuery.isLoading && !tagsQuery.data

  const allTags = useMemo(
    () => allTagsQuery.data?.tags?.map(({ tag }) => tag) ?? [],
    [allTagsQuery.data]
  )
  // existingSlugs comes from the unfiltered tag set, not from `items`,
  // so the duplicate-slug check inside the inline-create row catches
  // collisions even when the search filter, scope tab, or perPage cap
  // hides the existing tag from the current view.
  const existingSlugs = useMemo(
    () => new Set(allTags.map((tag) => tag.slug ?? "").filter(Boolean)),
    [allTags]
  )

  function patchSort(value: string) {
    const [field, order] = value.split(".") as [TagSortField, TagSortOrder]
    const next = new URLSearchParams(searchParams)
    next.set("sort", field)
    next.set("order", order)
    setSearchParams(next, { replace: true })
  }

  function patchTab(next: string) {
    const tab = parseTab(next)
    const params = new URLSearchParams(searchParams)
    if (tab === "all") {
      params.delete("tab")
    } else {
      params.set("tab", tab)
    }
    setSearchParams(params, { replace: true })
  }

  async function onInlineCreate(values: { label: string; slug: string; color: TagColor }) {
    try {
      await createMutation.mutateAsync(values)
      toast.success(t("tags:form.createSuccess"))
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      toast.error(t("tags:form.createError", { error: message }))
      throw err
    }
  }

  async function onCreateSubmit(values: CreateTagRequest) {
    try {
      await createMutation.mutateAsync(values)
      toast.success(t("tags:form.createSuccess"))
      setDialog({ open: false, mode: "create" })
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      toast.error(t("tags:form.createError", { error: message }))
    }
  }

  async function onEditSubmit(id: string, values: UpdateTagRequest) {
    try {
      await updateMutation.mutateAsync({ id, req: values })
      toast.success(t("tags:form.updateSuccess"))
      setDialog({ open: false, mode: "create" })
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      toast.error(t("tags:form.updateError", { error: message }))
    }
  }

  async function onDelete(tag: TagEntity & { id: string }, itemsCount: number, files: number) {
    const inUse = itemsCount > 0 || files > 0
    if (!inUse) {
      const ok = await confirm({
        title: t("tags:removal.confirmTitle"),
        description: t("tags:removal.confirmDescription", { label: tag.label }),
        confirmLabel: t("tags:removal.confirmAction"),
        destructive: true,
      })
      if (!ok) return
      try {
        await deleteMutation.mutateAsync({ id: tag.id })
        toast.success(t("tags:removal.deleteSuccess"))
      } catch (err) {
        toast.error(
          t("tags:removal.deleteError", {
            error: err instanceof Error ? err.message : String(err),
          })
        )
      }
      return
    }

    // In-use → confirm with stripped-references warning, then force.
    const ok = await confirm({
      title: t("tags:removal.inUseTitle"),
      description: t("tags:removal.inUseDescription", {
        label: tag.label,
        items: t("tags:usage.items", { count: itemsCount }),
        files: t("tags:usage.files", { count: files }),
      }),
      confirmLabel: t("tags:removal.forceAction"),
      cancelLabel: t("tags:removal.forceCancel"),
      destructive: true,
    })
    if (!ok) return
    try {
      await deleteMutation.mutateAsync({ id: tag.id, force: true })
      toast.success(t("tags:removal.deleteSuccess"))
    } catch (err) {
      toast.error(
        t("tags:removal.deleteError", {
          error: err instanceof Error ? err.message : String(err),
        })
      )
    }
  }

  // Both the count line and the Preview footer must reflect the
  // server-reported total (not `items.length`, which is capped by
  // perPage) and the unfiltered tag set (not the search/scope-filtered
  // view).
  const filteredCount = totalMatches
  const allTagsForPreview = allTags

  // emptyMessage picks the right key for the empty list — scope-aware
  // when no search query is set so users on the commodity/file tab see
  // a "no tags here yet for this scope" message rather than the
  // generic empty state.
  function emptyMessage(): string {
    if (urlQuery) return t("tags:list.emptyFiltered", { query: urlQuery })
    if (urlTab === "commodity") return t("tags:list.emptyScopeCommodity")
    if (urlTab === "file") return t("tags:list.emptyScopeFile")
    return t("tags:list.empty")
  }

  // tabBody is the toolbar + list payload mounted inside the active
  // TabsContent. Defined once and referenced from all three tab panels
  // so each TabsTrigger has a real aria-controls target (Radix mounts
  // only the active panel's children, so there is no triple-render).
  const tabBody = (
    <div className="flex flex-col gap-4 pt-3">
      <div className="flex flex-wrap items-end gap-3">
        <div className="relative min-w-64 flex-1">
          <Label htmlFor="tags-search-input" className="sr-only">
            {t("tags:search.label")}
          </Label>
          <Search
            className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            id="tags-search-input"
            type="search"
            value={pendingSearch}
            onChange={(e) => setPendingSearch(e.target.value)}
            placeholder={t("tags:search.placeholder")}
            // Right padding reserves space for the absolute-positioned
            // clear button when the input has a value, so typed text /
            // the caret never collide with the X glyph.
            className={cn("pl-8", pendingSearch ? "pr-8" : undefined)}
            data-testid="tags-search-input"
          />
          {pendingSearch ? (
            <button
              type="button"
              onClick={() => setPendingSearch("")}
              aria-label={t("tags:search.clear")}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
              data-testid="tags-search-clear"
            >
              <X aria-hidden="true" className="size-3.5" />
            </button>
          ) : null}
        </div>
        <div className="flex flex-col gap-1.5">
          <Label
            htmlFor="tags-sort-select"
            className="text-xs uppercase tracking-wide text-muted-foreground"
          >
            {t("tags:sort.label")}
          </Label>
          {/* eslint-disable-next-line no-restricted-syntax -- list sort utility selector; native <select> retained, covered by native-select unit (TagsListPage.test) */}
          <select
            id="tags-sort-select"
            data-testid="tags-sort"
            className="h-9 rounded-md border border-input bg-background px-2 text-sm"
            value={`${urlSort}.${urlOrder}`}
            onChange={(e) => patchSort(e.target.value)}
          >
            <option value="label.asc">{t("tags:sort.labelAsc")}</option>
            <option value="label.desc">{t("tags:sort.labelDesc")}</option>
            <option value="usage.desc">{t("tags:sort.mostUsed")}</option>
            <option value="created_at.desc">{t("tags:sort.newest")}</option>
          </select>
        </div>
      </div>

      {tagsQuery.isError ? (
        <div
          className="rounded-md border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive"
          role="alert"
          data-testid="tags-list-error"
        >
          {t("tags:list.loadError", {
            error: tagsQuery.error instanceof Error ? tagsQuery.error.message : "unknown",
          })}
        </div>
      ) : (
        <div
          className="rounded-xl border border-border bg-card overflow-hidden"
          data-testid="tags-list-container"
        >
          <div className="px-4 py-3 border-b border-border flex items-center justify-between">
            <p
              className="text-xs font-semibold uppercase tracking-widest text-muted-foreground"
              data-testid="tags-list-count"
            >
              {urlQuery
                ? t("tags:list.results", { count: filteredCount })
                : t("tags:list.totalTags", { count: totalMatches })}
            </p>
          </div>

          {isInitialLoading ? (
            <div className="flex flex-col gap-2 p-4" data-testid="tags-list-loading">
              {Array.from({ length: 4 }).map((_, idx) => (
                <Skeleton key={idx} className="h-10 w-full" />
              ))}
            </div>
          ) : items.length === 0 ? (
            <div
              className="flex flex-col items-center justify-center gap-3 py-16 text-center"
              data-testid="tags-list-empty"
            >
              <TagIcon aria-hidden="true" className="size-8 text-muted-foreground/30" />
              <p className="text-sm text-muted-foreground">{emptyMessage()}</p>
            </div>
          ) : (
            <ul className="divide-y divide-border" data-testid="tags-list">
              {items.map(({ tag, usage }) => (
                <li key={tag.id}>
                  <TagRow
                    tag={tag}
                    usage={usage}
                    previewItems={previewByTag.get(tag.slug ?? "") ?? []}
                    onEdit={() =>
                      setDialog({
                        open: true,
                        mode: "edit",
                        tag,
                      })
                    }
                    onDelete={() => onDelete(tag, usage?.commodities ?? 0, usage?.files ?? 0)}
                  />
                </li>
              ))}
            </ul>
          )}

          <Separator />
          <div className="px-4 py-3">
            <TagInlineCreate
              existingSlugs={existingSlugs}
              onCreate={onInlineCreate}
              isPending={createMutation.isPending}
            />
          </div>
        </div>
      )}
    </div>
  )

  return (
    <Page width="wide" data-testid="page-tags">
      <PageHeader
        title={t("tags:title")}
        subtitle={t("tags:description")}
        actions={
          <Button
            type="button"
            onClick={() => setDialog({ open: true, mode: "create" })}
            data-testid="tags-create-button"
          >
            <Plus className="mr-1.5 size-4" aria-hidden="true" />
            {t("tags:create.button")}
          </Button>
        }
      />

      <TagsStatsBar stats={statsQuery.data} loading={statsQuery.isLoading} />

      <Tabs value={urlTab} onValueChange={patchTab} className="gap-3">
        <TabsList data-testid="tags-tabs">
          <TabsTrigger value="all" data-testid="tags-tab-all">
            {t("tags:tabs.all")}
          </TabsTrigger>
          <TabsTrigger value="commodity" data-testid="tags-tab-commodity">
            {t("tags:tabs.commodity")}
          </TabsTrigger>
          <TabsTrigger value="file" data-testid="tags-tab-file">
            {t("tags:tabs.file")}
          </TabsTrigger>
        </TabsList>
        {/* One TabsContent per trigger value so each TabsTrigger's
            aria-controls resolves to a real tabpanel element (Radix
            otherwise skips inactive panels entirely, leaving the
            triggers' aria-controls dangling and tripping axe). Only
            the active panel mounts its children — `tabBody` is the
            same JSX in each slot but React + Radix mount it exactly
            once per active value, so there is no triple-render cost.
            Scoping happens via `urlTab → listOpts.scope`, which is
            evaluated outside the Tabs subtree, so the actual data
            query also runs only once per tab. */}
        <TabsContent value="all" className="m-0" data-testid="tags-tab-all-content">
          {tabBody}
        </TabsContent>
        <TabsContent value="commodity" className="m-0" data-testid="tags-tab-commodity-content">
          {tabBody}
        </TabsContent>
        <TabsContent value="file" className="m-0" data-testid="tags-tab-file-content">
          {tabBody}
        </TabsContent>
      </Tabs>

      {allTagsForPreview.length > 0 ? (
        <div
          className="rounded-xl border border-border bg-card px-4 py-4 space-y-3"
          data-testid="tags-preview"
        >
          <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
            {t("tags:preview.heading")}
          </p>
          <div className="flex flex-wrap gap-1.5">
            {allTagsForPreview.map((tag) => (
              <TagBadge
                key={tag.id}
                label={tag.label ?? tag.slug ?? ""}
                color={(tag.color ?? "muted") as TagColor}
                testId={`tags-preview-${tag.slug}`}
              />
            ))}
          </div>
        </div>
      ) : null}

      <TagFormDialog
        open={dialog.open}
        onOpenChange={(open) =>
          setDialog((prev) => (open ? prev : { open: false, mode: "create" }))
        }
        mode={dialog.mode}
        initialValues={dialog.tag}
        isPending={createMutation.isPending || updateMutation.isPending}
        onSubmit={async (values) => {
          if (dialog.mode === "edit" && dialog.tag) {
            await onEditSubmit(dialog.tag.id, {
              label: values.label,
              slug: values.slug,
              color: values.color as TagColor,
            })
          } else {
            await onCreateSubmit(values)
          }
        }}
      />
    </Page>
  )
}
