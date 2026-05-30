import { ChevronRight, FileBarChart, Shield } from "lucide-react"
import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { Card } from "@/components/ui/card"
import { Page, PageHeader } from "@/components/ui/page"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { cn } from "@/lib/utils"

// ReportsLandingPage (#1370) — the Reports section root. Lists the report
// types the group can generate. Today there is a single card (Insurance
// report); the page is structured as a card grid so future report types
// drop in as additional entries without reworking the layout.
export function ReportsLandingPage() {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug
  const insuranceHref = slug ? `/g/${encodeURIComponent(slug)}/reports/insurance` : "#"

  return (
    <Page width="wide" data-testid="page-reports">
      <RouteTitle title={t("reports:landing.title")} />
      <PageHeader title={t("reports:landing.title")} subtitle={t("reports:landing.description")} />

      <div className="grid gap-4 sm:grid-cols-2">
        <ReportCard
          to={insuranceHref}
          icon={Shield}
          title={t("reports:landing.insuranceCardTitle")}
          description={t("reports:landing.insuranceCardDescription")}
          testId="reports-card-insurance"
        />
      </div>
    </Page>
  )
}

interface ReportCardProps {
  to: string
  icon: typeof FileBarChart
  title: string
  description: string
  testId?: string
}

function ReportCard({ to, icon: Icon, title, description, testId }: ReportCardProps) {
  const interactive = to !== "#"
  return (
    <Card
      className={cn(
        "group relative flex items-start gap-3 p-5 transition-all",
        interactive && "hover:-translate-y-0.5 hover:border-primary/20 hover:shadow-sm"
      )}
      data-testid={testId}
    >
      {interactive ? (
        <Link
          to={to}
          aria-label={title}
          className="absolute inset-0 rounded-xl focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring/50"
        />
      ) : null}
      <div className="pointer-events-none flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10">
        <Icon className="size-5 text-primary" aria-hidden="true" />
      </div>
      <div className="pointer-events-none min-w-0 flex-1">
        <p className="text-sm font-semibold">{title}</p>
        <p className="mt-0.5 text-sm text-muted-foreground">{description}</p>
      </div>
      <ChevronRight
        className="pointer-events-none size-4 shrink-0 text-muted-foreground transition-colors group-hover:text-foreground"
        aria-hidden="true"
      />
    </Card>
  )
}
