import { useEffect, useMemo, useState } from "react"
import { ArrowDown, ArrowUp, Building2, ChevronsUpDown, Layers, Search, X } from "lucide-react"
import { useNavigate, useSearchParams } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAdminGroups, useAdminTenants } from "@/features/admin/hooks"
import type { AdminGroup } from "@/features/admin/api"
import { useDebouncedValue } from "@/hooks/useDebouncedValue"
import { cn } from "@/lib/utils"
import { formatDate } from "@/lib/intl"

import { GroupStatusBadge } from "./admin-shared"
import { AdminPagination } from "./AdminPagination"

// As-you-type search debounce — the typed value drives a server-side
// `?q` query, so we throttle keystrokes to one request per pause. 250ms
// matches the other search surfaces in this codebase (see AdminTenantsPage).
const SEARCH_DEBOUNCE_MS = 250

// Page size for the server-side paginated list. The BE caps per_page at
// 100; 20 keeps the table comfortably above the fold.
const PAGE_SIZE = 20

// Per-page count for the tenant-filter <Select> options. The cross-tenant
// group list lets the operator pin one owning tenant; we pull the tenant
// list to populate that dropdown. 100 is the BE's per_page cap — see the
// note in design-deviations.md on the rare >100-tenant deployment.
const TENANT_FILTER_PER_PAGE = 100

// Sortable columns the BE accepts on GET /admin/groups (?sort=). The table
// header cells are keyed by these; non-listed columns render as plain
// headers.
type SortField = "name" | "slug" | "created_at" | "status"
const SORTABLE: readonly SortField[] = ["name", "slug", "created_at", "status"]
const DEFAULT_SORT: SortField = "name"
const DEFAULT_ORDER: "asc" | "desc" = "asc"

type StatusFilter = "active" | "pending_deletion"

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

function parseStatus(raw: string | null): StatusFilter | undefined {
  return raw === "active" || raw === "pending_deletion" ? raw : undefined
}

