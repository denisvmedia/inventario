import { z } from "zod"

// Validation schemas for the commodity create/edit form. i18n keys live
// in the `commodities` namespace under `validation`; pages translate
// `errors[name].message` at render time the same way the location form
// (#1409) and group forms (#1413) do.

const NAME_REQUIRED = "commodities:validation.nameRequired"
const NAME_TOO_LONG = "commodities:validation.nameTooLong"
const SHORT_NAME_TOO_LONG = "commodities:validation.shortNameTooLong"
const TYPE_REQUIRED = "commodities:validation.typeRequired"
const AREA_REQUIRED = "commodities:validation.areaRequired"
const STATUS_REQUIRED = "commodities:validation.statusRequired"
const COUNT_MIN = "commodities:validation.countMin"
const CURRENCY_REQUIRED = "commodities:validation.currencyRequired"
const PURCHASE_DATE_FUTURE = "commodities:validation.purchaseDateFuture"
const COMMENTS_TOO_LONG = "commodities:validation.commentsTooLong"

// Zod-coerced number that preserves "" as undefined (so the resolver
// doesn't mark blank optional-number fields invalid). zod's
// `coerce.number()` would coerce "" to 0 and fail min(0) checks the
// wrong way.
const optionalNumber = z
  .union([
    z.string().transform((v) => (v === "" ? undefined : Number(v))),
    z.number(),
    z.undefined(),
  ])
  .refine((v) => v === undefined || (typeof v === "number" && !Number.isNaN(v)), {
    message: "commodities:validation.notANumber",
  })

export const commoditySchema = z
  .object({
    name: z.string().trim().min(1, NAME_REQUIRED).max(200, NAME_TOO_LONG),
    short_name: z.string().trim().max(20, SHORT_NAME_TOO_LONG),
    type: z.string().min(1, TYPE_REQUIRED),
    area_id: z.string().min(1, AREA_REQUIRED),
    status: z.string().min(1, STATUS_REQUIRED),
    count: z.coerce.number().int().min(1, COUNT_MIN),
    original_price: optionalNumber,
    original_price_currency: z.string().min(1, CURRENCY_REQUIRED),
    converted_original_price: optionalNumber,
    current_price: optionalNumber,
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
    // Future-date guard. Mirrors the legacy form's superRefine — surface
    // the error on `purchase_date` directly so RHF puts it next to the
    // input. Skipped when the field is empty (drafts permit a blank
    // date until publish).
    if (!vals.purchase_date) return
    const today = new Date().toISOString().slice(0, 10)
    if (vals.purchase_date > today) {
      ctx.addIssue({
        path: ["purchase_date"],
        code: z.ZodIssueCode.custom,
        message: PURCHASE_DATE_FUTURE,
      })
    }
  })

export type CommodityFormInput = z.infer<typeof commoditySchema>
