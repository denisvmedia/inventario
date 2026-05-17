import { zodResolver } from "@hookform/resolvers/zod"
import { useEffect } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { z } from "zod"

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
import { Textarea } from "@/components/ui/textarea"
import type { CommodityStatusValue } from "@/features/commodities/constants"

// CURRENCY_SYMBOLS mirrors the mock's `CURRENCIES` lookup. We only need
// the symbol prefix for the sale-price input adornment; the rest of
// currency handling (rendering values, conversion) lives in
// `formatCurrency` / `<CurrencyCombobox>` and is not duplicated here.
const CURRENCY_SYMBOLS: Record<string, string> = {
  USD: "$",
  EUR: "€",
  GBP: "£",
  JPY: "¥",
  CHF: "CHF",
  CAD: "$",
  AUD: "$",
  CZK: "Kč",
  PLN: "zł",
  HUF: "Ft",
  SEK: "kr",
  NOK: "kr",
  DKK: "kr",
  RON: "lei",
  BGN: "лв",
  RUB: "₽",
  UAH: "₴",
  TRY: "₺",
  CNY: "¥",
  INR: "₹",
  BRL: "R$",
  MXN: "$",
  KRW: "₩",
  IDR: "Rp",
  VND: "₫",
  THB: "฿",
  SGD: "$",
  HKD: "$",
  ZAR: "R",
}

function currencySymbolFor(code: string | undefined): string {
  if (!code) return "$"
  return CURRENCY_SYMBOLS[code] ?? code
}

// todayISO returns the local-date YYYY-MM-DD used as the default
// status-date. Matches LendDialog.todayISO: the input is `type="date"`,
// which already speaks local-calendar, so anchoring to local time
// avoids a one-day shift for users west of UTC.
function todayISO(): string {
  const d = new Date()
  const y = d.getFullYear()
  const m = String(d.getMonth() + 1).padStart(2, "0")
  const day = String(d.getDate()).padStart(2, "0")
  return `${y}-${m}-${day}`
}

// statusTransitionSchema validates the dialog payload. `status_date` is
// always required (the BE enforces the same when leaving in_use, see
// #1611's apiserver/commodities.go handler). `sale_price` is required
// AND non-negative when the target is `sold`; the FE drops it from the
// payload for any other target.
//
// Validation messages are i18n keys so the form can render localised
// errors via `t(form.formState.errors.X.message)` — same pattern used
// by the loans + auth forms.
const statusTransitionSchema = z
  .object({
    status_date: z
      .string()
      .min(1, "commodities:detail.statusTransitionDialog.errors.dateRequired")
      .regex(/^\d{4}-\d{2}-\d{2}$/, "commodities:detail.statusTransitionDialog.errors.dateInvalid"),
    status_note: z
      .string()
      .max(1024, "commodities:detail.statusTransitionDialog.errors.noteTooLong")
      .optional()
      .default(""),
    // The form uses an `<input type="number">` so the value comes in as
    // a string; we coerce here. Empty string → undefined so the cross-
    // field refinement can distinguish "not entered" from "entered 0".
    sale_price: z
      .union([
        z.literal(""),
        z
          .string()
          .regex(
            /^\d+(\.\d{1,2})?$/,
            "commodities:detail.statusTransitionDialog.errors.salePriceInvalid"
          ),
      ])
      .optional(),
    // Carried through from props so the cross-field rule can branch.
    target_status: z.string(),
  })
  .superRefine((val, ctx) => {
    if (val.target_status === "sold") {
      if (!val.sale_price || val.sale_price === "") {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          path: ["sale_price"],
          message: "commodities:detail.statusTransitionDialog.errors.salePriceRequired",
        })
      }
    }
  })

export type StatusTransitionFormInput = z.input<typeof statusTransitionSchema>

// StatusTransitionPayload is the normalised shape the caller hands to
// the PATCH wiring. `sale_price` is included only for sold; an empty
// `status_note` becomes "" (the BE column is `TEXT NOT NULL`-style:
// empty string is the "no note" sentinel).
export interface StatusTransitionPayload {
  status_date: string
  status_note: string
  sale_price?: number
}

export interface StatusTransitionDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Target terminal status the user picked from the action row. */
  targetStatus: CommodityStatusValue | null
  /** Item's original purchase currency — drives the sale-price symbol. */
  purchaseCurrency?: string
  onSubmit: (payload: StatusTransitionPayload) => Promise<void>
  isPending?: boolean
}