// AdminGroupsPage is the /admin/groups route. It lists every location
// group across every tenant in a paginated, sortable, searchable table.
// Search / tenant filter / status filter / sort / page all round-trip
// through the URL query string (?q, ?tenantID, ?status, ?sort, ?order,
// ?page) so a copied link reproduces the view.
//
// Search is server-side: the debounced input feeds `?q` (which the BE
// matches against name/slug). Filters, sort + pagination are likewise
// server-side via the AdminGroupsParams the query hook forwards.
export function AdminGroupsPage() {
  const { t } = useTranslation("admin")
  const navigate = useNavigate()
  const [searchParams, setSearchParams] = useSearchParams()

  const urlQ = searchParams.get("q") ?? ""
  const tenantID = searchParams.get("tenantID") ?? ""
  const status = parseStatus(searchParams.get("status"))
  const sort = parseSort(searchParams.get("sort"))
  const order = parseOrder(searchParams.get("order"))
  const page = parsePage(searchParams.get("page"))

  // The search box keeps its own immediate state so typing stays snappy;
  // the debounced value is what reaches the URL + the server. Seeded from
  // the URL so a deep-link with ?q lands with the box pre-filled.
  const [search, setSearch] = useState(urlQ)
  // Re-seed the input when the URL `q` changes via back/forward (or a
  // deep-link nav) — a controlled-input sync from URL state. Bounded by
  // URL changes; does not fight the debounced search → URL effect below.
  // Mirrors AdminTenantsPage.
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setSearch(urlQ), [urlQ])
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

  // The tenant filter <Select> needs the tenant list. One large page
  // covers every realistic deployment; the >100-tenant edge is logged in
  // design-deviations.md.
  const tenantsQuery = useAdminTenants({ page: 1, perPage: TENANT_FILTER_PER_PAGE, sort: "name" })
  const tenantOptions = useMemo(() => tenantsQuery.data?.tenants ?? [], [tenantsQuery.data])

  const query = useAdminGroups({
    page,
    perPage: PAGE_SIZE,
    q: urlQ || undefined,
    tenantID: tenantID || undefined,
    status,
    sort,
    order,
  })

  const groups = useMemo(() => query.data?.groups ?? [], [query.data])
  const meta = query.data?.meta ?? {}
  const totalGroups = meta.total ?? groups.length
  const totalPages = meta.total_pages ?? 1

  // Out-of-range recovery: if the URL carries a ?page beyond the last page
  // the server reports (a deep link, or a filter narrowed the result set
  // after the page was set), snap back to the last real page so the user
  // lands on data instead of a stranded empty state.
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

  function setTenantFilter(value: string) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value === "all") next.delete("tenantID")
        else next.set("tenantID", value)
        next.delete("page")
        return next
      },
      { replace: true }
    )
  }

  function setStatusFilter(value: string) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value === "all") next.delete("status")
        else next.set("status", value)
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

  return (
    <>
      <RouteTitle title={t("groups.title")} />
      <div className="flex flex-col gap-6" data-testid="admin-groups-page">
        <div className="flex items-start gap-3">
          <div className="flex size-9 items-center justify-center rounded-lg bg-primary/10 shrink-0">
            <Layers className="size-5 text-primary" />
          </div>
          <div>
            <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">
              {t("groups.title")}
            </h1>
            <p className="mt-1 text-muted-foreground">{t("groups.subtitle")}</p>
          </div>
        </div>

        <div className="flex flex-col gap-3 sm:flex-row">
          <div className="relative flex-1">
            <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
            <Input
              value={search}
              onChange={(event) => setSearch(event.target.value)}
              placeholder={t("groups.search.placeholder")}
              aria-label={t("groups.search.label")}
              className="pl-8"
              data-testid="admin-groups-search"
            />
            {search ? (
              <Button
                type="button"
                variant="ghost"
                size="icon"
                onClick={() => setSearch("")}
                className="absolute right-1.5 top-1/2 size-7 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                aria-label={t("groups.search.clear")}
              >
                <X className="size-3.5" />
              </Button>
            ) : null}
          </div>
          <Select value={tenantID || "all"} onValueChange={setTenantFilter}>
            <SelectTrigger
              className="sm:w-52"
              aria-label={t("groups.filter.tenantLabel")}
              data-testid="admin-groups-tenant-filter"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("groups.filter.allTenants")}</SelectItem>
              {tenantOptions.map((tenant) =>
                tenant.id ? (
                  <SelectItem key={tenant.id} value={tenant.id}>
                    {tenant.name ?? tenant.slug ?? tenant.id}
                  </SelectItem>
                ) : null
              )}
            </SelectContent>
          </Select>
          <Select value={status ?? "all"} onValueChange={setStatusFilter}>
            <SelectTrigger
              className="sm:w-44"
              aria-label={t("groups.filter.statusLabel")}
              data-testid="admin-groups-status-filter"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("groups.filter.allStatuses")}</SelectItem>
              <SelectItem value="active">{t("groups.filter.active")}</SelectItem>
              <SelectItem value="pending_deletion">{t("groups.filter.pendingDeletion")}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {query.isError ? (
          <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-6 text-sm text-destructive">
            {t("groups.loadError")}
          </div>
        ) : (
          <>
            <div className="rounded-xl border border-border bg-card overflow-hidden">
              <Table data-testid="admin-groups-list">
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <SortableHead
                      field="name"
                      label={t("groups.table.name")}
                      sort={sort}
                      order={order}
                      onSort={toggleSort}
                      className="pl-4"
                    />
                    <TableHead>{t("groups.table.tenant")}</TableHead>
                    <SortableHead
                      field="status"
                      label={t("groups.table.status")}
                      sort={sort}
                      order={order}
                      onSort={toggleSort}
                    />
                    <SortableHead
                      field="slug"
                      label={t("groups.table.slug")}
                      sort={sort}
                      order={order}
                      onSort={toggleSort}
                    />
                    <TableHead className="text-right">{t("groups.table.currency")}</TableHead>
                    <TableHead className="text-right">{t("groups.table.members")}</TableHead>
                    <SortableHead
                      field="created_at"
                      label={t("groups.table.created")}
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
                      <TableCell
                        colSpan={7}
                        className="h-32 text-center text-sm text-muted-foreground"
                      >
                        {t("groups.loading")}
                      </TableCell>
                    </TableRow>
                  ) : groups.length === 0 ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={7} className="h-32 text-center">
                        <div className="flex flex-col items-center justify-center gap-2">
                          <Layers className="size-8 text-muted-foreground/30" />
                          <p className="text-sm text-muted-foreground">{t("groups.empty")}</p>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : (
                    groups.map((group) => (
                      <GroupRow
                        key={group.id}
                        group={group}
                        onSelect={() =>
                          group.id && navigate(`/admin/groups/${encodeURIComponent(group.id)}`)
                        }
                      />
                    ))
                  )}
                </TableBody>
              </Table>
            </div>

            {!query.isLoading && groups.length > 0 ? (
              <AdminPagination
                page={page}
                totalPages={totalPages}
                total={totalGroups}
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

// A compact, non-interactive owning-tenant indicator. Mirrors the design
// mock's TenantChip; the chip reads from the embedded `tenant` object the
// BE returns on every admin group row.
function TenantChip({ name }: { name: string | undefined }) {
  const { t } = useTranslation("admin")
  return (
    <span className="inline-flex max-w-40 items-center gap-1.5 rounded-full border border-border bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground select-none">
      <Building2 className="size-3 shrink-0" />
      <span className="truncate">{name || t("groups.unknownTenant")}</span>
    </span>
  )
}

// A single group row. The whole row is a navigation affordance — clicking
// it (or pressing Enter / Space while it has focus) drills into
// /admin/groups/{id}.
function GroupRow({ group, onSelect }: { group: AdminGroup; onSelect: () => void }) {
  return (
    <TableRow
      className="cursor-pointer"
      onClick={onSelect}
      tabIndex={0}
      role="button"
      onKeyDown={(event) => {
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault()
          onSelect()
        }
      }}
      data-testid="admin-group-row"
    >
      <TableCell className="pl-4 py-3.5">
        <span className="text-sm font-medium">{group.name ?? "—"}</span>
      </TableCell>
      <TableCell className="py-3.5">
        <TenantChip name={group.tenant?.name} />
      </TableCell>
      <TableCell className="py-3.5">
        <GroupStatusBadge status={group.status} />
      </TableCell>
      <TableCell className="py-3.5 font-mono text-xs text-muted-foreground">
        {group.slug ?? "—"}
      </TableCell>
      <TableCell className="py-3.5 text-right text-sm text-muted-foreground">
        {group.currency || "—"}
      </TableCell>
      <TableCell className="py-3.5 text-right text-sm tabular-nums">
        {group.member_count ?? 0}
      </TableCell>
      <TableCell className="pr-4 py-3.5 text-sm text-muted-foreground">
        {group.created_at ? formatDate(group.created_at) : "—"}
      </TableCell>
    </TableRow>
  )
}
