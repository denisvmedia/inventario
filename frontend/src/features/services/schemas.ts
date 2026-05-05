import { z } from "zod"

// serviceFormSchema backs the SendForServiceDialog (RHF + zod) on
// commodity detail. provider_name + sent_at are required; everything
// else is optional. Empty string is the canonical "no value" sentinel
// for optional fields. Cost is paired: amount and currency must both
// be set or both be empty — enforced via .superRefine so the error
// lands on cost_currency for surface-level rendering.
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
      .refine((v) => v === "" || /^[A-Z]{3}$/.test(v), "services:validation.costCurrencyInvalid"),
  })
  .superRefine((values, ctx) => {
    const amountSet = values.cost_amount !== "" && values.cost_amount !== "0"
    const currencySet = values.cost_currency !== ""
    if (amountSet !== currencySet) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ["cost_currency"],
        message: "services:validation.costPairRequired",
      })
    }
  })

export type ServiceFormInput = z.input<typeof serviceFormSchema>
export type ServiceFormOutput = z.output<typeof serviceFormSchema>
