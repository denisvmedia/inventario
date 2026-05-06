import { zodResolver } from "@hookform/resolvers/zod"
import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { serviceFormSchema, type ServiceFormInput } from "@/features/services/schemas"

// SendForServiceSubmitValues mirrors LendSubmitValues' shape — every
// optional field surfaces as a non-undefined string so callers don't
// thread `?? ""` through the call chain.
export interface SendForServiceSubmitValues {
  provider_name: string
  provider_contact: string
  reason: string
  sent_at: string
  expected_return_at: string
  cost_amount: string
  cost_currency: string
}

export interface SendForServiceDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSubmit: (values: SendForServiceSubmitValues) => Promise<void>
  isPending?: boolean
}

function todayISO(): string {
  const d = new Date()
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, "0")
  const day = String(d.getDate()).padStart(2, "0")
  return `${y}-${m}-${day}`
}

const buildDefaults = (): ServiceFormInput => ({
  provider_name: "",
  provider_contact: "",
  reason: "",
  sent_at: todayISO(),
  expected_return_at: "",
  cost_amount: "",
  cost_currency: "",
})

export function SendForServiceDialog({
  open,
  onOpenChange,
  onSubmit,
  isPending = false,
}: SendForServiceDialogProps) {
  const { t } = useTranslation(["services", "common"])
  const {
    formState: { errors, isSubmitting },
    handleSubmit,
    register,
    reset,
  } = useForm<ServiceFormInput>({
    resolver: zodResolver(serviceFormSchema),
    defaultValues: buildDefaults(),
  })

  useEffect(() => {
    if (open) reset(buildDefaults())
  }, [open, reset])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md" data-testid="service-dialog">
        <DialogHeader>
          <DialogTitle>{t("services:dialog.title")}</DialogTitle>
          <DialogDescription>{t("services:dialog.description")}</DialogDescription>
        </DialogHeader>

        <form
          className="flex flex-col gap-4"
          onSubmit={handleSubmit(async (values) => {
            await onSubmit({
              provider_name: values.provider_name,
              provider_contact: values.provider_contact ?? "",
              reason: values.reason ?? "",
              sent_at: values.sent_at,
              expected_return_at: values.expected_return_at ?? "",
              cost_amount: values.cost_amount ?? "",
              cost_currency: values.cost_currency ?? "",
            })
          })}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="service-provider-name">{t("services:dialog.providerName")}</Label>
            <Input
              id="service-provider-name"
              data-testid="service-provider-name"
              placeholder={t("services:dialog.providerNamePlaceholder")}
              autoComplete="off"
              {...register("provider_name")}
            />
            {errors.provider_name?.message ? (
              <p className="text-xs text-destructive" data-testid="service-provider-name-error">
                {t(errors.provider_name.message)}
              </p>
            ) : null}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="service-provider-contact">{t("services:dialog.providerContact")}</Label>
            <Input
              id="service-provider-contact"
              data-testid="service-provider-contact"
              placeholder={t("services:dialog.providerContactPlaceholder")}
              autoComplete="off"
              {...register("provider_contact")}
            />
            <p className="text-xs text-muted-foreground">
              {t("services:dialog.providerContactHint")}
            </p>
            {errors.provider_contact?.message ? (
              <p className="text-xs text-destructive" data-testid="service-provider-contact-error">
                {t(errors.provider_contact.message)}
              </p>
            ) : null}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="service-reason">{t("services:dialog.reason")}</Label>
            <Input
              id="service-reason"
              data-testid="service-reason"
              placeholder={t("services:dialog.reasonPlaceholder")}
              autoComplete="off"
              {...register("reason")}
            />
            <p className="text-xs text-muted-foreground">{t("services:dialog.reasonHint")}</p>
            {errors.reason?.message ? (
              <p className="text-xs text-destructive" data-testid="service-reason-error">
                {t(errors.reason.message)}
              </p>
            ) : null}
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="service-sent-at">{t("services:dialog.sentAt")}</Label>
              <Input
                id="service-sent-at"
                type="date"
                data-testid="service-sent-at"
                {...register("sent_at")}
              />
              {errors.sent_at?.message ? (
                <p className="text-xs text-destructive" data-testid="service-sent-at-error">
                  {t(errors.sent_at.message)}
                </p>
              ) : null}
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="service-expected-return-at">
                {t("services:dialog.expectedReturnAt")}
              </Label>
              <Input
                id="service-expected-return-at"
                type="date"
                data-testid="service-expected-return-at"
                {...register("expected_return_at")}
              />
              {errors.expected_return_at?.message ? (
                <p
                  className="text-xs text-destructive"
                  data-testid="service-expected-return-at-error"
                >
                  {t(errors.expected_return_at.message)}
                </p>
              ) : null}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="service-cost-amount">{t("services:dialog.costAmount")}</Label>
              <Input
                id="service-cost-amount"
                inputMode="decimal"
                data-testid="service-cost-amount"
                placeholder="0.00"
                autoComplete="off"
                {...register("cost_amount")}
              />
              {errors.cost_amount?.message ? (
                <p className="text-xs text-destructive" data-testid="service-cost-amount-error">
                  {t(errors.cost_amount.message)}
                </p>
              ) : null}
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="service-cost-currency">{t("services:dialog.costCurrency")}</Label>
              <Input
                id="service-cost-currency"
                data-testid="service-cost-currency"
                placeholder="EUR"
                maxLength={3}
                autoComplete="off"
                {...register("cost_currency")}
              />
              {errors.cost_currency?.message ? (
                <p className="text-xs text-destructive" data-testid="service-cost-currency-error">
                  {t(errors.cost_currency.message)}
                </p>
              ) : null}
            </div>
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isSubmitting || isPending}
              data-testid="service-cancel"
            >
              {t("services:dialog.cancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting || isPending} data-testid="service-submit">
              {t("services:dialog.submit")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
