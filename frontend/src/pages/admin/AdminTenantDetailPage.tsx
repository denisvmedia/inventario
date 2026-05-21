import { useEffect, useState } from "react"
import { ArrowLeft, Building2, Globe, Hash, Layers, Search, Users, X } from "lucide-react"
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom"
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAdminGroups, useAdminTenant, useAdminTenantUsers } from "@/features/admin/hooks"
import { useDebouncedValue } from "@/hooks/useDebouncedValue"
import { HttpError } from "@/lib/http"
import { formatDate, formatDateTime } from "@/lib/intl"

import { AccountStateBadge, GroupStatusBadge, TenantStatusBadge } from "./admin-shared"
import { AdminPagination } from "./AdminPagination"

const SEARCH_DEBOUNCE_MS = 250
const PAGE_SIZE = 20

// The two detail tabs. Persisted as ?tab= so a copied link reproduces
// the active tab; "users" is the default and is dropped from the URL.
type DetailTab = "users" | "groups"
const DEFAULT_TAB: DetailTab = "users"

function parseTab(raw: string | null): DetailTab {
  return raw === "groups" ? "groups" : "users"
}

function parsePage(raw: string | null): number {
  const n = Number(raw)
  return Number.isInteger(n) && n >= 1 ? n : 1
}

// AdminTenantDetailPage is /admin/tenants/:tenantId — a read-only tenant
// header (name, slug, domain, status, plan, user/group counts) plus a
// Users tab and a Groups tab. Tab selection, per-tab search / filter and
// pagination all round-trip through the URL query string so a copied
// link reproduces the exact view.
//
// Naming: the issue text proposed `TenantDetailPage.tsx`, but the #1752
// foundation established the `Admin*Page` convention for this surface
// (AdminTenantsPage / AdminUsersPage / AdminGroupsPage); this page
// follows the codebase convention. See devdocs/frontend/design-deviations.md.
export function AdminTenantDetailPage() {
  const { t } = useTranslation("admin")
  const navigate = useNavigate()
  const { tenantId = "" } = useParams()
  const [searchParams, setSearchParams] = useSearchParams()
  const tab = parseTab(searchParams.get("tab"))

  const tenant = useAdminTenant(tenantId)

  // GET /admin/tenants/{id} returns HTTP 404 for a missing tenant
  // (apiserver maps registry.ErrNotFound → NewNotFoundError), so a
  // genuine not-found surfaces as a query error. Treat that 404 as "not
  // found" (its own friendly empty state) and keep the generic
  // load-error card for every other failure — including a malformed
  // 200-with-empty-body, which `getAdminTenant` now rejects as a thrown
  // error rather than letting an id-less object through.
  const isNotFound =
    tenant.isError && tenant.error instanceof HttpError && tenant.error.status === 404

  function setTab(next: string) {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev)
        if (next === DEFAULT_TAB) params.delete("tab")
        else params.set("tab", next)
        // Tab-local query state (search / filter / page) is meaningless
        // across tabs — drop it on switch so the new tab starts clean.
        params.delete("q")
        params.delete("page")
        params.delete("active")
        params.delete("status")
        return params
      },
      { replace: true }
    )
  }

  const tenantName = tenant.data?.name ?? t("tenantDetail.fallbackName")

  return (
    <>
      <RouteTitle title={tenantName} />
      <div className="flex flex-col gap-6" data-testid="admin-tenant-detail-page">
        <Button
          variant="ghost"
          size="sm"
          asChild
          className="gap-1.5 -ml-2 self-start text-muted-foreground hover:text-foreground"
        >
          <Link to="/admin/tenants">
            <ArrowLeft className="size-4" />
            {t("tenantDetail.back")}
          </Link>
        </Button>

        {isNotFound ? (
          <div
            className="flex flex-col items-center justify-center gap-3 py-24"
            data-testid="admin-tenant-not-found"
          >
            <Building2 className="size-8 text-muted-foreground/30" />
            <p className="text-sm text-muted-foreground">{t("tenantDetail.notFound")}</p>
          </div>
        ) : tenant.isError ? (
          <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-6 text-sm text-destructive">
            {t("tenantDetail.loadError")}
          </div>
        ) : tenant.isLoading || !tenant.data ? (
          <div className="rounded-xl border border-border bg-card p-6 text-sm text-muted-foreground">
            {t("tenantDetail.loading")}
          </div>
        ) : (
          <>
            <TenantHeaderCard tenant={tenant.data} />

            <Tabs value={tab} onValueChange={setTab}>
              <TabsList>
                <TabsTrigger value="users" data-testid="admin-tenant-tab-users">
                  <Users className="size-3.5" />
                  {t("tenantDetail.tabs.users")}
                </TabsTrigger>
                <TabsTrigger value="groups" data-testid="admin-tenant-tab-groups">
                  <Layers className="size-3.5" />
                  {t("tenantDetail.tabs.groups")}
                </TabsTrigger>
              </TabsList>

              <TabsContent value="users">
                <TenantUsersTab
                  tenantId={tenantId}
                  searchParams={searchParams}
                  setSearchParams={setSearchParams}
                  onSelectUser={(userId) => navigate(`/admin/users/${encodeURIComponent(userId)}`)}
                />
              </TabsContent>
              <TabsContent value="groups">
                <TenantGroupsTab
                  tenantId={tenantId}
                  searchParams={searchParams}
                  setSearchParams={setSearchParams}
                  onSelectGroup={(groupId) =>
                    navigate(`/admin/groups/${encodeURIComponent(groupId)}`)
                  }
                />
              </TabsContent>
            </Tabs>
          </>
        )}
      </div>
    </>
  )
}

