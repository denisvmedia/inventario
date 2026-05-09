import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { FolderOpen, MapPin, Package, Pin, Plus, Sparkles, TrendingUp } from "lucide-react"

import { RouteTitle } from "@/components/routing/RouteTitle"
import { StatCard } from "@/components/dashboard/StatCard"
import { RecentlyAdded } from "@/components/dashboard/RecentlyAdded"
import { ExpiringWarranties } from "@/components/dashboard/ExpiringWarranties"
import { WarrantyHealth } from "@/components/dashboard/WarrantyHealth"
import { ComingSoonBanner } from "@/components/coming-soon/ComingSoonBanner"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useAreas } from "@/features/areas/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useDashboardData } from "@/features/dashboard/hooks"
import { useFiles } from "@/features/files/hooks"
import { useLocations } from "@/features/locations/hooks"
import { useGroupMigrationLock } from "@/features/currency-migration/lock"
import { formatCurrency } from "@/lib/intl"
import { cn } from "@/lib/utils"

// DashboardPage is the user's group landing at /g/:slug. Layout:
//
//   1. Heading + tagline.
//   2. Two stat-card rows (totals + value, then locations / areas / files).
//   3. Two-up grid of value-by-location/area placeholders (still gated
//      on a future per-place value endpoint — those panels keep their
//      `ComingSoonBanner`).
//   4. Two-up grid: ExpiringWarranties (left) + RecentlyAdded (right).
//   5. Full-width WarrantyHealth card.
//
// Warranty data lights up via #1367 / #1529: counts + the expiring
// shortlist come from `useDashboardData`'s warrantyBuckets() pass over
// the same commodities query the page already runs.
//
// Slugs are passed through `encodeURIComponent` (matching the rest of
// the navigation surface) so a slug that ever contains a `/` or `?`
// can't break the URL we hand to react-router.
export function DashboardPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const data = useDashboardData()
  const locationsQuery = useLocations()
  const areasQuery = useAreas()
  const filesQuery = useFiles()
  const migrationLock = useGroupMigrationLock()
  const slug = currentGroup?.slug
  const currency = currentGroup?.group_currency ?? "USD"
  const itemsHref = slug ? `/g/${encodeURIComponent(slug)}/commodities` : undefined
  const addItemHref = slug ? `/g/${encodeURIComponent(slug)}/commodities/new` : undefined
  const locationsHref = slug ? `/g/${encodeURIComponent(slug)}/locations` : undefined
  const filesHref = slug ? `/g/${encodeURIComponent(slug)}/files` : undefined
  const formattedValue = data.isLoading ? "—" : formatCurrency(data.totalValue, currency)
  const avgValue =
    data.isLoading || data.totalItems === 0
      ? "—"
      : formatCurrency(data.totalValue / data.totalItems, currency)
  const locationsCount = locationsQuery.data?.length ?? 0
  const areasCount = areasQuery.data?.length ?? 0
  const filesCount = filesQuery.data?.total ?? filesQuery.data?.files.length ?? 0
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
              navigate(addItemHref)
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
            <div className="grid grid-cols-2 gap-4 lg:grid-cols-3">
              <StatCard
                label={t("dashboard:stats.totalValue")}
                value={formattedValue}
                sub={t("dashboard:stats.totalValueSub")}
                icon={TrendingUp}
                to={itemsHref}
                isLoading={data.isLoading}
                testId="dashboard-total-value"
              />
              <StatCard
                label={t("dashboard:stats.avgValue")}
                value={avgValue}
                sub={t("dashboard:stats.avgValueSub")}
                icon={TrendingUp}
                isLoading={data.isLoading}
                testId="dashboard-avg-value"
              />
              <StatCard
                label={t("dashboard:stats.totalItems")}
                value={data.isLoading ? "—" : data.totalItems}
                sub={t("dashboard:stats.totalItemsSub")}
                icon={Package}
                to={itemsHref}
                isLoading={data.isLoading}
                testId="dashboard-commodities-count"
              />
            </div>
            <div className="grid grid-cols-2 gap-4 lg:grid-cols-3">
              <StatCard
                label={t("dashboard:stats.locations")}
                value={locationsQuery.isLoading ? "—" : locationsCount}
                sub={t("dashboard:stats.locationsSub")}
                icon={MapPin}
                to={locationsHref}
                isLoading={locationsQuery.isLoading}
                testId="dashboard-locations-count"
              />
              <StatCard
                label={t("dashboard:stats.areas")}
                value={areasQuery.isLoading ? "—" : areasCount}
                sub={t("dashboard:stats.areasSub")}
                icon={Pin}
                to={locationsHref}
                isLoading={areasQuery.isLoading}
                testId="dashboard-areas-count"
              />
              <StatCard
                label={t("dashboard:stats.files")}
                value={filesQuery.isLoading ? "—" : filesCount}
                sub={t("dashboard:stats.filesSub")}
                icon={FolderOpen}
                to={filesHref}
                isLoading={filesQuery.isLoading}
                testId="dashboard-files-count"
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
