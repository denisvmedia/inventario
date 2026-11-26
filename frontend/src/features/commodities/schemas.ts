import { z } from "zod"

import type { CustomField } from "@/features/customFields/api"
import { validateFormValue } from "@/features/customFields/values"

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

const NO_PRICE_IN_GROUP_CURRENCY = "commodities:validation.noPriceInGroupCurrency"
const NAME_REQUIRED = "commodities:validation.nameRequired"
const NAME_TOO_LONG = "commodities:validation.nameTooLong"
const SHORT_NAME_REQUIRED = "commodities:validation.shortNameRequired"
const SHORT_NAME_TOO_LONG = "commodities:validation.shortNameTooLong"
const BRAND_TOO_LONG = "commodities:validation.brandTooLong"
const MODEL_TOO_LONG = "commodities:validation.modelTooLong"
const TYPE_REQUIRED = "commodities:validation.typeRequired"
const STATUS_REQUIRED = "commodities:validation.statusRequired"
const COUNT_MIN = "commodities:validation.countMin"
const CURRENCY_REQUIRED = "commodities:validation.currencyRequired"
const PURCHASE_DATE_FUTURE = "commodities:validation.purchaseDateFuture"
const PRICE_NEGATIVE = "commodities:validation.priceNegative"
const COMMENTS_TOO_LONG = "commodities:validation.commentsTooLong"
const NOT_A_NUMBER = "commodities:validation.notANumber"
// #1554: a Count > 1 commodity (a bundle of interchangeable units)
// can't carry a warranty — it's not a single tracked instance. The
// schema rejects the pair at submit time so the user sees the same
// hint the BE 422 would surface, just earlier.
const QUANTITY_FORBIDS_WARRANTY = "commodities:validation.quantityForbidsWarranty"
const URL_INVALID = "commodities:validation.urlInvalid"

// optionalNumberString accepts a number-as-string and refuses anything
// that isn't blank or numeric. It stays a string in the schema so the
// form's TFieldValues type doesn't pick up a string|number union.
const optionalNumberString = z
  .string()
  .refine((v) => v === "" || !Number.isNaN(Number(v)), { message: NOT_A_NUMBER })

// optionalPriceString is a price field: blank means the price is unknown,
// "0" is a known zero (a gift, a free item). A known price can't be
// negative, as on the BE.
const optionalPriceString = optionalNumberString.refine((v) => v.trim() === "" || Number(v) >= 0, {
  message: PRICE_NEGATIVE,
})

// isKnownPrice reports whether a price field holds an amount. Zero counts.
function isKnownPrice(raw: string | undefined): boolean {
  const v = (raw ?? "").trim()
  return v !== "" && !Number.isNaN(Number(v))
}

// urlOrEmpty accepts a blank string (the user added the row but never
// typed) OR a value that parses as an http(s) URL with a host —
// matches the BE's URL validator (`models/url.go`: scheme in {http,
// https} + non-empty host). Empty entries are dropped at submit time
// inside `toRequest`, so they don't reach the BE.
const urlOrEmpty = z
  .string()
  .trim()
  .refine(
    (v) => {
      if (v === "") return true
      try {
        const u = new URL(v)
        return (u.protocol === "http:" || u.protocol === "https:") && !!u.hostname
      } catch {
        return false
      }
    },
    { message: URL_INVALID }
  )

// buildCommoditySchema closes over the active group's currency so
// the cross-field price-in-group-currency check (PriceRule.ErrNoPrice
// InGroupCurrency on the BE) only fires when the purchase currency
// differs from the group's. When the user is buying in the group's
// own currency the converted amount is the same number — the mock
// skips the field entirely (AddItemDialog L1198 `isForeignCurrency`
// branch) and so do we. Pass an empty string to skip the cross-field
// check entirely (used by tests / boot states where the active group
// hasn't loaded yet).
export function buildCommoditySchema(
  groupCurrency: string = "",
  customFields: readonly CustomField[] = []
) {
  const groupCurrencyUpper = groupCurrency.trim().toUpperCase()
  return baseCommoditySchema.superRefine((vals, ctx) => {
    // A custom field value the server would refuse, on a draft too: the
    // server checks the values of every write.
    for (const field of customFields) {
      const v = vals.custom_fields?.[field.id]
      const key = v ? validateFormValue(field, v) : undefined
      if (key) {
        ctx.addIssue({
          path: ["custom_fields", field.id, "value"],
          code: z.ZodIssueCode.custom,
          message: `customFields:errors.${key}`,
        })
      }
    }
    if (vals.draft) return
    if (!groupCurrencyUpper) return
    const purchaseCurrencyUpper = (vals.original_price_currency ?? "").trim().toUpperCase()
    // Same currency ⇒ converted price is moot, skip the requirement.
    if (purchaseCurrencyUpper === groupCurrencyUpper) return
    // An unknown purchase price needs no conversion: there is no amount
    // to convert, and the BE asks for none.
    if (!isKnownPrice(vals.original_price)) return
    // A known foreign-currency price needs an amount in the group
    // currency: the converted price or the current value, a known zero
    // included. Mirrors PriceRule.ErrNoPriceInGroupCurrency on the BE
    // (go/models/rules/price.go), so the user sees the hint here instead
    // of a 422 from the server.
    const convertedSet = isKnownPrice(vals.converted_original_price)
    const currentSet = isKnownPrice(vals.current_price)
    if (!convertedSet && !currentSet) {
      ctx.addIssue({
        path: ["converted_original_price"],
        code: z.ZodIssueCode.custom,
        message: NO_PRICE_IN_GROUP_CURRENCY,
      })
      ctx.addIssue({
        path: ["current_price"],
        code: z.ZodIssueCode.custom,
        message: NO_PRICE_IN_GROUP_CURRENCY,
      })
    }
  })
}