// Read-only identity + metrics card for the tenant.
function TenantHeaderCard({
  tenant,
}: {
  tenant: NonNullable<ReturnType<typeof useAdminTenant>["data"]>
}) {
  const { t } = useTranslation("admin")
  const stats = [
    { label: t("tenantDetail.header.plan"), value: tenant.plan_id || "—" },
    { label: t("tenantDetail.header.users"), value: String(tenant.user_count ?? 0) },
    { label: t("tenantDetail.header.groups"), value: String(tenant.group_count ?? 0) },
    {
      label: t("tenantDetail.header.created"),
      value: tenant.created_at ? formatDate(tenant.created_at) : "—",
    },
  ]
  return (
    <div className="rounded-xl border border-border bg-card p-6" data-testid="admin-tenant-header">
      <div className="flex items-start gap-4">
        <div className="flex size-12 items-center justify-center rounded-xl bg-primary/10 shrink-0">
          <Building2 className="size-6 text-primary" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-semibold tracking-tight">{tenant.name ?? "—"}</h1>
            <TenantStatusBadge status={tenant.status} />
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-sm text-muted-foreground">
            <span className="inline-flex items-center gap-1.5">
              <Hash className="size-3.5" />
              <span className="font-mono text-xs">{tenant.slug ?? "—"}</span>
            </span>
            {tenant.domain ? (
              <span className="inline-flex items-center gap-1.5">
                <Globe className="size-3.5" />
                {tenant.domain}
              </span>
            ) : null}
          </div>
        </div>
      </div>

      <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
        {stats.map((s) => (
          <div key={s.label} className="rounded-lg border border-border bg-muted/40 px-3 py-2.5">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              {s.label}
            </p>
            <p className="mt-0.5 text-sm font-semibold">{s.value}</p>
          </div>
        ))}
      </div>
    </div>
  )
}

interface TabProps {
  tenantId: string
  searchParams: URLSearchParams
  setSearchParams: ReturnType<typeof useSearchParams>[1]
}

