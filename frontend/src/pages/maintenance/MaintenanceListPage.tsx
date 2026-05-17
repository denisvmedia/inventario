import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { daysUntilDue } from "@/features/maintenance/api"
import { useGroupMaintenance } from "@/features/maintenance/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

// MaintenanceListPage is the dedicated /maintenance surface (#1368) —
// group-wide list of schedules ordered by next_due_at. Each row links
// to the parent commodity so the user can drill into the Maintenance
// tab there to mark done, edit cadence, or delete.
export function MaintenanceListPage() {
  const { t } = useTranslation(["maintenance", "common"])
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""

  const list = useGroupMaintenance({ perPage: 100 })
  const rows = list.data?.schedules ?? []

  return (
    <div className="flex flex-col gap-6 p-6" data-testid="page-maintenance">
      <header className="flex flex-col gap-1">
        <h1 className="text-2xl font-semibold">{t("maintenance:list.heading")}</h1>
        <p className="text-sm text-muted-foreground">{t("maintenance:list.subheading")}</p>
      </header>

      <Card>
        <CardHeader>
          <CardTitle className="sr-only">{t("maintenance:list.heading")}</CardTitle>
        </CardHeader>
        <CardContent>
          {list.isLoading ? (
            <div className="flex flex-col gap-2" data-testid="maintenance-loading">
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
            </div>
          ) : rows.length === 0 ? (
            <p className="text-sm text-muted-foreground" data-testid="maintenance-empty">
              {t("maintenance:list.empty")}
            </p>
          ) : (
            <table className="w-full text-sm" data-testid="maintenance-table">
              <thead className="text-left text-xs text-muted-foreground">
                <tr>
                  <th className="px-2 py-2 font-medium">{t("maintenance:list.commodity")}</th>
                  <th className="px-2 py-2 font-medium">{t("maintenance:list.schedule")}</th>
                  <th className="px-2 py-2 font-medium">{t("maintenance:list.nextDue")}</th>
                  <th className="px-2 py-2 font-medium">{t("maintenance:list.lastDone")}</th>
                </tr>
              </thead>
              <tbody>
                {rows.map(({ schedule, commodity }) => {
                  const days = daysUntilDue(schedule)
                  const overdue = days !== null && days < 0
                  const dueSoon = days !== null && days >= 0 && days <= 14
                  return (
                    <tr
                      key={schedule.id}
                      className={cn(
                        "border-t border-border",
                        overdue ? "bg-destructive/5" : dueSoon ? "bg-amber-50/40" : undefined
                      )}
                      data-testid={`maintenance-row-${schedule.id}`}
                    >
                      <td className="px-2 py-2">
                        {commodity ? (
                          <Link
                            to={`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodity.id)}?tab=maintenance`}
                            className="font-medium hover:underline"
                          >
                            {commodity.name}
                          </Link>
                        ) : (
                          <span className="text-muted-foreground">
                            {t("maintenance:list.noCommodity")}
                          </span>
                        )}
                      </td>
                      <td className="px-2 py-2">
                        <div className="font-medium">{schedule.title}</div>
                        <div className="text-xs text-muted-foreground">
                          {t("maintenance:row.intervalLabel", {
                            defaultValue: "Every {{count}} days",
                            count: schedule.interval_days ?? 0,
                          })}
                        </div>
                      </td>
                      <td className="px-2 py-2">
                        <div>{schedule.next_due_at ? formatDate(schedule.next_due_at) : "—"}</div>
                        {overdue ? (
                          <Badge variant="destructive" className="mt-1">
                            {t("maintenance:row.overdue")}
                          </Badge>
                        ) : dueSoon ? (
                          <Badge variant="secondary" className="mt-1">
                            {t("maintenance:row.dueSoon")}
                          </Badge>
                        ) : !schedule.enabled ? (
                          <Badge variant="outline" className="mt-1">
                            {t("maintenance:row.paused")}
                          </Badge>
                        ) : null}
                      </td>
                      <td className="px-2 py-2 text-muted-foreground">
                        {schedule.last_done_at ? formatDate(schedule.last_done_at) : "—"}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
