import { useMemo, useState } from "react"
import { Activity, Building2, Layers, Search, Users, X } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAdminTenants } from "@/features/admin/hooks"
import type { AdminTenant } from "@/features/admin/api"
import { useDebouncedValue } from "@/hooks/useDebouncedValue"
import { cn } from "@/lib/utils"
import { formatDate } from "@/lib/intl"

// As-you-type search debounce — the typed value drives a server-side
// `?q` query, so we throttle keystrokes to one request per pause. 250ms
// matches the other search surfaces in this codebase (see TagsListPage).
const SEARCH_DEBOUNCE_MS = 250

// Per-status badge tone. Mirrors the design-mock AccountStateBadge /
// TenantStatusBadge palette: status tokens, never raw colors.
const STATUS_TONE: Record<string, string> = {
  active: "text-status-active bg-status-active/10",
  suspended: "text-status-expiring bg-status-expiring/10",
  inactive: "text-status-none bg-status-none/10",
}

function TenantStatusBadge({ status }: { status: string | undefined }) {
  const tone = (status && STATUS_TONE[status]) || STATUS_TONE.inactive
  return (
    <Badge
      variant="outline"
      className={cn("h-5 text-xs border-current/20 font-medium capitalize", tone)}
    >
      {status ?? "—"}
    </Badge>
  )
}

interface StatTile {
  labelKey: string
  value: number
  icon: typeof Building2
}

// AdminTenantsPage is the admin landing route (/admin/tenants). It lists
// every tenant on the platform with a stats row, free-text search, and a
// divide-y card list of rows.
//
// The design-mock TenantsView uses shadcn `<Table>` + `<Pagination>`
// primitives, neither of which exists in this frontend yet. To keep this
// foundation issue from pulling in two new primitives, the list is
// rendered with the established frontend `divide-y` card-list convention
// (see LoginHistoryPage). Logged as a deviation in
// devdocs/frontend/design-deviations.md. Server-side pagination plumbing
// stays in the api/hooks layer for later sub-issues to surface.
//
// Search is server-side: the debounced input feeds `?q` (which the BE
// matches against name/slug/domain), so filtering stays correct once
// there is more than one page of tenants.
export function AdminTenantsPage() {
  const { t } = useTranslation("admin")
  const [search, setSearch] = useState("")
  const debouncedSearch = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS)
  const query = useAdminTenants({ q: debouncedSearch || undefined })

  const tenants = useMemo(() => query.data?.tenants ?? [], [query.data])

  const stats = useMemo<StatTile[]>(
    () => [
      { labelKey: "tenants.stats.tenants", value: tenants.length, icon: Building2 },
      {
        labelKey: "tenants.stats.active",
        value: tenants.filter((tenant) => tenant.status === "active").length,
        icon: Activity,
      },
      {
        labelKey: "tenants.stats.totalUsers",
        value: tenants.reduce((sum, tenant) => sum + (tenant.user_count ?? 0), 0),
        icon: Users,
      },
      {
        labelKey: "tenants.stats.totalGroups",
        value: tenants.reduce((sum, tenant) => sum + (tenant.group_count ?? 0), 0),
        icon: Layers,
      },
    ],
    [tenants]
  )

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

        <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
          {stats.map((stat) => (
            <div
              key={stat.labelKey}
              className="rounded-xl border border-border bg-card px-4 py-3 flex items-center gap-3"
            >
              <div className="flex size-8 items-center justify-center rounded-lg bg-muted shrink-0">
                <stat.icon className="size-4 text-muted-foreground" />
              </div>
              <div>
                <p className="text-xs text-muted-foreground">{t(stat.labelKey)}</p>
                <p className="text-lg font-semibold leading-tight tabular-nums">{stat.value}</p>
              </div>
            </div>
          ))}
        </div>

        <div className="relative">
          <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t("tenants.search.placeholder")}
            className="pl-8"
            data-testid="admin-tenants-search"
          />
          {search ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              onClick={() => setSearch("")}
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
        ) : query.isLoading ? (
          <div className="rounded-xl border border-border p-6 text-sm text-muted-foreground">
            {t("tenants.loading")}
          </div>
        ) : tenants.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-2 rounded-xl border border-border bg-card py-16">
            <Building2 className="size-8 text-muted-foreground/30" />
            <p className="text-sm text-muted-foreground">{t("tenants.empty")}</p>
          </div>
        ) : (
          <ul
            className="rounded-xl border border-border divide-y divide-border bg-card"
            data-testid="admin-tenants-list"
          >
            {tenants.map((tenant) => (
              <TenantRow key={tenant.id} tenant={tenant} />
            ))}
          </ul>
        )}
      </div>
    </>
  )
}

function TenantRow({ tenant }: { tenant: AdminTenant }) {
  const { t } = useTranslation("admin")
  return (
    <li className="flex items-center gap-4 p-4" data-testid="admin-tenant-row">
      <div className="flex size-9 items-center justify-center rounded-lg bg-muted shrink-0">
        <Building2 className="size-4 text-muted-foreground" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-sm font-medium truncate">{tenant.name ?? "—"}</span>
          <TenantStatusBadge status={tenant.status} />
        </div>
        <p className="font-mono text-xs text-muted-foreground truncate">
          {tenant.slug ?? ""}
          {tenant.domain ? ` · ${tenant.domain}` : ""}
        </p>
      </div>
      <div className="hidden shrink-0 items-center gap-6 sm:flex">
        <div className="text-right">
          <p className="text-xs text-muted-foreground">{t("tenants.table.users")}</p>
          <p className="text-sm font-medium tabular-nums">{tenant.user_count ?? 0}</p>
        </div>
        <div className="text-right">
          <p className="text-xs text-muted-foreground">{t("tenants.table.groups")}</p>
          <p className="text-sm font-medium tabular-nums">{tenant.group_count ?? 0}</p>
        </div>
        <div className="text-right">
          <p className="text-xs text-muted-foreground">{t("tenants.table.created")}</p>
          <p className="text-sm text-muted-foreground">
            {tenant.created_at ? formatDate(tenant.created_at) : "—"}
          </p>
        </div>
      </div>
    </li>
  )
}
