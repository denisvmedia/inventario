import { useState } from "react"
import { useTranslation } from "react-i18next"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { isOpen as loanIsOpen } from "@/features/loans/api"
import { useLoansForCommodity } from "@/features/loans/hooks"
import { daysOverdue, hasCost, isOpen, type ServiceEntity } from "@/features/services/api"
import {
  useDeleteService,
  useReturnService,
  useServicesForCommodity,
  useStartService,
} from "@/features/services/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatDate } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"

import { SendForServiceDialog } from "./SendForServiceDialog"

interface ServiceTabProps {
  commodityId: string
  // #1554: bundle commodities (count > 1) cannot be sent for service.
  // Mirror LendTab — render an empty-state hint and hide the CTA.
  commodityCount?: number
}

// ServiceTab is the in-service surface on commodity detail. Mirrors
// LendTab structurally — at most one open service row + a history
// list — but the copy and icons emphasise repair / workshop semantics
// instead of borrower semantics.
export function ServiceTab({ commodityId, commodityCount }: ServiceTabProps) {
  const { t } = useTranslation(["services", "common"])
  const toast = useAppToast()
  const confirm = useConfirm()
  const [open, setOpen] = useState(false)

  const isBundle = (commodityCount ?? 0) > 1
  const list = useServicesForCommodity(commodityId)
  const start = useStartService()
  const ret = useReturnService()
  const remove = useDeleteService()
  // Cross-kind invariant: a commodity that is currently lent out cannot
  // be sent for service. Pull the loan list for this commodity so we
  // can hide the Send-for-service action and explain why — sparing the
  // user from filling the dialog only to discover the 409 on submit.
  const loanList = useLoansForCommodity(commodityId)
  const openLoan = (loanList.data?.loans ?? []).find((l) => loanIsOpen(l))

  const services = list.data?.services ?? []
  const current = services.find((s) => isOpen(s))
  const history = services.filter((s) => s.id !== current?.id)

  async function handleSubmit(values: {
    provider_name: string
    provider_contact: string
    reason: string
    sent_at: string
    expected_return_at: string
    cost_amount: string
    cost_currency: string
  }) {
    try {
      await start.mutateAsync({
        commodity_id: commodityId,
        provider_name: values.provider_name,
        provider_contact: values.provider_contact || undefined,
        reason: values.reason || undefined,
        sent_at: values.sent_at,
        expected_return_at: values.expected_return_at || undefined,
        cost_amount: values.cost_amount || undefined,
        cost_currency: values.cost_currency || undefined,
      })
      toast.success(t("services:toast.sendSuccess"))
      setOpen(false)
    } catch (err) {
      // HttpError.message is the generic "Request to ... failed with NNN"
      // string; the human-readable BE message lives in `err.data` (the
      // parsed JSON:API error envelope). parseServerError pulls the
      // detail/title/error/message field out and falls back to a generic
      // toast copy when nothing useful is in the payload.
      toast.error(parseServerError(err, t("services:toast.sendError")))
    }
  }

  async function handleReturn(svc: ServiceEntity & { id: string }) {
    const ok = await confirm({
      title: t("services:confirm.markReturned"),
      confirmLabel: t("services:current.markReturned"),
    })
    if (!ok) return
    try {
      await ret.mutateAsync({ commodityID: commodityId, serviceID: svc.id })
      toast.success(t("services:toast.returnSuccess"))
    } catch (err) {
      toast.error(parseServerError(err, t("services:toast.returnError")))
    }
  }

  async function handleDelete(svc: ServiceEntity & { id: string }) {
    const ok = await confirm({
      title: t("services:confirm.delete"),
      destructive: true,
      confirmLabel: t("services:current.delete"),
    })
    if (!ok) return
    try {
      await remove.mutateAsync({ commodityID: commodityId, serviceID: svc.id })
      toast.success(t("services:toast.deleteSuccess"))
    } catch (err) {
      toast.error(parseServerError(err, t("services:toast.deleteError")))
    }
  }

  return (
    <Card data-testid="commodity-detail-service">
      <CardHeader className="flex-row items-center justify-between gap-4">
        <CardTitle>{t("services:tab.title")}</CardTitle>
        {!current && !openLoan && !isBundle ? (
          <Button
            type="button"
            size="sm"
            onClick={() => setOpen(true)}
            data-testid="commodity-detail-service-button"
          >
            {t("services:tab.sendButton")}
          </Button>
        ) : null}
      </CardHeader>
      <CardContent className="flex flex-col gap-6">
        {isBundle ? (
          <p
            className="text-sm text-muted-foreground"
            data-testid="service-bundle-empty-state"
          >
            {t("commodities:trackingRestrictions.serviceDisabled")}
          </p>
        ) : null}

        {!isBundle && list.isLoading ? (
          <p className="text-sm text-muted-foreground">{t("common:loading", "Loading...")}</p>
        ) : null}

        {!isBundle && !current && openLoan ? (
          <Alert data-testid="service-blocked-by-loan">
            <AlertDescription>
              {t("services:tab.blockedByLoan", { borrower: openLoan.borrower_name })}
            </AlertDescription>
          </Alert>
        ) : null}

        {!isBundle && current ? (
          <CurrentServiceCard
            service={current}
            onReturn={() => handleReturn(current)}
            onDelete={() => handleDelete(current)}
            returning={ret.isPending}
            deleting={remove.isPending}
          />
        ) : !isBundle && !list.isLoading ? (
          <p className="text-sm text-muted-foreground" data-testid="service-empty-state">
            {t("services:tab.emptyState")}
          </p>
        ) : null}

        {!isBundle && history.length > 0 ? (
          <div className="flex flex-col gap-2" data-testid="service-history">
            <h3 className="text-sm font-medium">{t("services:tab.historyTitle")}</h3>
            <ul className="flex flex-col gap-2">
              {history.map((svc) => (
                <li
                  key={svc.id}
                  className="flex items-baseline justify-between gap-2 text-sm"
                  data-testid={`service-history-row-${svc.id}`}
                >
                  <span className="truncate">
                    <span className="font-medium">{svc.provider_name}</span>
                    {svc.reason ? (
                      <span className="ml-1 text-muted-foreground">({svc.reason})</span>
                    ) : null}
                  </span>
                  <span className="text-xs text-muted-foreground whitespace-nowrap">
                    {formatDate(svc.sent_at as string)}
                    {svc.returned_at ? ` → ${formatDate(svc.returned_at as string)}` : ""}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        ) : null}
      </CardContent>

      <SendForServiceDialog
        open={open}
        onOpenChange={setOpen}
        onSubmit={handleSubmit}
        isPending={start.isPending}
      />
    </Card>
  )
}

interface CurrentServiceCardProps {
  service: ServiceEntity & { id: string }
  onReturn: () => void
  onDelete: () => void
  returning: boolean
  deleting: boolean
}

function CurrentServiceCard({
  service,
  onReturn,
  onDelete,
  returning,
  deleting,
}: CurrentServiceCardProps) {
  const { t } = useTranslation(["services"])
  const overdueDays = daysOverdue(service)
  return (
    <div
      className={
        overdueDays > 0
          ? "rounded-md border border-amber-300 bg-amber-50 p-3 dark:bg-amber-950/30"
          : "rounded-md border bg-muted/30 p-3"
      }
      data-testid="service-current"
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <p className="text-sm font-medium">
            {t("services:current.atProvider", { name: service.provider_name })}
          </p>
          {service.provider_contact ? (
            <p className="text-xs text-muted-foreground">{service.provider_contact}</p>
          ) : null}
          <p className="text-xs text-muted-foreground">
            {t("services:current.sentOn", { date: formatDate(service.sent_at as string) })}
            {service.expected_return_at ? (
              <>
                {" — "}
                {t("services:current.expectedOn", {
                  date: formatDate(service.expected_return_at as string),
                })}
              </>
            ) : null}
          </p>
          {service.reason ? <p className="text-sm">{service.reason}</p> : null}
          {hasCost(service) ? (
            <p className="text-xs text-muted-foreground">
              {t("services:current.cost", {
                amount: service.cost_amount,
                currency: service.cost_currency,
              })}
            </p>
          ) : null}
          {overdueDays > 0 ? (
            <Badge
              variant="destructive"
              className="mt-1 self-start"
              data-testid="service-overdue-badge"
            >
              {t("services:current.overdue", { count: overdueDays })}
            </Badge>
          ) : null}
        </div>
        <div className="flex flex-col gap-1.5">
          <Button
            type="button"
            size="sm"
            onClick={onReturn}
            disabled={returning}
            data-testid="service-mark-returned"
          >
            {t("services:current.markReturned")}
          </Button>
          <Button
            type="button"
            size="sm"
            variant="ghost"
            onClick={onDelete}
            disabled={deleting}
            className="text-destructive"
            data-testid="service-delete"
          >
            {t("services:current.delete")}
          </Button>
        </div>
      </div>
    </div>
  )
}
