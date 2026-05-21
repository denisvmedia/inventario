import { useEffect, useMemo, useState } from "react"
import { ArrowDown, ArrowUp, Building2, ChevronsUpDown, Search, X } from "lucide-react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAdminTenants } from "@/features/admin/hooks"
import type { AdminTenant } from "@/features/admin/api"
import { useDebouncedValue } from "@/hooks/useDebouncedValue"
import { cn } from "@/lib/utils"
import { formatDate } from "@/lib/intl"

import { TenantStatusBadge } from "./admin-shared"
import { AdminPagination } from "./AdminPagination"

// As-you-type search debounce — the typed value drives a server-side
// `?q` query, so we throttle keystrokes to one request per pause. 250ms
// matches the other search surfaces in this codebase (see TagsListPage).
const SEARCH_DEBOUNCE_MS = 250

// Page size for the server-side paginated list. The BE caps per_page at
// 100; 20 keeps the table comfortably above the fold.
const PAGE_SIZE = 20

// Sortable columns the BE accepts on GET /admin/tenants (?sort=). The
// table header cells are keyed by these; non-listed columns render as
// plain headers.
type SortField = "name" | "slug" | "status" | "created_at"
const SORTABLE: readonly SortField[] = ["name", "slug", "status", "created_at"]
const DEFAULT_SORT: SortField = "name"
const DEFAULT_ORDER: "asc" | "desc" = "asc"

function parseSort(raw: string | null): SortField {
  return SORTABLE.includes(raw as SortField) ? (raw as SortField) : DEFAULT_SORT
}

function parseOrder(raw: string | null): "asc" | "desc" {
  return raw === "desc" ? "desc" : DEFAULT_ORDER
}

function parsePage(raw: string | null): number {
  const n = Number(raw)
  return Number.isInteger(n) && n >= 1 ? n : 1
}

// AdminTenantsPage is the admin landing route (/admin/tenants). It lists
// every tenant on the platform in a paginated, sortable, searchable
// table. Sort / page / search state round-trips through the URL query
// string (?q, ?sort, ?order, ?page) so a copied link reproduces the view.
//
// Search is server-side: the debounced input feeds `?q` (which the BE
// matches against name/slug/domain). Sort + pagination are likewise
// server-side via the AdminTenantsParams the query hook forwards.
export function AdminTenantsPage() {
  const { t } = useTranslation("admin")
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()

  const urlQ = searchParams.get("q") ?? ""
  const sort = parseSort(searchParams.get("sort"))
  const order = parseOrder(searchParams.get("order"))
  const page = parsePage(searchParams.get("page"))

  // The search box keeps its own immediate state so typing stays snappy;
  // the debounced value is what reaches the URL + the server. Seeded from
  // the URL so a deep-link with ?q lands with the box pre-filled.
  const [search, setSearch] = useState(urlQ)
  const debouncedSearch = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS)

  // Push the debounced search term into the URL. Clearing it also drops
  // ?page so the user lands back on page 1 of the unfiltered list.
  useEffect(() => {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (debouncedSearch) next.set("q", debouncedSearch)
        else next.delete("q")
        if (next.get("q") !== prev.get("q")) next.delete("page")
        return next
      },
      { replace: true }
    )
  }, [debouncedSearch, setSearchParams])

  const query = useAdminTenants({
    page,
    perPage: PAGE_SIZE,
    q: urlQ || undefined,
    sort,
    order,
  })

  const tenants = useMemo(() => query.data?.tenants ?? [], [query.data])
  const meta = query.data?.meta ?? {}
  const totalTenants = meta.total ?? tenants.length
  const totalPages = meta.total_pages ?? 1

  // Out-of-range recovery: if the URL carries a ?page beyond the last
  // page the server reports (e.g. a deep link, or a search narrowed the
  // result set after the page was set), snap back to the last real page
  // so the user lands on data instead of a stranded empty state.
  useEffect(() => {
    if (query.data && page > totalPages && totalPages >= 1) {
      goToPage(totalPages)
    }
    // goToPage is stable enough for this guard; re-running on page /
    // totalPages change is what matters.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data, page, totalPages])

  // Clicking a sortable header toggles asc/desc on the active column, or
  // switches to a new column (asc). Changing the sort resets to page 1.
  function toggleSort(field: SortField) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        const nextOrder = sort === field && order === "asc" ? "desc" : "asc"
        next.set("sort", field)
        next.set("order", nextOrder)
        next.delete("page")
        return next
      },
      { replace: true }
    )
  }

  function goToPage(nextPage: number) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (nextPage <= 1) next.delete("page")
        else next.set("page", String(nextPage))
        return next
      },
      { replace: true }
    )
  }

  function clearSearch() {
    setSearch("")
  }

  return (
    <>
      <RouteTitle title={t("tenants.title")} />
      <div className="flex flex-col gap-6" data-testid="admin-tenants-page">
        <div>
          <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">
            {t("tenants.title")}
          </h1>
          <p className="mt-1 text-muted-foreground">{t("tenants.subtitle")}</p>
        </div>

        <div className="rounded-xl border border-border bg-card px-4 py-3 flex items-center gap-3">
          <div className="flex size-8 items-center justify-center rounded-lg bg-muted shrink-0">
            <Building2 className="size-4 text-muted-foreground" />
          </div>
          <div>
            <p className="text-xs text-muted-foreground">{t("tenants.stats.tenants")}</p>
            <p className="text-lg font-semibold leading-tight tabular-nums">{totalTenants}</p>
          </div>
        </div>

        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t("tenants.search.placeholder")}
            aria-label={t("tenants.search.label")}
            className="pl-8"
            data-testid="admin-tenants-search"
          />
          {search ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={clearSearch}
              className="absolute right-1.5 top-1/2 size-7 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              aria-label={t("tenants.search.clear")}
            >
              <X className="size-3.5" />
            </Button>
          ) : null}
        </div>

        {query.isError ? (
          <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-6 text-sm text-destructive">
            {t("tenants.loadError")}
          </div>
        ) : (
          <>
            <div className="rounded-xl border border-border bg-card overflow-hidden">
              <Table data-testid="admin-tenants-list">
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <SortableHead
                      field="name"
                      label={t("tenants.table.name")}
                      sort={sort}
                      order={order}
                      onSort={toggleSort}
                      className="pl-4"
                    />
                    <TableHead>{t("tenants.table.domain")}</TableHead>
                    <SortableHead
                      field="status"
                      label={t("tenants.table.status")}
                      sort={sort}
                      order={order}
                      onSort={toggleSort}
                    />
                    <TableHead className="text-right">{t("tenants.table.users")}</TableHead>
                    <TableHead className="text-right">{t("tenants.table.groups")}</TableHead>
                    <SortableHead
                      field="created_at"
                      label={t("tenants.table.created")}
                      sort={sort}
                      order={order}
                      onSort={toggleSort}
                      className="pr-4"
                    />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {query.isLoading ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={6} className="h-32 text-center text-sm text-muted-foreground">
                        {t("tenants.loading")}
                      </TableCell>
                    </TableRow>
                  ) : tenants.length === 0 ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={6} className="h-32 text-center">
                        <div className="flex flex-col items-center justify-center gap-2">
                          <Building2 className="size-8 text-muted-foreground/30" />
                          <p className="text-sm text-muted-foreground">{t("tenants.empty")}</p>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : (
                    tenants.map((tenant) => (
                      <TenantRow
                        key={tenant.id}
                        tenant={tenant}
                        onSelect={() =>
                          tenant.id && navigate(`/admin/tenants/${encodeURIComponent(tenant.id)}`)
                        }
                      />
                    ))
                  )}
                </TableBody>
              </Table>
            </div>

            {!query.isLoading && tenants.length > 0 ? (
              <AdminPagination
                page={page}
                totalPages={totalPages}
                total={totalTenants}
                pageSize={PAGE_SIZE}
                onPageChange={goToPage}
              />
            ) : null}
          </>
        )}
      </div>
    </>
  )
}

