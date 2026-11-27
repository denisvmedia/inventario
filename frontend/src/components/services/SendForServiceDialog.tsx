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
import { FieldError } from "@/components/FieldError"
import { serviceFormSchema, type ServiceFormInput } from "@/features/services/schemas"
import { applyServerFieldErrors } from "@/lib/form-errors"

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
    setError,
  } = useForm<ServiceFormInput>({
    resolver: zodResolver(serviceFormSchema),
    defaultValues: buildDefaults(),
  })

  useEffect(() => {
    if (open) reset(buildDefaults())
  }, [open, reset])

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        variant="fullscreen-mobile"
        className="sm:max-w-md"
        data-testid="service-dialog"
      >
        <DialogHeader>
          <DialogTitle>{t("services:dialog.title")}</DialogTitle>
          <DialogDescription>{t("services:dialog.description")}</DialogDescription>
        </DialogHeader>

        <form
          className="flex flex-col gap-4"
          onSubmit={handleSubmit(async (values) => {
            try {
              await onSubmit({
                provider_name: values.provider_name,
                provider_contact: values.provider_contact ?? "",
                reason: values.reason ?? "",
                sent_at: values.sent_at,
                expected_return_at: values.expected_return_at ?? "",
                cost_amount: values.cost_amount ?? "",
                cost_currency: values.cost_currency ?? "",
              })
            } catch (err) {
              // Host toasts a summary and re-throws; map field-level 422s.
              applyServerFieldErrors(err, setError, {
                fields: [
                  "provider_name",
                  "provider_contact",
                  "reason",
                  "sent_at",
                  "expected_return_at",
                  "cost_amount",
                  "cost_currency",
                ],
              })
            }
          })}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="service-provider-name">{t("services:dialog.providerName")}</Label>
            <Input
              id="service-provider-name"
              data-testid="service-provider-name"
              placeholder={t("services:dialog.providerNamePlaceholder")}
              autoComplete="off"
              aria-invalid={!!errors.provider_name}
              aria-describedby={
                errors.provider_name?.message ? "service-provider-name-error" : undefined
              }
              {...register("provider_name")}
            />
            <FieldError
              id="service-provider-name-error"
              testId="service-provider-name-error"
              message={errors.provider_name?.message}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="service-provider-contact">{t("services:dialog.providerContact")}</Label>
            <Input
              id="service-provider-contact"
              data-testid="service-provider-contact"
              placeholder={t("services:dialog.providerContactPlaceholder")}
              autoComplete="off"
              aria-invalid={!!errors.provider_contact}
              aria-describedby={
                errors.provider_contact?.message ? "service-provider-contact-error" : undefined
              }
              {...register("provider_contact")}
            />
            <p className="text-xs text-muted-foreground">
              {t("services:dialog.providerContactHint")}
            </p>
            <FieldError
              id="service-provider-contact-error"
              testId="service-provider-contact-error"
              message={errors.provider_contact?.message}
            />
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="service-reason">{t("services:dialog.reason")}</Label>
            <Input
              id="service-reason"
              data-testid="service-reason"
              placeholder={t("services:dialog.reasonPlaceholder")}
              autoComplete="off"
              aria-invalid={!!errors.reason}
              aria-describedby={errors.reason?.message ? "service-reason-error" : undefined}
              {...register("reason")}
            />
            <p className="text-xs text-muted-foreground">{t("services:dialog.reasonHint")}</p>
            <FieldError
              id="service-reason-error"
              testId="service-reason-error"
              message={errors.reason?.message}
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="service-sent-at">{t("services:dialog.sentAt")}</Label>
              <Input
                id="service-sent-at"
                type="date"
                data-testid="service-sent-at"
                aria-invalid={!!errors.sent_at}
                aria-describedby={errors.sent_at?.message ? "service-sent-at-error" : undefined}
                {...register("sent_at")}
              />
              <FieldError
                id="service-sent-at-error"
                testId="service-sent-at-error"
                message={errors.sent_at?.message}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="service-expected-return-at">
                {t("services:dialog.expectedReturnAt")}
              </Label>
              <Input
                id="service-expected-return-at"
                type="date"
                data-testid="service-expected-return-at"
                aria-invalid={!!errors.expected_return_at}
                aria-describedby={
                  errors.expected_return_at?.message
                    ? "service-expected-return-at-error"
                    : undefined
                }
                {...register("expected_return_at")}
              />
              <FieldError
                id="service-expected-return-at-error"
                testId="service-expected-return-at-error"
                message={errors.expected_return_at?.message}
              />
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
                aria-invalid={!!errors.cost_amount}
                aria-describedby={
                  errors.cost_amount?.message ? "service-cost-amount-error" : undefined
                }
                {...register("cost_amount")}
              />
              <FieldError
                id="service-cost-amount-error"
                testId="service-cost-amount-error"
                message={errors.cost_amount?.message}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="service-cost-currency">{t("services:dialog.costCurrency")}</Label>
              <Input
                id="service-cost-currency"
                data-testid="service-cost-currency"
                placeholder="EUR"
                maxLength={3}
                autoComplete="off"
                aria-invalid={!!errors.cost_currency}
                aria-describedby={
                  errors.cost_currency?.message ? "service-cost-currency-error" : undefined
                }
                {...register("cost_currency")}
              />
              <FieldError
                id="service-cost-currency-error"
                testId="service-cost-currency-error"
                message={errors.cost_currency?.message}
              />
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