// The longest brand and model the BE accepts (models.MaxCommodityBrandRunes
// and MaxCommodityModelRunes).
export const BRAND_MAX_LEN = 100
export const MODEL_MAX_LEN = 100

const baseCommoditySchema = z
  .object({
    name: z.string().trim().min(1, NAME_REQUIRED).max(200, NAME_TOO_LONG),
    // BE always requires `short_name` regardless of draft (rules.NotEmpty
    // unconditional in models.Commodity.ValidateWithContext).
    short_name: z.string().trim().min(1, SHORT_NAME_REQUIRED).max(40, SHORT_NAME_TOO_LONG),
    brand: z.string().trim().max(BRAND_MAX_LEN, BRAND_TOO_LONG),
    model: z.string().trim().max(MODEL_MAX_LEN, MODEL_TOO_LONG),
    type: z.string().min(1, TYPE_REQUIRED),
    // Area is optional (#1987): the create dialog no longer asks for a
    // location, and an item can be left unassigned (#1986). Edit mode
    // still exposes the picker but never requires it.
    area_id: z.string(),
    // The seller the item was bought from (#2804), "" for none.
    seller_id: z.string(),
    status: z.string().min(1, STATUS_REQUIRED),
    count: z.string().refine((v) => /^\d+$/.test(v) && Number(v) >= 1, { message: COUNT_MIN }),
    original_price: optionalPriceString,
    original_price_currency: z.string().min(1, CURRENCY_REQUIRED),
    converted_original_price: optionalPriceString,
    current_price: optionalPriceString,
    serial_number: z.string().trim(),
    extra_serial_numbers: z.array(z.string().trim()),
    part_numbers: z.array(z.string().trim()),
    tags: z.array(z.string().trim()),
    purchase_date: z.string().trim(),
    urls: z.array(urlOrEmpty),
    comments: z.string().max(1000, COMMENTS_TOO_LONG),
    draft: z.boolean(),
    // Warranty section (#1367). Both fields are optional — a commodity
    // without a tracked warranty surfaces as "none" both in the FE pill
    // and the BE filter. Notes capped to match the backend's TEXT
    // column convention; date is plain ISO YYYY-MM-DD with no future
    // bound (a 5-year warranty starting today is normal).
    warranty_expires_at: z.string().trim(),
    warranty_notes: z.string().max(1000, COMMENTS_TOO_LONG),
    // Custom field values by field id (#2803), as text the way the other
    // inputs keep numbers; see features/customFields/values.ts. A key is a
    // field the form shows. The checks need the fields' types, so they
    // live in buildCommoditySchema.
    custom_fields: z
      .record(z.string(), z.object({ value: z.string(), currency: z.string() }))
      .optional(),
  })
  .superRefine((vals, ctx) => {
    // #1554: bundle commodities (count > 1) don't carry warranty. Fire
    // a per-field message so RHF surfaces it next to each offending
    // input, plus a single `count` issue so the Quantity field is also
    // highlighted (one count issue total, even when both warranty
    // fields are populated — multiple identical issues on the same
    // path would render duplicate error rows).
    const count = Number(vals.count)
    const hasWarrantyExpiry = !!vals.warranty_expires_at && vals.warranty_expires_at.trim() !== ""
    const hasWarrantyNotes = !!vals.warranty_notes && vals.warranty_notes.trim() !== ""
    if (count > 1 && (hasWarrantyExpiry || hasWarrantyNotes)) {
      if (hasWarrantyExpiry) {
        ctx.addIssue({
          path: ["warranty_expires_at"],
          code: z.ZodIssueCode.custom,
          message: QUANTITY_FORBIDS_WARRANTY,
        })
      }
      if (hasWarrantyNotes) {
        ctx.addIssue({
          path: ["warranty_notes"],
          code: z.ZodIssueCode.custom,
          message: QUANTITY_FORBIDS_WARRANTY,
        })
      }
      ctx.addIssue({
        path: ["count"],
        code: z.ZodIssueCode.custom,
        message: QUANTITY_FORBIDS_WARRANTY,
      })
    }
    // Future-date guard. Surface the error on `purchase_date` directly
    // so RHF puts it next to the input. A blank purchase date is an
    // unknown one and is fine on any item, draft or reviewed.
    if (vals.purchase_date) {
      // Local components, not toISOString: the input is a local calendar date,
      // and comparing it against UTC's today rejected a same-day purchase for
      // anyone east of UTC+10 for the first hours of their day (#2132).
      const now = new Date()
      const today = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(
        now.getDate()
      ).padStart(2, "0")}`
      if (vals.purchase_date > today) {
        ctx.addIssue({
          path: ["purchase_date"],
          code: z.ZodIssueCode.custom,
          message: PURCHASE_DATE_FUTURE,
        })
      }
    }
    // No price or purchase date is required, on a draft or on a
    // reviewed item: either may be unknown. The currency rule for a
    // known foreign-currency price lives in buildCommoditySchema.
  })

// commoditySchema is the schema without the group-currency rule, for
// callers that have no group currency to check against.
export const commoditySchema = baseCommoditySchema

export type CommodityFormInput = z.infer<typeof baseCommoditySchema>