// A sortable column header. Clicking it cycles asc → desc and shows the
// active direction with an arrow; inactive sortable columns show a neutral
// up/down glyph so the affordance is discoverable.
function SortableHead({
  field,
  label,
  sort,
  order,
  onSort,
  className,
}: {
  field: SortField
  label: string
  sort: SortField
  order: "asc" | "desc"
  onSort: (field: SortField) => void
  className?: string
}) {
  const isActive = sort === field
  const Icon = isActive ? (order === "asc" ? ArrowUp : ArrowDown) : ChevronsUpDown
  return (
    <TableHead
      className={className}
      aria-sort={isActive ? (order === "asc" ? "ascending" : "descending") : "none"}
    >
      <button
        type="button"
        onClick={() => onSort(field)}
        className={cn(
          "-ml-1 inline-flex items-center gap-1 rounded px-1 py-0.5 transition-colors hover:text-foreground",
          isActive ? "text-foreground" : "text-muted-foreground"
        )}
        aria-label={label}
      >
        {label}
        <Icon className={cn("size-3.5", isActive ? "" : "opacity-50")} />
      </button>
    </TableHead>
  )
}

// A single tenant row. The whole row is a navigation affordance — clicking
// it drills into /admin/tenants/{id}.
function TenantRow({ tenant, onSelect }: { tenant: AdminTenant; onSelect: () => void }) {
  return (
    <TableRow className="cursor-pointer" onClick={onSelect} data-testid="admin-tenant-row">
      <TableCell className="pl-4 py-3.5">
        <div className="flex flex-col">
          <span className="text-sm font-medium">{tenant.name ?? "—"}</span>
          <span className="font-mono text-xs text-muted-foreground">{tenant.slug ?? ""}</span>
        </div>
      </TableCell>
      <TableCell className="py-3.5 text-sm text-muted-foreground">
        {tenant.domain || "—"}
      </TableCell>
      <TableCell className="py-3.5">
        <TenantStatusBadge status={tenant.status} />
      </TableCell>
      <TableCell className="py-3.5 text-right text-sm tabular-nums">
        {tenant.user_count ?? 0}
      </TableCell>
      <TableCell className="py-3.5 text-right text-sm tabular-nums">
        {tenant.group_count ?? 0}
      </TableCell>
      <TableCell className="pr-4 py-3.5 text-sm text-muted-foreground">
        {tenant.created_at ? formatDate(tenant.created_at) : "—"}
      </TableCell>
    </TableRow>
  )
}
