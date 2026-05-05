import { z } from "zod"

// loanFormSchema backs the LendDialog (RHF + zod) on commodity detail.
// `borrower_name` is required; everything else is optional. Empty
// string is the canonical "no value" sentinel for optional fields so
// the form re-mounts with the same shape after submission.
//
// The schema validates due_back_at format only — it does NOT transform
// empty strings. The `|| undefined` normalisation that strips
// `due_back_at: ""` from the API payload happens at the LendTab submit
// call site (so the BE Date validator never sees an empty string).
// Folding the strip into the schema would mean either a `.transform`
// here (changes the parsed type to `string | undefined`, ripples through
// LendFormInput / LendFormOutput) or relying on the API client to drop
// empties — the call-site approach keeps the form types simple.
//
// Validation messages are i18n keys so the form can render localised
// errors via `t(form.formState.errors.X.message)` — same pattern used
// by the auth + commodities forms.
export const lendFormSchema = z.object({
  borrower_name: z
    .string()
    .trim()
    .min(1, "loans:validation.borrowerNameRequired")
    .max(200, "loans:validation.borrowerNameTooLong"),
  borrower_contact: z
    .string()
    .max(200, "loans:validation.borrowerContactTooLong")
    .optional()
    .default(""),
  borrower_note: z
    .string()
    .max(1000, "loans:validation.borrowerNoteTooLong")
    .optional()
    .default(""),
  lent_at: z
    .string()
    .min(1, "loans:validation.lentAtRequired")
    .regex(/^\d{4}-\d{2}-\d{2}$/, "loans:validation.lentAtInvalid"),
  due_back_at: z
    .string()
    .optional()
    .default("")
    .refine((v) => v === "" || /^\d{4}-\d{2}-\d{2}$/.test(v), "loans:validation.dueBackAtInvalid"),
})

export type LendFormInput = z.input<typeof lendFormSchema>
export type LendFormOutput = z.output<typeof lendFormSchema>
