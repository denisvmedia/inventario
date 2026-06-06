import { useTranslation } from "react-i18next"
import { Link, useSearchParams } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent } from "@/components/ui/card"
import { Page, PageHeader } from "@/components/ui/page"
import { Skeleton } from "@/components/ui/skeleton"
import { useGroupLoans } from "@/features/loans/hooks"
import { daysOverdue, isOpen, type LoanState } from "@/features/loans/api"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

const VALID_STATES: readonly LoanState[] = ["all", "open", "overdue", "returned"]

function parseState(raw: string | null): LoanState {
  return (VALID_STATES as readonly string[]).includes(raw ?? "") ? (raw as LoanState) : "all"
}

// LoansListPage is the dedicated /lent surface — group-wide list of
// loans with a state filter (open / overdue / returned / all). Each
// row links to the parent commodity so the user can drill into the
// Lend tab there to mark a return or update the borrower contact.
export function LoansListPage() {
  const { t } = useTranslation(["loans", "common"])
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const [searchParams, setSearchParams] = useSearchParams()
  const state = parseState(searchParams.get("state"))

  const list = useGroupLoans({ state, perPage: 50 })

  function setState(next: LoanState) {
    const params = new URLSearchParams(searchParams)
    if (next === "all") {
      params.delete("state")
    } else {
      params.set("state", next)
    }
    setSearchParams(params, { replace: true })
  }

  return (
    <Page width="wide" data-testid="page-lent">
      <PageHeader title={t("loans:list.title")} subtitle={t("loans:list.subtitle")} />

      <div
        role="tablist"
        className="flex gap-1 border-b border-border"
        data-testid="lent-state-tabs"
      >
        {VALID_STATES.map((s) => (
          <button
            key={s}
            role="tab"
            type="button"
            aria-selected={state === s}
            onClick={() => setState(s)}
            data-testid={`lent-state-${s}`}
            className={cn(
              "px-3 py-2 text-sm border-b-2 -mb-px",
              state === s
                ? "border-primary text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
          >
            {t(`loans:list.state${s.charAt(0).toUpperCase() + s.slice(1)}`)}
          </button>
        ))}
      </div>

      <Card>
        <CardContent>
          <h2 className="sr-only">{t("loans:list.title")}</h2>
          {list.isLoading ? (
            <div className="flex flex-col gap-2" data-testid="lent-loading">
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
              <Skeleton className="h-10" />
            </div>
          ) : list.data && list.data.loans.length === 0 ? (
            <p className="text-sm text-muted-foreground" data-testid="lent-empty">
              {t("loans:list.empty")}
            </p>
          ) : (
            <table className="w-full text-sm" data-testid="lent-table">
              <thead className="text-left text-xs text-muted-foreground">
                <tr>
                  <th className="px-2 py-2 font-medium">{t("loans:list.headerItem")}</th>
                  <th className="px-2 py-2 font-medium">{t("loans:list.headerBorrower")}</th>
                  <th className="px-2 py-2 font-medium">{t("loans:list.headerLentAt")}</th>
                  <th className="px-2 py-2 font-medium">{t("loans:list.headerDueBackAt")}</th>
                  <th className="px-2 py-2 font-medium">{t("loans:list.headerStatus")}</th>
                </tr>
              </thead>
              <tbody>
                {(list.data?.loans ?? []).map(({ loan, commodity }) => {
                  const overdueDays = daysOverdue(loan)
                  const open = isOpen(loan)
                  return (
                    <tr
                      key={loan.id}
                      className="border-t border-border"
                      data-testid={`lent-row-${loan.id}`}
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
                        <span>{loan.borrower_name}</span>
                        {loan.borrower_contact ? (
                          <span className="ml-1 text-muted-foreground">
                            ({loan.borrower_contact})
                          </span>
                        ) : null}
                      </td>
                      <td className="px-2 py-2 text-muted-foreground">
                        {loan.lent_at ? formatDate(loan.lent_at as string) : ""}
                      </td>
                      <td className="px-2 py-2 text-muted-foreground">
                        {loan.due_back_at ? formatDate(loan.due_back_at as string) : "—"}
                      </td>
                      <td className="px-2 py-2">
                        {!open ? (
                          <Badge variant="secondary">
                            {t("loans:list.returnedStatus", {
                              date: loan.returned_at ? formatDate(loan.returned_at as string) : "",
                            })}
                          </Badge>
                        ) : overdueDays > 0 ? (
                          <Badge variant="destructive">
                            {t("loans:list.overdueStatus", { count: overdueDays })}
                          </Badge>
                        ) : (
                          <Badge>{t("loans:list.openStatus")}</Badge>
                        )}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </CardContent>
      </Card>
    </Page>
  )
}
