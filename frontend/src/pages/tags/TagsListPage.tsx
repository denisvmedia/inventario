import { Plus, Search } from "lucide-react"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router-dom"

import { TagFormDialog } from "@/components/tags/TagFormDialog"
import { TagRow } from "@/components/tags/TagRow"
import { TagsStatsBar } from "@/components/tags/TagsStatsBar"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
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
      perPage: 200,
      scope: scopeForTab(urlTab),
    }),
    [urlQuery, urlSort, urlOrder, urlTab]
  )
  const tagsQuery = useTags(listOpts)
  const statsQuery = useTagStats()

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

  const createMutation = useCreateTag()
  const updateMutation = useUpdateTag()
  const deleteMutation = useDeleteTag()

  const [dialog, setDialog] = useState<DialogState>({ open: false, mode: "create" })

  const items = tagsQuery.data?.tags ?? []
  const isInitialLoading = tagsQuery.isLoading && !tagsQuery.data

  function patchSort(value: string) {
    const [field, order] = value.split(".") as [TagSortField, TagSortOrder]
    const next = new URLSearchParams(searchParams)
    next.set("sort", field)
    next.set("order", order)
    setSearchParams(next, { replace: true })
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

  async function onDelete(tag: TagEntity & { id: string }, items: number, files: number) {
    const inUse = items > 0 || files > 0
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
        items: t("tags:usage.items", { count: items }),
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

  return (
    <div className="flex flex-col gap-6 p-6" data-testid="page-tags">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h1 className="text-2xl font-semibold tracking-tight">{t("tags:title")}</h1>
          <p className="max-w-prose text-sm text-muted-foreground">{t("tags:description")}</p>
        </div>
        <Button
          type="button"
          onClick={() => setDialog({ open: true, mode: "create" })}
          data-testid="tags-create-button"
        >
          <Plus className="mr-1.5 size-4" aria-hidden="true" />
          {t("tags:create.button")}
        </Button>
      </header>

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
        {/* Render TabsContent for each value so Radix manages the
            aria-roledescription / active-state attributes, but render
            only the body once outside — keeping a single list query +
            single DOM tree avoids paying for triple-mount on every tab
            switch. */}
        <TabsContent value={urlTab} className="m-0" />
      </Tabs>

      <div className="flex flex-wrap items-end gap-3">
        <div className="relative min-w-64 flex-1">
          <Label htmlFor="tags-search-input" className="sr-only">
            {t("tags:search.label")}
          </Label>
          <Search
            className="absolute left-2 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            id="tags-search-input"
            type="search"
            value={pendingSearch}
            onChange={(e) => setPendingSearch(e.target.value)}
            placeholder={t("tags:search.placeholder")}
            className="pl-8"
            data-testid="tags-search-input"
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <Label
            htmlFor="tags-sort-select"
            className="text-xs uppercase tracking-wide text-muted-foreground"
          >
            Sort
          </Label>
          <select
            id="tags-sort-select"
            data-testid="tags-sort"
            className="h-9 rounded-md border border-input bg-background px-2 text-sm"
            value={`${urlSort}.${urlOrder}`}
            onChange={(e) => patchSort(e.target.value)}
          >
            <option value="label.asc">A → Z</option>
            <option value="label.desc">Z → A</option>
            <option value="usage.desc">Most used</option>
            <option value="created_at.desc">Newest</option>
          </select>
        </div>
      </div>

      {isInitialLoading ? (
        <div className="flex flex-col gap-2" data-testid="tags-list-loading">
          {Array.from({ length: 4 }).map((_, idx) => (
            <Skeleton key={idx} className="h-14 w-full" />
          ))}
        </div>
      ) : tagsQuery.isError ? (
        <div
          className="rounded-md border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive"
          role="alert"
          data-testid="tags-list-error"
        >
          {t("tags:list.loadError", {
            error: tagsQuery.error instanceof Error ? tagsQuery.error.message : "unknown",
          })}
        </div>
      ) : items.length === 0 ? (
        <div
          className="rounded-md border bg-muted/30 px-4 py-10 text-center text-sm text-muted-foreground"
          data-testid="tags-list-empty"
        >
          {urlQuery
            ? t("tags:list.emptyFiltered", { query: urlQuery })
            : urlTab === "commodity"
              ? t("tags:list.emptyScopeCommodity")
              : urlTab === "file"
                ? t("tags:list.emptyScopeFile")
                : t("tags:list.empty")}
        </div>
      ) : (
        <ul className="flex flex-col gap-2" data-testid="tags-list">
          {items.map(({ tag, usage }) => (
            <li key={tag.id}>
              <TagRow
                tag={tag}
                usage={usage}
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
    </div>
  )
}

// Re-exports below are intentionally minimal: the page is consumed only
// by the router. Tests import named pieces from features/tags directly.