export function StatusTransitionDialog({
  open,
  onOpenChange,
  targetStatus,
  purchaseCurrency,
  onSubmit,
  isPending = false,
}: StatusTransitionDialogProps) {
  const { t } = useTranslation(["commodities", "common"])
  const {
    formState: { errors, isSubmitting },
    handleSubmit,
    register,
    reset,
    setValue,
  } = useForm<StatusTransitionFormInput>({
    resolver: zodResolver(statusTransitionSchema),
    defaultValues: {
      status_date: todayISO(),
      status_note: "",
      sale_price: "",
      target_status: targetStatus ?? "",
    },
  })

  // Reset on open so a previous confirmation doesn't leave stale text
  // / sale-price in the form when the user re-opens it later (or picks
  // a different terminal status from the action row).
  useEffect(() => {
    if (open) {
      reset({
        status_date: todayISO(),
        status_note: "",
        sale_price: "",
        target_status: targetStatus ?? "",
      })
    }
  }, [open, reset, targetStatus])

  // Keep target_status synced if the parent flips the status while the
  // dialog is mounted (e.g., user re-picks from the chip row without
  // closing in between).
  useEffect(() => {
    setValue("target_status", targetStatus ?? "")
  }, [targetStatus, setValue])

  if (!targetStatus) return null

  const statusLabel = t(`commodities:status.${targetStatus}`)
  const symbol = currencySymbolFor(purchaseCurrency)
  const isSold = targetStatus === "sold"

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md" data-testid="status-transition-dialog">
        <DialogHeader>
          <DialogTitle>
            {t("commodities:detail.statusTransitionDialog.title", { label: statusLabel })}
          </DialogTitle>
          <DialogDescription>
            {t(`commodities:detail.statusTransitionDialog.description.${targetStatus}`)}
          </DialogDescription>
        </DialogHeader>

        <form
          className="flex flex-col gap-4"
          // noValidate: zod owns validation; matches LendDialog so
          // webkit's HTML5 validator can't block submission on a
          // partially-typed `<input type="number">` value.
          noValidate
          onSubmit={handleSubmit(async (values) => {
            const payload: StatusTransitionPayload = {
              status_date: values.status_date,
              status_note: values.status_note ?? "",
            }
            if (isSold && values.sale_price && values.sale_price !== "") {
              payload.sale_price = Number(values.sale_price)
            }
            await onSubmit(payload)
          })}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="status-transition-date">
              {t("commodities:detail.statusTransitionDialog.dateLabel")}
            </Label>
            <Input
              id="status-transition-date"
              type="date"
              data-testid="status-transition-date"
              {...register("status_date")}
            />
            {errors.status_date?.message ? (
              <p className="text-xs text-destructive" data-testid="status-transition-date-error">
                {t(errors.status_date.message)}
              </p>
            ) : null}
          </div>

          {isSold ? (
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="status-transition-sale-price">
                {t("commodities:detail.statusTransitionDialog.salePriceLabel")}
              </Label>
              <div className="relative">
                <span className="absolute left-3 top-1/2 -translate-y-1/2 text-sm text-muted-foreground">
                  {symbol}
                </span>
                <Input
                  id="status-transition-sale-price"
                  type="number"
                  min="0"
                  step="0.01"
                  inputMode="decimal"
                  className="pl-8"
                  data-testid="status-transition-sale-price"
                  {...register("sale_price")}
                />
              </div>
              {errors.sale_price?.message ? (
                <p
                  className="text-xs text-destructive"
                  data-testid="status-transition-sale-price-error"
                >
                  {t(errors.sale_price.message)}
                </p>
              ) : null}
            </div>
          ) : null}

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="status-transition-note">
              {t("commodities:detail.statusTransitionDialog.noteLabel")}
            </Label>
            <Textarea
              id="status-transition-note"
              rows={2}
              className="resize-none"
              placeholder={t(
                `commodities:detail.statusTransitionDialog.notePlaceholder.${targetStatus}`
              )}
              data-testid="status-transition-note"
              {...register("status_note")}
            />
            {errors.status_note?.message ? (
              <p className="text-xs text-destructive" data-testid="status-transition-note-error">
                {t(errors.status_note.message)}
              </p>
            ) : null}
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isSubmitting || isPending}
              data-testid="status-transition-cancel"
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              type="submit"
              disabled={isSubmitting || isPending}
              data-testid="status-transition-confirm"
            >
              {t("common:actions.confirm")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
