import { Plus, Search, Shapes, X } from "lucide-react"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router-dom"

import { ItemTypeDeleteDialog } from "@/components/itemTypes/ItemTypeDeleteDialog"
import { ItemTypeFormDialog } from "@/components/itemTypes/ItemTypeFormDialog"
import { ItemTypeRow } from "@/components/itemTypes/ItemTypeRow"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Page, PageHeader } from "@/components/ui/page"
import { Skeleton } from "@/components/ui/skeleton"
import { commodityTypeHref } from "@/features/commodities/query"
import { useCurrentGroup } from "@/features/group/GroupContext"
import type { ItemType } from "@/features/itemTypes/api"
import { itemTypeLabel } from "@/features/itemTypes/display"
import { useCreateItemType, useItemTypes, useUpdateItemType } from "@/features/itemTypes/hooks"
import type { ItemTypeFormValues } from "@/features/itemTypes/schemas"
import { useAppToast } from "@/hooks/useAppToast"
import { nameKey } from "@/lib/name-key"
import { parseServerError } from "@/lib/server-error"
import { cn } from "@/lib/utils"

// The same delay as the Tags page search.
const SEARCH_DEBOUNCE_MS = 250

interface DialogState {
  open: boolean
  itemType: ItemType | null
}

// The Item types page (#2802): the group's item types with the number of
// items of each. The mock has no item types; the page follows the Tags page
// (design-mocks TagsView), as the Sellers page does: header with a create
// button, a search, and one card listing the rows.
export function ItemTypesListPage() {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const groupSlug = currentGroup?.slug ?? ""
  const toast = useAppToast()
  const [searchParams, setSearchParams] = useSearchParams()

  const urlQuery = searchParams.get("q") ?? ""
  const [pendingSearch, setPendingSearch] = useState(urlQuery)
  // Re-seed the input when the URL changes through back / forward.
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setPendingSearch(urlQuery), [urlQuery])
  useEffect(() => {
    const trimmed = pendingSearch.trim()
    if (trimmed === urlQuery) return
    const timer = window.setTimeout(() => {
      const next = new URLSearchParams(searchParams)
      if (trimmed) next.set("q", trimmed)
      else next.delete("q")
      setSearchParams(next, { replace: true })
    }, SEARCH_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [pendingSearch, urlQuery, searchParams, setSearchParams])

  const typesQuery = useItemTypes()
  const createMutation = useCreateItemType()
  const updateMutation = useUpdateItemType()
  const [dialog, setDialog] = useState<DialogState>({ open: false, itemType: null })
  const [deleting, setDeleting] = useState<ItemType | null>(null)

  const all = useMemo(() => typesQuery.data ?? [], [typesQuery.data])
  // The whole list is loaded, so the search runs here, on the names the
  // page shows.
  const visible = useMemo(() => {
    const key = nameKey(urlQuery)
    if (!key) return all
    return all.filter((type) => nameKey(itemTypeLabel(t, type)).includes(key))
  }, [all, urlQuery, t])
  const isInitialLoading = typesQuery.isLoading && !typesQuery.data

  // A failure rejects, and the dialog maps a duplicate name and field errors
  // onto its inputs.
  async function onSubmit(values: ItemTypeFormValues) {
    const editing = dialog.itemType
    if (editing) {
      await updateMutation.mutateAsync({ id: editing.id, input: values })
      toast.success(t("itemTypes:toast.updated"))
    } else {
      await createMutation.mutateAsync(values)
      toast.success(t("itemTypes:toast.created"))
    }
  }

  return (
    <Page width="wide" data-testid="page-item-types">
      <PageHeader
        title={t("itemTypes:title")}
        subtitle={t("itemTypes:description")}
        actions={
          <Button
            type="button"
            onClick={() => setDialog({ open: true, itemType: null })}
            data-testid="item-types-create-button"
          >
            <Plus className="mr-1.5 size-4" aria-hidden="true" />
            {t("itemTypes:create")}
          </Button>
        }
      />

      <div className="flex flex-col gap-4">
        <div className="relative max-w-md">
          <Label htmlFor="item-types-search-input" className="sr-only">
            {t("itemTypes:search.label")}
          </Label>
          <Search
            className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            id="item-types-search-input"
            type="search"
            value={pendingSearch}
            onChange={(e) => setPendingSearch(e.target.value)}
            placeholder={t("itemTypes:search.placeholder")}
            className={cn("pl-8", pendingSearch ? "pr-8" : undefined)}
            data-testid="item-types-search-input"
          />
          {pendingSearch ? (
            <button
              type="button"
              onClick={() => setPendingSearch("")}
              aria-label={t("itemTypes:search.clear")}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
              data-testid="item-types-search-clear"
            >
              <X aria-hidden="true" className="size-3.5" />
            </button>
          ) : null}
        </div>

        {typesQuery.isError ? (
          <div
            className="rounded-md border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive"
            role="alert"
            data-testid="item-types-list-error"
          >
            {parseServerError(typesQuery.error, t("itemTypes:list.loadError"))}
          </div>
        ) : (
          <div
            className="overflow-hidden rounded-xl border border-border bg-card"
            data-testid="item-types-list-container"
          >
            <div className="flex items-center justify-between border-b border-border px-4 py-3">
              <p
                className="text-xs font-semibold uppercase tracking-widest text-muted-foreground"
                data-testid="item-types-list-count"
              >
                {urlQuery
                  ? t("itemTypes:list.results", { count: visible.length })
                  : t("itemTypes:list.total", { count: all.length })}
              </p>
            </div>

            {isInitialLoading ? (
              <div className="flex flex-col gap-2 p-4" data-testid="item-types-list-loading">
                {Array.from({ length: 6 }).map((_, idx) => (
                  <Skeleton key={idx} className="h-10 w-full" />
                ))}
              </div>
            ) : visible.length === 0 ? (
              <div
                className="flex flex-col items-center justify-center gap-3 py-16 text-center"
                data-testid="item-types-list-empty"
              >
                <Shapes aria-hidden="true" className="size-8 text-muted-foreground/30" />
                <p className="text-sm text-muted-foreground">
                  {urlQuery
                    ? t("itemTypes:list.emptyFiltered", { query: urlQuery })
                    : t("itemTypes:list.empty")}
                </p>
              </div>
            ) : (
              <ul className="divide-y divide-border" data-testid="item-types-list">
                {visible.map((itemType) => (
                  <li key={itemType.id}>
                    <ItemTypeRow
                      itemType={itemType}
                      href={
                        groupSlug ? commodityTypeHref(groupSlug, itemType.slug ?? "") : undefined
                      }
                      onEdit={() => setDialog({ open: true, itemType })}
                      onDelete={() => setDeleting(itemType)}
                      canDelete={all.length > 1}
                    />
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </div>

      <ItemTypeFormDialog
        open={dialog.open}
        onOpenChange={(open) =>
          setDialog((prev) => (open ? prev : { open: false, itemType: prev.itemType }))
        }
        itemType={dialog.itemType}
        types={all}
        onSubmit={onSubmit}
        isPending={createMutation.isPending || updateMutation.isPending}
      />

      <ItemTypeDeleteDialog itemType={deleting} types={all} onClose={() => setDeleting(null)} />
    </Page>
  )
}