// ─── Users tab ────────────────────────────────────────────────
// Paginated user listing for the tenant, with free-text search and a
// tri-state isActive filter. All state lives in the URL (?q, ?active,
// ?page) so it round-trips on copy.
function TenantUsersTab({
  tenantId,
  searchParams,
  setSearchParams,
  onSelectUser,
}: TabProps & { onSelectUser: (userId: string) => void }) {
  const { t } = useTranslation("admin")

  const urlQ = searchParams.get("q") ?? ""
  const activeRaw = searchParams.get("active")
  const isActive = activeRaw === "true" ? true : activeRaw === "false" ? false : undefined
  const page = parsePage(searchParams.get("page"))

  const [search, setSearch] = useState(urlQ)
  // Re-seed the input when the URL `q` changes via back/forward (or a
  // tab switch that clears ?q) — a controlled-input sync from URL state.
  // Bounded by URL changes; does not fight the debounced search → URL
  // effect below. Mirrors TagsListPage.
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setSearch(urlQ), [urlQ])
  const debouncedSearch = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS)

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

  const query = useAdminTenantUsers(tenantId, {
    page,
    perPage: PAGE_SIZE,
    q: urlQ || undefined,
    isActive,
  })

  const users = query.data?.users ?? []
  const meta = query.data?.meta ?? {}
  const total = meta.total ?? users.length
  const totalPages = meta.total_pages ?? 1

  // Out-of-range recovery — see AdminTenantsPage for the rationale. A
  // search / filter that shrinks the result set can leave ?page beyond
  // the last page; snap back so the user lands on data, not the empty
  // state with no pager to escape.
  useEffect(() => {
    if (query.data && page > totalPages && totalPages >= 1) {
      goToPage(totalPages)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data, page, totalPages])

  function setActiveFilter(value: string) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value === "all") next.delete("active")
        else next.set("active", value)
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
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <div className="relative flex-1">
          <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t("tenantDetail.users.searchPlaceholder")}
            aria-label={t("tenantDetail.users.searchLabel")}
            className="pl-8"
            data-testid="admin-tenant-users-search"
          />
          {search ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={() => setSearch("")}
              className="absolute right-1.5 top-1/2 size-7 -translate-y-1/2 text-muted-foreground hover:text-foreground"
              aria-label={t("tenantDetail.users.searchClear")}
            >
              <X className="size-3.5" />
            </Button>
          ) : null}
        </div>
        <Select
          value={activeRaw === "true" ? "true" : activeRaw === "false" ? "false" : "all"}
          onValueChange={setActiveFilter}
        >
          <SelectTrigger
            className="sm:w-44"
            aria-label={t("tenantDetail.users.filterLabel")}
            data-testid="admin-tenant-users-filter"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("tenantDetail.users.filter.all")}</SelectItem>
            <SelectItem value="true">{t("tenantDetail.users.filter.active")}</SelectItem>
            <SelectItem value="false">{t("tenantDetail.users.filter.blocked")}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {query.isError ? (
        <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-6 text-sm text-destructive">
          {t("tenantDetail.users.loadError")}
        </div>
      ) : (
        <>
          <div className="rounded-xl border border-border bg-card overflow-hidden">
            <Table data-testid="admin-tenant-users-table">
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="pl-4">{t("tenantDetail.users.col.name")}</TableHead>
                  <TableHead>{t("tenantDetail.users.col.email")}</TableHead>
                  <TableHead className="text-right">{t("tenantDetail.users.col.groups")}</TableHead>
                  <TableHead>{t("tenantDetail.users.col.state")}</TableHead>
                  <TableHead className="pr-4">{t("tenantDetail.users.col.lastLogin")}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {query.isLoading ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={5}
                      className="h-24 text-center text-sm text-muted-foreground"
                    >
                      {t("tenantDetail.users.loading")}
                    </TableCell>
                  </TableRow>
                ) : users.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={5}
                      className="h-24 text-center text-sm text-muted-foreground"
                    >
                      {t("tenantDetail.users.empty")}
                    </TableCell>
                  </TableRow>
                ) : (
                  users.map((user) => (
                    <TableRow
                      key={user.id}
                      className="cursor-pointer"
                      onClick={() => user.id && onSelectUser(user.id)}
                      tabIndex={0}
                      role="button"
                      onKeyDown={(event) => {
                        if (user.id && (event.key === "Enter" || event.key === " ")) {
                          event.preventDefault()
                          onSelectUser(user.id)
                        }
                      }}
                      data-testid="admin-tenant-user-row"
                    >
                      <TableCell className="pl-4 py-3.5 text-sm font-medium">
                        {user.name || "—"}
                      </TableCell>
                      <TableCell className="py-3.5 text-sm text-muted-foreground">
                        {user.email || "—"}
                      </TableCell>
                      <TableCell className="py-3.5 text-right text-sm tabular-nums">
                        {user.group_membership_count ?? 0}
                      </TableCell>
                      <TableCell className="py-3.5">
                        <AccountStateBadge active={user.is_active} />
                      </TableCell>
                      <TableCell className="pr-4 py-3.5 text-sm text-muted-foreground">
                        {user.last_login_at
                          ? formatDateTime(user.last_login_at)
                          : t("tenantDetail.users.neverLoggedIn")}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>

          {!query.isLoading && users.length > 0 ? (
            <AdminPagination
              page={page}
              totalPages={totalPages}
              total={total}
              pageSize={PAGE_SIZE}
              onPageChange={goToPage}
            />
          ) : null}
        </>
      )}
    </div>
  )
}

