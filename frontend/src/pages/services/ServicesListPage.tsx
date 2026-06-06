import { useTranslation } from "react-i18next"
import { Link, useSearchParams } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import { Page, PageHeader } from "@/components/ui/page"
import { Skeleton } from "@/components/ui/skeleton"
import { useGroupServices } from "@/features/services/hooks"
import { daysOverdue, isOpen, type ServiceState } from "@/features/services/api"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

const VALID_STATES: readonly ServiceState[] = ["all", "open", "overdue", "completed"]

function parseState(raw: string | null): ServiceState {
  return (VALID_STATES as readonly string[]).includes(raw ?? "") ? (raw as ServiceState) : "all"
}

// ServicesListPage is the dedicated /in-service surface — group-wide
// list of service rows with a state filter (open / overdue / completed
// / all). Mirrors LoansListPage; the table includes "Provider" and
// "Reason" columns that are service-specific, plus a sortable
// "Expected back" column (sort handled client-side via the BE's
// default DESC ordering on sent_at).
export function ServicesListPage() {
  const { t } = useTranslation(["services", "common"])
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const [searchParams, setSearchParams] = useSearchParams()
  const state = parseState(searchParams.get("state"))

  const list = useGroupServices({ state, perPage: 50 })

  function setState(next: ServiceState) {
    const params = new URLSearchParams(searchParams)
    if (next === "all") {
      params.delete("state")
    } else {
      params.set("state", next)
    }
    setSearchParams(params, { replace: true })
  }

  return (
    <Page width="wide" data-testid="page-in-service">
      <PageHeader title={t("services:list.title")} subtitle={t("services:list.subtitle")} />

      <div
        role="tablist"
        className="flex gap-1 border-b border-border"
        data-testid="in-service-state-tabs"
      >
        {VALID_STATES.map((s) => (
          <button
            key={s}
            role="tab"
            type="button"
            aria-selected={state === s}
            onClick={() => setState(s)}
            data-testid={`in-service-state-${s}`}
            className={cn(
              "px-3 py-2 text-sm border-b-2 -mb-px",
              state === s
                ? "border-primary text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
          >
            {t(`services:list.state${s.charAt(0).toUpperCase() + s.slice(1)}`)}
          </button>
        ))}
      </div>

      <Card>
        <CardContent>
          <h2 className="sr-only">{t("services:list.title")}</h2>
          {list.isLoading ? (
            <div className="flex flex-col gap-2" data-testid="in-service-loading">
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
            </div>
          ) : list.data && list.data.services.length === 0 ? (
            <p className="text-sm text-muted-foreground" data-testid="in-service-empty">
              {t("services:list.empty")}
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-sm" data-testid="in-service-table">
                <thead className="whitespace-nowrap text-left text-xs text-muted-foreground">
                  <tr>
                    <th className="px-2 py-2 font-medium">{t("services:list.headerItem")}</th>
                    <th className="px-2 py-2 font-medium">{t("services:list.headerProvider")}</th>
                    <th className="px-2 py-2 font-medium">{t("services:list.headerReason")}</th>
                    <th className="px-2 py-2 font-medium">{t("services:list.headerSentAt")}</th>
                    <th className="px-2 py-2 font-medium">
                      {t("services:list.headerExpectedReturnAt")}
                    </th>
                    <th className="px-2 py-2 font-medium">{t("services:list.headerCost")}</th>
                    <th className="px-2 py-2 font-medium">{t("services:list.headerStatus")}</th>
                  </tr>
                </thead>
                <tbody>
                  {(list.data?.services ?? []).map(({ service, commodity }) => {
                    const overdueDays = daysOverdue(service)
                    const open = isOpen(service)
                    return (
                      <tr
                        key={service.id}
                        className="border-t border-border"
                        data-testid={`in-service-row-${service.id}`}
                      >
                        <td className="px-2 py-2">
                          {commodity ? (
                            <Link
                              to={`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodity.id)}`}
                              className="font-medium hover:underline"
                            >
                              {commodity.name}
                            </Link>
                          ) : (
                            <span className="text-muted-foreground">—</span>
                          )}
                        </td>
                        <td className="px-2 py-2">
                          <span>{service.provider_name}</span>
                          {service.provider_contact ? (
                            <span className="ml-1 text-muted-foreground">
                              ({service.provider_contact})
                            </span>
                          ) : null}
                        </td>
                        <td className="px-2 py-2 text-muted-foreground">
                          {service.reason ? service.reason : "—"}
                        </td>
                        <td className="whitespace-nowrap px-2 py-2 text-muted-foreground">
                          {service.sent_at ? formatDate(service.sent_at as string) : ""}
                        </td>
                        <td className="whitespace-nowrap px-2 py-2 text-muted-foreground">
                          {service.expected_return_at
                            ? formatDate(service.expected_return_at as string)
                            : "—"}
                        </td>
                        <td className="whitespace-nowrap px-2 py-2 text-muted-foreground">
                          {service.cost_amount && service.cost_currency ? (
                            <>
                              {service.cost_amount} {service.cost_currency}
                            </>
                          ) : (
                            "—"
                          )}
                        </td>
                        <td className="px-2 py-2">
                          {!open ? (
                            <Badge variant="secondary">
                              {t("services:list.returnedStatus", {
                                date: service.returned_at
                                  ? formatDate(service.returned_at as string)
                                  : "",
                              })}
                            </Badge>
                          ) : overdueDays > 0 ? (
                            <Badge variant="destructive">
                              {t("services:list.overdueStatus", { count: overdueDays })}
                            </Badge>
                          ) : (
                            <Badge>{t("services:list.openStatus")}</Badge>
                          )}
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            </div>
          )}
        </CardContent>
      </Card>
    </Page>
  )
}
