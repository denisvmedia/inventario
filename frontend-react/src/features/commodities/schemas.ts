import { z } from "zod"

// Validation schemas for the commodity create/edit form. i18n keys live
// in the `commodities` namespace under `validation`; pages translate
// `errors[name].message` at render time the same way the location form
// (#1409) and group forms (#1413) do.
//
// All number-shaped inputs are kept as strings here so the schema's
// input/output types match — react-hook-form's resolver gets unhappy
// when zod's input type and output type diverge (which is what
// `z.coerce.number()` does). Numeric parsing happens at submit time
// inside CommodityFormDialog.toRequest.

const NAME_REQUIRED = "commodities:validation.nameRequired"
const NAME_TOO_LONG = "commodities:validation.nameTooLong"
const SHORT_NAME_REQUIRED = "commodities:validation.shortNameRequired"
const SHORT_NAME_TOO_LONG = "commodities:validation.shortNameTooLong"
const TYPE_REQUIRED = "commodities:validation.typeRequired"
const AREA_REQUIRED = "commodities:validation.areaRequired"
const STATUS_REQUIRED = "commodities:validation.statusRequired"
const COUNT_MIN = "commodities:validation.countMin"
const CURRENCY_REQUIRED = "commodities:validation.currencyRequired"
const PURCHASE_DATE_REQUIRED = "commodities:validation.purchaseDateRequired"
const PURCHASE_DATE_FUTURE = "commodities:validation.purchaseDateFuture"
const ORIGINAL_PRICE_REQUIRED = "commodities:validation.originalPriceRequired"
const CONVERTED_PRICE_REQUIRED = "commodities:validation.convertedPriceRequired"
const CURRENT_PRICE_REQUIRED = "commodities:validation.currentPriceRequired"
const COMMENTS_TOO_LONG = "commodities:validation.commentsTooLong"
const NOT_A_NUMBER = "commodities:validation.notANumber"

// optionalNumberString accepts a number-as-string and refuses anything
// that isn't blank or numeric. It stays a string in the schema so the
// form's TFieldValues type doesn't pick up a string|number union.
const optionalNumberString = z
  .string()
  .refine((v) => v === "" || !Number.isNaN(Number(v)), { message: NOT_A_NUMBER })

export const commoditySchema = z
  .object({
    name: z.string().trim().min(1, NAME_REQUIRED).max(200, NAME_TOO_LONG),
    // BE always requires `short_name` regardless of draft (rules.NotEmpty
    // unconditional in models.Commodity.ValidateWithContext).
    short_name: z
      .string()
      .trim()
      .min(1, SHORT_NAME_REQUIRED)
      .max(20, SHORT_NAME_TOO_LONG),
    type: z.string().min(1, TYPE_REQUIRED),
    area_id: z.string().min(1, AREA_REQUIRED),
    status: z.string().min(1, STATUS_REQUIRED),
    count: z
      .string()
      .refine((v) => /^\d+$/.test(v) && Number(v) >= 1, { message: COUNT_MIN }),
    original_price: optionalNumberString,
    original_price_currency: z.string().min(1, CURRENCY_REQUIRED),
    converted_original_price: optionalNumberString,
    current_price: optionalNumberString,
    serial_number: z.string().trim(),
    extra_serial_numbers: z.array(z.string().trim()),
    part_numbers: z.array(z.string().trim()),
    tags: z.array(z.string().trim()),
    purchase_date: z.string().trim(),
    urls: z.array(z.string().trim()),
    comments: z.string().max(1000, COMMENTS_TOO_LONG),
    draft: z.boolean(),
  })
  .superRefine((vals, ctx) => {
    // Future-date guard. Surface the error on `purchase_date` directly
    // so RHF puts it next to the input.
    if (vals.purchase_date) {
      const today = new Date().toISOString().slice(0, 10)
      if (vals.purchase_date > today) {
        ctx.addIssue({
          path: ["purchase_date"],
          code: z.ZodIssueCode.custom,
          message: PURCHASE_DATE_FUTURE,
        })
      }
    }
    // Non-draft commodities require purchase_date and the price triad
    // (original / converted / current) — see
    // models.Commodity.ValidateWithContext's `whenNotDraft` block. Drafts
    // skip these checks so the user can save partial state.
    if (vals.draft) return
    if (!vals.purchase_date) {
      ctx.addIssue({
        path: ["purchase_date"],
        code: z.ZodIssueCode.custom,
        message: PURCHASE_DATE_REQUIRED,
      })
    }
    if (vals.original_price === "") {
      ctx.addIssue({
        path: ["original_price"],
        code: z.ZodIssueCode.custom,
        message: ORIGINAL_PRICE_REQUIRED,
      })
    }
    if (vals.converted_original_price === "") {
      ctx.addIssue({
        path: ["converted_original_price"],
        code: z.ZodIssueCode.custom,
        message: CONVERTED_PRICE_REQUIRED,
      })
    }
    if (vals.current_price === "") {
      ctx.addIssue({
        path: ["current_price"],
        code: z.ZodIssueCode.custom,
        message: CURRENT_PRICE_REQUIRED,
      })
    }
  })

export type CommodityFormInput = z.infer<typeof commoditySchema>
