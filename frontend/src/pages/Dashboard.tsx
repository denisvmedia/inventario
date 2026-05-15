import { useTranslation } from "react-i18next"
import { useLocation, useNavigate } from "react-router-dom"
import {
  MapPin,
  Package,
  Pin,
  Plus,
  ShieldCheck,
  ShieldOff,
  Sparkles,
  TrendingUp,
} from "lucide-react"

import { RouteTitle } from "@/components/routing/RouteTitle"
import { StatCard } from "@/components/dashboard/StatCard"
import { RecentlyAdded } from "@/components/dashboard/RecentlyAdded"
import { ExpiringWarranties } from "@/components/dashboard/ExpiringWarranties"
import { WarrantyHealth } from "@/components/dashboard/WarrantyHealth"
import { ComingSoonBanner } from "@/components/coming-soon/ComingSoonBanner"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useDashboardData } from "@/features/dashboard/hooks"
import { useGroupMigrationLock } from "@/features/currency-migration/lock"
import { formatCurrency } from "@/lib/intl"
import { cn } from "@/lib/utils"

// DashboardPage is the user's group landing at /g/:slug. Layout:
//
//   1. Heading + tagline.
//   2. Mobile-only Add-item CTA card.
//   3. Hero stat-card grid: 4 cards (Total Items / Active Warranties /
//      Expired Warranties / Est. Total Value), 2×2 on mobile and 4×1
//      from `lg:` up — ports `design-mocks/src/views/DashboardView.tsx`
//      L112-L131 1:1 to surface the warranty framing the product leads
//      with. Locations / Areas / Files counts that the previous 6-card
//      grid carried are reachable from the sidebar nav and the matching
//      list pages, so dropping them here trades one duplicated count for
//      a cleaner mobile read (#1544 item 2 decision).
//   4. Two-up grid of value-by-location/area placeholders (still gated
//      on a future per-place value endpoint — those panels keep their
//      `ComingSoonBanner`).
//   5. Two-up grid: ExpiringWarranties (left) + RecentlyAdded (right).
//   6. Full-width WarrantyHealth card.
//
// Slugs are passed through `encodeURIComponent` (matching the rest of
// the navigation surface) so a slug that ever contains a `/` or `?`
// can't break the URL we hand to react-router.
export function DashboardPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const location = useLocation()
  const { currentGroup } = useCurrentGroup()
  const data = useDashboardData()
  const migrationLock = useGroupMigrationLock()
  const slug = currentGroup?.slug
  const currency = currentGroup?.group_currency ?? "USD"
  const itemsHref = slug ? `/g/${encodeURIComponent(slug)}/commodities` : undefined
  const addItemHref = slug ? `/g/${encodeURIComponent(slug)}/commodities/new` : undefined
  // Warranty stat cards drill into the dedicated WarrantiesListPage with
  // its tab pre-selected — `?tab=active|expired` matches the param the
  // page reads in `parseTab(searchParams.get("tab"))`.
  const warrantiesHref = slug ? `/g/${encodeURIComponent(slug)}/warranties` : undefined
  const activeWarrantiesHref = warrantiesHref ? `${warrantiesHref}?tab=active` : undefined
  const expiredWarrantiesHref = warrantiesHref ? `${warrantiesHref}?tab=expired` : undefined
  const formattedValue = data.isLoading ? "—" : formatCurrency(data.totalValue, currency)
  const activeCount = data.warrantyStatusCounts.active
  const expiringCount = data.warrantyStatusCounts.expiring
  const expiredCount = data.warrantyStatusCounts.expired
  return (
    <>
      <RouteTitle title={t("dashboard:documentTitle")} />
      <div
        className="flex flex-col gap-8 p-6 max-w-5xl mx-auto w-full"
        data-testid="page-dashboard"
      >
        <header>
          <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">
            {t("dashboard:heading")}
          </h1>
          <p className="mt-1 text-muted-foreground leading-7">{t("dashboard:tagline")}</p>
        </header>

        {addItemHref ? (
          // Real <button>, not <Link>, so the cursor matches the
          // design-mock DashboardView exactly: Tailwind v4 preflight
          // drops `cursor: pointer` from buttons, so an <a> here would
          // visually diverge. Navigate imperatively in onClick.
          <button
            type="button"
            data-testid="dashboard-mobile-add-item"
            aria-disabled={migrationLock.locked || undefined}
            title={migrationLock.locked ? t("errors:lockedDuringMigration") : undefined}
            onClick={() => {
              if (migrationLock.locked) return
              // Pass `state.background` so the modal-overlay tree
              // (router.tsx) renders the create dialog on top of the
              // current page instead of swapping the backdrop to the
              // items list.
              navigate(addItemHref, { state: { background: location } })
            }}
            className={cn(
              "group flex w-full items-center gap-4 rounded-2xl border border-border bg-card px-5 py-4 text-left transition-all md:hidden",
              migrationLock.locked
                ? "cursor-not-allowed opacity-60"
                : "hover:border-primary/30 hover:bg-muted/40 hover:shadow-sm active:scale-[0.98]"
            )}
          >
            <div className="flex size-12 shrink-0 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-sm transition-transform group-active:scale-95">
              <Plus aria-hidden="true" className="size-5" />
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-base font-semibold leading-tight">
                {t("dashboard:mobileCta.title")}
              </p>
              <p className="mt-0.5 text-sm text-muted-foreground">
                {t("dashboard:mobileCta.subtitle")}
              </p>
            </div>
            <Sparkles aria-hidden="true" className="size-4 shrink-0 text-muted-foreground/50" />
          </button>
        ) : null}

        {data.isError ? (
          // Error state: render the heading + an alert instead of stat
          // cards. Showing skeletal "0 / $0.00 / Nothing here" on a
          // failed load would read as "you have no inventory" — exactly
          // the wrong story. The alert leaves the chrome (sidebar,
          // top-bar, etc.) intact so the user can navigate away.
          <Alert variant="destructive" data-testid="dashboard-error">
            <AlertTitle>{t("dashboard:error.title")}</AlertTitle>
            <AlertDescription>{t("dashboard:error.description")}</AlertDescription>
          </Alert>
        ) : (
          <>
            <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
              <StatCard
                label={t("dashboard:stats.totalItems")}
                value={data.isLoading ? "—" : data.totalItems}
                sub={t("dashboard:stats.totalItemsSub")}
                icon={Package}
                to={itemsHref}
                isLoading={data.isLoading}
                testId="dashboard-commodities-count"
              />
              <StatCard
                label={t("dashboard:stats.activeWarranties")}
                value={data.isLoading ? "—" : activeCount}
                sub={
                  data.isLoading
                    ? undefined
                    : t("dashboard:stats.activeWarrantiesSub", { count: expiringCount })
                }
                icon={ShieldCheck}
                tone="text-status-active"
                to={activeWarrantiesHref}
                isLoading={data.isLoading}
                testId="dashboard-active-warranties"
              />
              <StatCard
                label={t("dashboard:stats.expiredWarranties")}
                value={data.isLoading ? "—" : expiredCount}
                sub={t("dashboard:stats.expiredWarrantiesSub")}
                icon={ShieldOff}
                tone="text-status-expired"
                to={expiredWarrantiesHref}
                isLoading={data.isLoading}
                testId="dashboard-expired-warranties"
              />
              <StatCard
                label={t("dashboard:stats.totalValue")}
                value={formattedValue}
                sub={t("dashboard:stats.totalValueSub")}
                icon={TrendingUp}
                to={itemsHref}
                isLoading={data.isLoading}
                testId="dashboard-total-value"
              />
            </div>

            <div className="grid gap-6 lg:grid-cols-2">
              <Card data-testid="dashboard-value-by-location">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2 text-base">
                    <MapPin aria-hidden="true" className="size-4 text-muted-foreground" />
                    {t("dashboard:valueByLocation.title")}
                  </CardTitle>
                  <CardDescription>{t("dashboard:valueByLocation.description")}</CardDescription>
                </CardHeader>
                <CardContent>
                  <ComingSoonBanner surface="warranties" />
                </CardContent>
              </Card>

              <Card data-testid="dashboard-value-by-area">
                <CardHeader>
                  <CardTitle className="flex items-center gap-2 text-base">
                    <Pin aria-hidden="true" className="size-4 text-muted-foreground" />
                    {t("dashboard:valueByArea.title")}
                  </CardTitle>
                  <CardDescription>{t("dashboard:valueByArea.description")}</CardDescription>
                </CardHeader>
                <CardContent>
                  <ComingSoonBanner surface="warranties" />
                </CardContent>
              </Card>
            </div>

            <div className="grid gap-6 lg:grid-cols-2">
              <ExpiringWarranties items={data.expiringWarranties} isLoading={data.isLoading} />
              <RecentlyAdded items={data.recent} isLoading={data.isLoading} />
            </div>

            <WarrantyHealth counts={data.warrantyStatusCounts} isLoading={data.isLoading} />
          </>
        )}
      </div>
    </>
  )
}
