import { useState } from "react"
import { useTranslation } from "react-i18next"

import { Alert, AlertDescription } from "@/components/ui/alert"
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
import { isOpen as serviceIsOpen } from "@/features/services/api"
import { useServicesForCommodity } from "@/features/services/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatDate } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"

import { LendDialog } from "./LendDialog"

interface LendTabProps {
  commodityId: string
  // #1554: when the parent commodity has count > 1 the row is a
  // bundle of interchangeable units, not a single tracked instance.
  // Lending isn't a meaningful operation in that case (which one
  // of the 12 bulbs is on loan?). The tab swaps its body for an
  // empty-state hint and disables the Lend CTA.
  commodityCount?: number
}

// LendTab is the lend-out surface on the commodity detail page. It
// renders the (at most one) currently-open loan as a card with a
// "mark returned" affordance, plus a history list of closed loans for
// audit context. The card is bordered amber when overdue.
export function LendTab({ commodityId, commodityCount }: LendTabProps) {
  const { t } = useTranslation(["loans", "common"])
  const toast = useAppToast()
  const confirm = useConfirm()
  const [open, setOpen] = useState(false)

  const isBundle = (commodityCount ?? 0) > 1
  const list = useLoansForCommodity(commodityId)
  const start = useStartLoan()
  const ret = useReturnLoan()
  const remove = useDeleteLoan()
  // Cross-kind invariant (#1508): a commodity that is currently in
  // service cannot be lent out. Pull the service list so the Lend
  // affordance can hide and explain instead of letting the user fill
  // the dialog only to discover the 409 on submit.
  const serviceList = useServicesForCommodity(commodityId)
  const openService = (serviceList.data?.services ?? []).find((s) => serviceIsOpen(s))

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
      // HttpError.message is the generic "Request to ... failed with NNN"
      // string; the BE conflict payload lives in `err.data` (parsed
      // JSON:API envelope). parseServerError extracts the detail field
      // so the toast actually shows "commodity is already out (kind=…,
      // id=…, party=…)" instead of the generic transport message.
      toast.error(parseServerError(err, t("loans:toast.lendError")))
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
      toast.error(parseServerError(err, t("loans:toast.returnError")))
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
      toast.error(parseServerError(err, t("loans:toast.deleteError")))
    }
  }

  return (
    <Card data-testid="commodity-detail-lend">
      <CardHeader className="flex-row items-center justify-between gap-4">
        <CardTitle>{t("loans:tab.title")}</CardTitle>
        {!current && !openService && !isBundle ? (
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
        {isBundle ? (
          <p
            className="text-sm text-muted-foreground"
            data-testid="lend-bundle-empty-state"
          >
            {t("commodities:trackingRestrictions.lendDisabled")}
          </p>
        ) : null}

        {list.isLoading && !isBundle ? (
          <p className="text-sm text-muted-foreground">{t("common:loading", "Loading...")}</p>
        ) : null}

        {!current && openService && !isBundle ? (
          <Alert data-testid="lend-blocked-by-service">
            <AlertDescription>
              {t("loans:tab.blockedByService", { provider: openService.provider_name })}
            </AlertDescription>
          </Alert>
        ) : null}

        {!isBundle && current ? (
          <CurrentLoanCard
            loan={current}
            onReturn={() => handleReturn(current)}
            onDelete={() => handleDelete(current)}
            returning={ret.isPending}
            deleting={remove.isPending}
          />
        ) : !isBundle && !list.isLoading ? (
          <p className="text-sm text-muted-foreground" data-testid="lend-empty-state">
            {t("loans:tab.emptyState")}
          </p>
        ) : null}

        {!isBundle && history.length > 0 ? (
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
