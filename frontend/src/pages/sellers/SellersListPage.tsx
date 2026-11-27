import { Plus, Search, Store, X } from "lucide-react"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router-dom"

import { SellerFormDialog } from "@/components/sellers/SellerFormDialog"
import { SellerRow } from "@/components/sellers/SellerRow"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Page, PageHeader } from "@/components/ui/page"
import { Skeleton } from "@/components/ui/skeleton"
import { commoditySellerHref } from "@/features/commodities/query"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useGroupPermissions } from "@/features/group/roles"
import { sellerNameKey, type Seller } from "@/features/sellers/api"
import {
  useCreateSeller,
  useDeleteSeller,
  useSellers,
  useUpdateSeller,
} from "@/features/sellers/hooks"
import type { SellerFormValues } from "@/features/sellers/schemas"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { parseServerError } from "@/lib/server-error"
import { cn } from "@/lib/utils"

// The same delay as the Tags page search.
const SEARCH_DEBOUNCE_MS = 250

interface DialogState {
  open: boolean
  seller: Seller | null
}

// The Sellers page (#2804): the group's sellers with the number of items
// bought from each. The mock has no sellers; the page follows the Tags page
// (design-mocks TagsView): header with a create button, a search, and one
// card listing the rows.
export function SellersListPage() {
  const { t } = useTranslation()
  // Viewers read this page; creating, renaming and deleting need the user
  // role, so they get it without those actions.
  const { canEditContent } = useGroupPermissions()
  const { currentGroup } = useCurrentGroup()
  const groupSlug = currentGroup?.slug ?? ""
  const toast = useAppToast()
  const confirm = useConfirm()
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

  const sellersQuery = useSellers()
  const createMutation = useCreateSeller()
  const updateMutation = useUpdateSeller()
  const deleteMutation = useDeleteSeller()
  const [dialog, setDialog] = useState<DialogState>({ open: false, seller: null })

  const all = useMemo(() => sellersQuery.data ?? [], [sellersQuery.data])
  // The whole list is loaded, so the search runs here, matching the name
  // the way the server's ?q= does.
  const visible = useMemo(() => {
    const key = sellerNameKey(urlQuery)
    if (!key) return all
    return all.filter((s) => sellerNameKey(s.name ?? "").includes(key))
  }, [all, urlQuery])
  const isInitialLoading = sellersQuery.isLoading && !sellersQuery.data

  // A failure rejects, and the dialog maps a duplicate name and field errors
  // onto its inputs.
  async function onSubmit(values: SellerFormValues) {
    const editing = dialog.seller
    if (editing) {
      await updateMutation.mutateAsync({ id: editing.id, input: values })
      toast.success(t("sellers:toast.updated"))
    } else {
      await createMutation.mutateAsync(values)
      toast.success(t("sellers:toast.created"))
    }
  }

  async function onDelete(seller: Seller) {
    const items = seller.item_count ?? 0
    const ok = await confirm({
      title: t("sellers:removal.confirmTitle", { name: seller.name }),
      description:
        items > 0
          ? t("sellers:removal.inUseDescription", { name: seller.name, count: items })
          : t("sellers:removal.confirmDescription", { name: seller.name }),
      confirmLabel: t("sellers:removal.confirmAction"),
      destructive: true,
    })
    if (!ok) return
    try {
      await deleteMutation.mutateAsync(seller.id)
      toast.success(t("sellers:toast.deleted"))
    } catch (err) {
      toast.error(parseServerError(err, t("sellers:toast.deleteError")))
    }
  }

  return (
    <Page width="wide" data-testid="page-sellers">
      <PageHeader
        title={t("sellers:title")}
        subtitle={t("sellers:description")}
        actions={
          canEditContent ? (
            <Button
              type="button"
              onClick={() => setDialog({ open: true, seller: null })}
              data-testid="sellers-create-button"
            >
              <Plus className="mr-1.5 size-4" aria-hidden="true" />
              {t("sellers:create")}
            </Button>
          ) : undefined
        }
      />

      <div className="flex flex-col gap-4">
        <div className="relative max-w-md">
          <Label htmlFor="sellers-search-input" className="sr-only">
            {t("sellers:search.label")}
          </Label>
          <Search
            className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            id="sellers-search-input"
            type="search"
            value={pendingSearch}
            onChange={(e) => setPendingSearch(e.target.value)}
            placeholder={t("sellers:search.placeholder")}
            className={cn("pl-8", pendingSearch ? "pr-8" : undefined)}
            data-testid="sellers-search-input"
          />
          {pendingSearch ? (
            <button
              type="button"
              onClick={() => setPendingSearch("")}
              aria-label={t("sellers:search.clear")}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
              data-testid="sellers-search-clear"
            >
              <X aria-hidden="true" className="size-3.5" />
            </button>
          ) : null}
        </div>

        {sellersQuery.isError ? (
          <div
            className="rounded-md border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive"
            role="alert"
            data-testid="sellers-list-error"
          >
            {parseServerError(sellersQuery.error, t("sellers:list.loadError"))}
          </div>
        ) : (
          <div
            className="overflow-hidden rounded-xl border border-border bg-card"
            data-testid="sellers-list-container"
          >
            <div className="flex items-center justify-between border-b border-border px-4 py-3">
              <p
                className="text-xs font-semibold uppercase tracking-widest text-muted-foreground"
                data-testid="sellers-list-count"
              >
                {urlQuery
                  ? t("sellers:list.results", { count: visible.length })
                  : t("sellers:list.total", { count: all.length })}
              </p>
            </div>

            {isInitialLoading ? (
              <div className="flex flex-col gap-2 p-4" data-testid="sellers-list-loading">
                {Array.from({ length: 4 }).map((_, idx) => (
                  <Skeleton key={idx} className="h-10 w-full" />
                ))}
              </div>
            ) : visible.length === 0 ? (
              <div
                className="flex flex-col items-center justify-center gap-3 py-16 text-center"
                data-testid="sellers-list-empty"
              >
                <Store aria-hidden="true" className="size-8 text-muted-foreground/30" />
                <p className="text-sm text-muted-foreground">
                  {urlQuery
                    ? t("sellers:list.emptyFiltered", { query: urlQuery })
                    : t("sellers:list.empty")}
                </p>
              </div>
            ) : (
              <ul className="divide-y divide-border" data-testid="sellers-list">
                {visible.map((seller) => (
                  <li key={seller.id}>
                    <SellerRow
                      seller={seller}
                      href={groupSlug ? commoditySellerHref(groupSlug, seller.id) : undefined}
                      onEdit={canEditContent ? () => setDialog({ open: true, seller }) : undefined}
                      onDelete={canEditContent ? () => void onDelete(seller) : undefined}
                    />
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </div>

      <SellerFormDialog
        open={canEditContent && dialog.open}
        onOpenChange={(open) => setDialog((prev) => (open ? prev : { open: false, seller: null }))}
        seller={dialog.seller}
        onSubmit={onSubmit}
        isPending={createMutation.isPending || updateMutation.isPending}
      />
    </Page>
  )
}
