import { z } from "zod"

// serviceFormSchema backs the SendForServiceDialog (RHF + zod) on
// commodity detail. provider_name + sent_at are required; everything
// else is optional. Empty string is the canonical "no value" sentinel
// for optional fields. Cost is paired: the parsed values carry amount
// and currency together or neither. An amount without a currency is an
// error on cost_currency. The currency picker starts on the group
// currency, so a currency without an amount means "no cost" and is
// dropped. The currency is upper-cased before it is checked.
//
// Validation messages are i18n keys so the form can render localised
// errors via `t(form.formState.errors.X.message)`.
export const serviceFormSchema = z
  .object({
    provider_name: z
      .string()
      .trim()
      .min(1, "services:validation.providerNameRequired")
      .max(200, "services:validation.providerNameTooLong"),
    provider_contact: z
      .string()
      .max(200, "services:validation.providerContactTooLong")
      .optional()
      .default(""),
    reason: z.string().max(1000, "services:validation.reasonTooLong").optional().default(""),
    sent_at: z
      .string()
      .min(1, "services:validation.sentAtRequired")
      .regex(/^\d{4}-\d{2}-\d{2}$/, "services:validation.sentAtInvalid"),
    expected_return_at: z
      .string()
      .optional()
      .default("")
      .refine(
        (v) => v === "" || /^\d{4}-\d{2}-\d{2}$/.test(v),
        "services:validation.expectedReturnAtInvalid"
      ),
    cost_amount: z
      .string()
      .optional()
      .default("")
      .refine(
        (v) => v === "" || /^\d+(\.\d{1,2})?$/.test(v),
        "services:validation.costAmountInvalid"
      ),
    cost_currency: z
      .string()
      .optional()
      .default("")
      .transform((v) => v.trim().toUpperCase())
      .refine((v) => v === "" || /^[A-Z]{3}$/.test(v), "services:validation.costCurrencyInvalid"),
  })
  .superRefine((values, ctx) => {
    // An amount needs a currency. We test only "is the field non-empty?" —
    // a literal "0" amount is a legitimate cost (warranty repair, comp
    // fix), and rejecting it would force users to fudge the bill.
    if (values.cost_amount !== "" && values.cost_currency === "") {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["cost_currency"],
        message: "services:validation.costPairRequired",
      })
    }
  })
  .transform((values) => (values.cost_amount === "" ? { ...values, cost_currency: "" } : values))

export type ServiceFormInput = z.input<typeof serviceFormSchema>
export type ServiceFormOutput = z.output<typeof serviceFormSchema>