// ─── Groups tab ───────────────────────────────────────────────
// Paginated group listing for the tenant, filtered by status. State
// lives in the URL (?status, ?page).
function TenantGroupsTab({
  tenantId,
  searchParams,
  setSearchParams,
  onSelectGroup,
}: TabProps & { onSelectGroup: (groupId: string) => void }) {
  const { t } = useTranslation("admin")

  const statusRaw = searchParams.get("status")
  const status = statusRaw === "active" || statusRaw === "pending_deletion" ? statusRaw : undefined
  const page = parsePage(searchParams.get("page"))

  const query = useAdminGroups({
    tenantID: tenantId,
    page,
    perPage: PAGE_SIZE,
    status,
  })

  const groups = query.data?.groups ?? []
  const meta = query.data?.meta ?? {}
  const total = meta.total ?? groups.length
  const totalPages = meta.total_pages ?? 1

  // Out-of-range recovery — see AdminTenantsPage for the rationale.
  useEffect(() => {
    if (query.data && page > totalPages && totalPages >= 1) {
      goToPage(totalPages)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data, page, totalPages])

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
    <div className="flex flex-col gap-4">
      <div className="flex">
        <Select value={status ?? "all"} onValueChange={setStatusFilter}>
          <SelectTrigger
            className="sm:w-52"
            aria-label={t("tenantDetail.groups.filterLabel")}
            data-testid="admin-tenant-groups-filter"
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("tenantDetail.groups.filter.all")}</SelectItem>
            <SelectItem value="active">{t("tenantDetail.groups.filter.active")}</SelectItem>
            <SelectItem value="pending_deletion">
              {t("tenantDetail.groups.filter.pendingDeletion")}
            </SelectItem>
          </SelectContent>
        </Select>
      </div>

      {query.isError ? (
        <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-6 text-sm text-destructive">
          {t("tenantDetail.groups.loadError")}
        </div>
      ) : (
        <>
          <div className="rounded-xl border border-border bg-card overflow-hidden">
            <Table data-testid="admin-tenant-groups-table">
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="pl-4">{t("tenantDetail.groups.col.group")}</TableHead>
                  <TableHead>{t("tenantDetail.groups.col.status")}</TableHead>
                  <TableHead>{t("tenantDetail.groups.col.currency")}</TableHead>
                  <TableHead className="text-right pr-4">
                    {t("tenantDetail.groups.col.members")}
                  </TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {query.isLoading ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={4}
                      className="h-24 text-center text-sm text-muted-foreground"
                    >
                      {t("tenantDetail.groups.loading")}
                    </TableCell>
                  </TableRow>
                ) : groups.length === 0 ? (
                  <TableRow className="hover:bg-transparent">
                    <TableCell
                      colSpan={4}
                      className="h-24 text-center text-sm text-muted-foreground"
                    >
                      {t("tenantDetail.groups.empty")}
                    </TableCell>
                  </TableRow>
                ) : (
                  groups.map((group) => (
                    <TableRow
                      key={group.id}
                      className="cursor-pointer"
                      onClick={() => group.id && onSelectGroup(group.id)}
                      tabIndex={0}
                      role="button"
                      onKeyDown={(event) => {
                        if (group.id && (event.key === "Enter" || event.key === " ")) {
                          event.preventDefault()
                          onSelectGroup(group.id)
                        }
                      }}
                      data-testid="admin-tenant-group-row"
                    >
                      <TableCell className="pl-4 py-3.5">
                        <div className="flex flex-col">
                          <span className="text-sm font-medium">{group.name ?? "—"}</span>
                          <span className="font-mono text-xs text-muted-foreground">
                            {group.slug ?? ""}
                          </span>
                        </div>
                      </TableCell>
                      <TableCell className="py-3.5">
                        <GroupStatusBadge status={group.status} />
                      </TableCell>
                      <TableCell className="py-3.5 text-sm text-muted-foreground">
                        {group.currency || "—"}
                      </TableCell>
                      <TableCell className="pr-4 py-3.5 text-right text-sm tabular-nums">
                        {group.member_count ?? 0}
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </div>

          {!query.isLoading && groups.length > 0 ? (
            <AdminPagination
              page={page}
              totalPages={totalPages}
              total={total}
              pageSize={PAGE_SIZE}
              onPageChange={goToPage}
            />
          ) : null}
        </>
      )}
    </div>
  )
}
