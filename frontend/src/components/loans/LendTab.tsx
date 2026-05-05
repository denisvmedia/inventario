import { useState } from "react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  useDeleteLoan,
  useLoansForCommodity,
  useReturnLoan,
  useStartLoan,
} from "@/features/loans/hooks"
import { daysOverdue, isOpen, type LoanEntity } from "@/features/loans/api"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatDate } from "@/lib/intl"

import { LendDialog } from "./LendDialog"

interface LendTabProps {
  commodityId: string
}

// LendTab is the lend-out surface on the commodity detail page. It
// renders the (at most one) currently-open loan as a card with a
// "mark returned" affordance, plus a history list of closed loans for
// audit context. The card is bordered amber when overdue.
export function LendTab({ commodityId }: LendTabProps) {
  const { t } = useTranslation(["loans", "common"])
  const toast = useAppToast()
  const confirm = useConfirm()
  const [open, setOpen] = useState(false)

  const list = useLoansForCommodity(commodityId)
  const start = useStartLoan()
  const ret = useReturnLoan()
  const remove = useDeleteLoan()

  const loans = list.data?.loans ?? []
  const current = loans.find((l) => isOpen(l))
  const history = loans.filter((l) => l.id !== current?.id)

  async function handleSubmit(values: {
    borrower_name: string
    borrower_contact: string
    borrower_note: string
    lent_at: string
    due_back_at: string
  }) {
    try {
      await start.mutateAsync({
        commodity_id: commodityId,
        borrower_name: values.borrower_name,
        borrower_contact: values.borrower_contact || undefined,
        borrower_note: values.borrower_note || undefined,
        lent_at: values.lent_at,
        due_back_at: values.due_back_at || undefined,
      })
      toast.success(t("loans:toast.lendSuccess"))
      setOpen(false)
    } catch (err) {
      // The BE returns 409 with a "loan_id=..." message when there's
      // already an open loan. Surface the human-readable form of
      // whatever the server said — Error.message contains it.
      toast.error(err instanceof Error ? err.message : t("loans:toast.lendError"))
    }
  }

  async function handleReturn(loan: LoanEntity & { id: string }) {
    const ok = await confirm({
      title: t("loans:confirm.markReturned"),
      confirmLabel: t("loans:current.markReturned"),
    })
    if (!ok) return
    try {
      await ret.mutateAsync({ commodityID: commodityId, loanID: loan.id })
      toast.success(t("loans:toast.returnSuccess"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("loans:toast.returnError"))
    }
  }

  async function handleDelete(loan: LoanEntity & { id: string }) {
    const ok = await confirm({
      title: t("loans:confirm.delete"),
      destructive: true,
      confirmLabel: t("loans:current.delete"),
    })
    if (!ok) return
    try {
      await remove.mutateAsync({ commodityID: commodityId, loanID: loan.id })
      toast.success(t("loans:toast.deleteSuccess"))
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("loans:toast.deleteError"))
    }
  }

  return (
    <Card data-testid="commodity-detail-lend">
      <CardHeader className="flex-row items-center justify-between gap-4">
        <CardTitle>{t("loans:tab.title")}</CardTitle>
        {!current ? (
          <Button
            type="button"
            size="sm"
            onClick={() => setOpen(true)}
            data-testid="commodity-detail-lend-button"
          >
            {t("loans:tab.lendButton")}
          </Button>
        ) : null}
      </CardHeader>
      <CardContent className="flex flex-col gap-6">
        {list.isLoading ? (
          <p className="text-sm text-muted-foreground">{t("common:loading", "Loading...")}</p>
        ) : null}

        {current ? (
          <CurrentLoanCard
            loan={current}
            onReturn={() => handleReturn(current)}
            onDelete={() => handleDelete(current)}
            returning={ret.isPending}
            deleting={remove.isPending}
          />
        ) : !list.isLoading ? (
          <p className="text-sm text-muted-foreground" data-testid="lend-empty-state">
            {t("loans:tab.emptyState")}
          </p>
        ) : null}

        {history.length > 0 ? (
          <div className="flex flex-col gap-2" data-testid="lend-history">
            <h3 className="text-sm font-medium">{t("loans:tab.historyTitle")}</h3>
            <ul className="flex flex-col gap-2">
              {history.map((loan) => (
                <li
                  key={loan.id}
                  className="flex items-baseline justify-between gap-2 text-sm"
                  data-testid={`lend-history-row-${loan.id}`}
                >
                  <span className="truncate">
                    <span className="font-medium">{loan.borrower_name}</span>
                    {loan.borrower_contact ? (
                      <span className="ml-1 text-muted-foreground">({loan.borrower_contact})</span>
                    ) : null}
                  </span>
                  <span className="text-xs text-muted-foreground whitespace-nowrap">
                    {formatDate(loan.lent_at as string)}
                    {loan.returned_at ? ` → ${formatDate(loan.returned_at as string)}` : ""}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </CardContent>

      <LendDialog
        open={open}
        onOpenChange={setOpen}
        onSubmit={handleSubmit}
        isPending={start.isPending}
      />
    </Card>
  )
}

interface CurrentLoanCardProps {
  loan: LoanEntity & { id: string }
  onReturn: () => void
  onDelete: () => void
  returning: boolean
  deleting: boolean
}

function CurrentLoanCard({ loan, onReturn, onDelete, returning, deleting }: CurrentLoanCardProps) {
  const { t } = useTranslation(["loans"])
  const overdueDays = daysOverdue(loan)
  return (
    <div
      className={
        overdueDays > 0
          ? "rounded-md border border-amber-300 bg-amber-50 p-3 dark:bg-amber-950/30"
          : "rounded-md border bg-muted/30 p-3"
      }
      data-testid="lend-current"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <p className="text-sm font-medium">
            {t("loans:current.lentTo", { name: loan.borrower_name })}
          </p>
          {loan.borrower_contact ? (
            <p className="text-xs text-muted-foreground">{loan.borrower_contact}</p>
          ) : null}
          <p className="text-xs text-muted-foreground">
            {t("loans:current.lentOn", { date: formatDate(loan.lent_at as string) })}
            {loan.due_back_at ? (
              <>
                {" — "}
                {t("loans:current.dueOn", {
                  date: formatDate(loan.due_back_at as string),
                })}
              </>
            ) : null}
          </p>
          {loan.borrower_note ? <p className="text-sm">{loan.borrower_note}</p> : null}
          {overdueDays > 0 ? (
            <Badge
              variant="destructive"
              className="mt-1 self-start"
              data-testid="lend-overdue-badge"
            >
              {t("loans:current.overdue", { count: overdueDays })}
            </Badge>
          ) : null}
        </div>
        <div className="flex flex-col gap-1.5">
          <Button
            type="button"
            size="sm"
            onClick={onReturn}
            disabled={returning}
            data-testid="lend-mark-returned"
          >
            {t("loans:current.markReturned")}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={onDelete}
            disabled={deleting}
            className="text-destructive"
            data-testid="lend-delete"
          >
            {t("loans:current.delete")}
          </Button>
        </div>
      </div>
    </div>
  )
}
