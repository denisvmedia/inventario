import { z } from "zod"

// loanFormSchema backs the LendDialog (RHF + zod) on commodity detail.
// `borrower_name` is required; everything else is optional. Empty
// string is the canonical "no value" sentinel for optional fields so
// the form re-mounts with the same shape after submission. The
// transform on `due_back_at` strips empty strings so the API call
// doesn't send `due_back_at: ""` (which the BE Date validator
// rejects).
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
    .refine(
      (v) => v === "" || /^\d{4}-\d{2}-\d{2}$/.test(v),
      "loans:validation.dueBackAtInvalid"
    ),
})

export type LendFormInput = z.input<typeof lendFormSchema>
export type LendFormOutput = z.output<typeof lendFormSchema>
