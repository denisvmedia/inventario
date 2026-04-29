import { z } from "zod"

// Auth form schemas. Each schema mirrors the legacy `frontend/src/views/*View.schema.ts`
// rules so the e2e behavior stays compatible during the dual-bundle window:
// client-side validation is intentionally loose where the server owns the
// real rule (email format, password complexity), surfacing only the gating
// rules ("non-empty", "matches confirmation") that affect the submit button.
//
// Strings keep raw "Email is required" English fallbacks; pages translate
// each error key via the auth namespace at render time. RHF's `errors[name]?.message`
// holds these literal strings so tests don't need to plumb i18n into the
// schemas themselves.

// rememberMe is intentionally absent: the field existed in the design mock
// but the auth-storage layer always uses localStorage, so a checkbox would
// promise behavior we don't deliver. Re-add when the persistence story
// supports a session-only mode (see #1414 / future TTL signal).
export const loginSchema = z.object({
  email: z.string().min(1, "auth:validation.emailRequired"),
  password: z.string().min(1, "auth:validation.passwordRequired"),
})
export type LoginInput = z.infer<typeof loginSchema>

export const registerSchema = z.object({
  name: z.string().min(1, "auth:validation.nameRequired").max(255, "auth:validation.nameTooLong"),
  email: z.string().min(1, "auth:validation.emailRequired"),
  password: z.string().min(1, "auth:validation.passwordRequired"),
  // Boolean refined to "must be true" rather than `z.literal(true)` so the
  // inferred input type is just `boolean` — the form holds `false` until the
  // user opts in, and zod surfaces the error at submit time.
  acceptTerms: z.boolean().refine((v) => v === true, {
    message: "auth:validation.termsRequired",
  }),
})
export type RegisterInput = z.infer<typeof registerSchema>

export const forgotPasswordSchema = z.object({
  email: z.string().min(1, "auth:validation.emailRequired"),
})
export type ForgotPasswordInput = z.infer<typeof forgotPasswordSchema>

export const resetPasswordSchema = z
  .object({
    password: z.string().min(8, "auth:validation.passwordMinLength"),
    confirmPassword: z.string().min(1, "auth:validation.passwordConfirmRequired"),
  })
  .superRefine((value, ctx) => {
    if (value.password !== value.confirmPassword) {
      ctx.addIssue({
        code: "custom",
        path: ["confirmPassword"],
        message: "auth:validation.passwordsMismatch",
      })
    }
  })
export type ResetPasswordInput = z.infer<typeof resetPasswordSchema>
