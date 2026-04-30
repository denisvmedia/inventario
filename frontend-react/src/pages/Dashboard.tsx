import { useTranslation } from "react-i18next"
import { Package, ShieldAlert, ShieldCheck, ShieldOff, TrendingUp } from "lucide-react"

import { RouteTitle } from "@/components/routing/RouteTitle"
import { StatCard } from "@/components/dashboard/StatCard"
import { RecentlyAdded } from "@/components/dashboard/RecentlyAdded"
import { ComingSoonBanner } from "@/components/coming-soon/ComingSoonBanner"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useDashboardData } from "@/features/dashboard/hooks"
import { formatCurrency } from "@/lib/intl"

// DashboardPage is the user's group landing at /g/:slug. Layout:
//
//   1. Heading + tagline.
//   2. Four stat cards (totals + warranties + value).
//   3. Two-up grid: warranty placeholder (left) + recently added (right).
//
// Warranty status is currently a tag-based concept (#1367); the three
// warranty cards render placeholder zeros + a "Coming soon" affordance
// rather than guessing at counts. The placeholder card on the lower
// half (`ComingSoonBanner` with `surface="warranties"`) tracks #1367
// directly so reviewers can find the issue from the UI.
//
// Stat cards link to /g/:slug/commodities — the items list (#1410)
// will eventually accept query-string filters (`?status=expiring`)
// that the warranty cards point at; today they all link to the same
// list because there's nothing to filter on yet.
export function DashboardPage() {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const data = useDashboardData()
  const slug = currentGroup?.slug
  const currency = currentGroup?.main_currency ?? "USD"
  const itemsHref = slug ? `/g/${slug}/commodities` : undefined
  const warrantiesHref = slug ? `/g/${slug}/warranties` : undefined
  const formattedValue = data.isLoading ? "—" : formatCurrency(data.totalValue, currency)
  const warrantyComingSoon = t("dashboard:stats.warrantyComingSoon")
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

        <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
          <StatCard
            label={t("dashboard:stats.totalItems")}
            value={data.isLoading ? "—" : data.totalItems}
            sub={t("dashboard:stats.totalItemsSub")}
            icon={Package}
            to={itemsHref}
            isLoading={data.isLoading}
            testId="stat-total-items"
          />
          <StatCard
            label={t("dashboard:stats.activeWarranties")}
            value={"—"}
            sub={warrantyComingSoon}
            icon={ShieldCheck}
            tone="text-muted-foreground"
            to={warrantiesHref}
            testId="stat-active-warranties"
          />
          <StatCard
            label={t("dashboard:stats.expiredWarranties")}
            value={"—"}
            sub={warrantyComingSoon}
            icon={ShieldOff}
            tone="text-muted-foreground"
            to={warrantiesHref}
            testId="stat-expired-warranties"
          />
          <StatCard
            label={t("dashboard:stats.totalValue")}
            value={formattedValue}
            sub={t("dashboard:stats.totalValueSub")}
            icon={TrendingUp}
            to={itemsHref}
            isLoading={data.isLoading}
            testId="stat-total-value"
          />
        </div>

        <div className="grid gap-6 lg:grid-cols-2">
          <Card data-testid="dashboard-warranty-placeholder">
            <CardHeader>
              <CardTitle className="flex items-center gap-2 text-base">
                <ShieldAlert
                  aria-hidden="true"
                  className="size-4 text-muted-foreground"
                />
                {t("dashboard:warrantyPanel.title")}
              </CardTitle>
              <CardDescription>{t("dashboard:warrantyPanel.description")}</CardDescription>
            </CardHeader>
            <CardContent>
              <ComingSoonBanner surface="warranties" />
            </CardContent>
          </Card>

          <RecentlyAdded items={data.recent} isLoading={data.isLoading} />
        </div>
      </div>
    </>
  )
}
