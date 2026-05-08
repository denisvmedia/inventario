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

// editLoanFormSchema backs the EditLoanDialog (issue #1513). It's a
// strict subset of lendFormSchema — only the fields that PATCH allows
// (borrower_name / contact / note + due_back_at). lent_at is
// intentionally read-only on edit (changing the lend date after the
// fact is audit confusion — see UpdateLoan in commodity_loan_service.go).
//
// due_back_at uses the same "" sentinel for "not set"; the form's
// submit handler maps the sentinel + the original loan's value to the
// tri-state PATCH payload (absent / null / value).
export const editLoanFormSchema = z.object({
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
  due_back_at: z
    .string()
    .optional()
    .default("")
    .refine((v) => v === "" || /^\d{4}-\d{2}-\d{2}$/.test(v), "loans:validation.dueBackAtInvalid"),
})

export type EditLoanFormInput = z.input<typeof editLoanFormSchema>
export type EditLoanFormOutput = z.output<typeof editLoanFormSchema>
