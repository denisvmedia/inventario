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

// Edits a logged-in user's profile from /profile/edit. Email is omitted
// because the backend ignores it on PUT /auth/me. default_group_id is
// validated against the user's actual memberships at submit time, not
// here in the schema (zod doesn't know which groups the user belongs
// to); empty string is mapped to null in the page handler.
export const profileEditSchema = z.object({
  // Trim before validating: a value like "   " would otherwise pass the
  // .min(1) check, then values.name.trim() would be empty on submit and
  // the server would reject it. Trimming inside the schema keeps the
  // client-side check aligned with what's actually sent on the wire.
  name: z
    .string()
    .trim()
    .min(1, "auth:validation.nameRequired")
    .max(255, "auth:validation.nameTooLong"),
  defaultGroupId: z.string(),
})
export type ProfileEditInput = z.infer<typeof profileEditSchema>

// Cross-field rule on /profile/edit's password card — current required,
// new required and ≥8 chars (matching the reset-password rule), confirm
// must match new, and new must differ from current to surface the
// "this is the same password you already had" mistake without a server
// round-trip.
export const changePasswordSchema = z
  .object({
    currentPassword: z.string().min(1, "auth:validation.passwordRequired"),
    newPassword: z.string().min(8, "auth:validation.passwordMinLength"),
    confirmPassword: z.string().min(1, "auth:validation.passwordConfirmRequired"),
  })
  .superRefine((value, ctx) => {
    if (value.newPassword !== value.confirmPassword) {
      ctx.addIssue({
        code: "custom",
        path: ["confirmPassword"],
        message: "auth:validation.passwordsMismatch",
      })
    }
    if (value.newPassword === value.currentPassword) {
      ctx.addIssue({
        code: "custom",
        path: ["newPassword"],
        message: "auth:validation.passwordSameAsCurrent",
      })
    }
  })
export type ChangePasswordInput = z.infer<typeof changePasswordSchema>

// Set-password form (#1394) for OAuth-only users whose row was provisioned
// with an empty password_hash. Same shape as changePasswordSchema minus the
// "current" verification — the BE's /auth/change-password handler skips the
// current-password check when the user has no hash on file.
export const setPasswordSchema = z
  .object({
    newPassword: z.string().min(8, "auth:validation.passwordMinLength"),
    confirmPassword: z.string().min(1, "auth:validation.passwordConfirmRequired"),
  })
  .superRefine((value, ctx) => {
    if (value.newPassword !== value.confirmPassword) {
      ctx.addIssue({
        code: "custom",
        path: ["confirmPassword"],
        message: "auth:validation.passwordsMismatch",
      })
    }
  })
export type SetPasswordInput = z.infer<typeof setPasswordSchema>

// Self-service account deletion (#2147) — /settings → Account danger zone.
// Mirrors deleteGroupSchema's split of concerns: the schema only enforces
// the gating "non-empty" rules, while the typed-confirmation match
// (confirmEmail must equal the signed-in user's address) is checked in the
// dialog handler since the schema doesn't know the user's email.
//
// The password rule is conditional on whether the user has an in-app
// password hash. Password-backed users must re-authenticate (the BE
// re-verifies via user.CheckPassword), so the field is required. OAuth-only
// users (#1394) have an empty hash and the BE skips re-auth — for them the
// access token already proves identity, so requiring a password here would
// lock them out of self-deletion entirely (there's nothing to type). The
// dialog builds its resolver from the right variant via
// `makeDeleteAccountSchema(hasPassword)` and the password input is hidden in
// the no-password case. The inferred input type always carries `password`
// (an empty string in the OAuth case) so the form shape stays stable.
function makeDeleteAccountSchema(hasPassword: boolean) {
  return z.object({
    confirmEmail: z.string().trim().min(1, "auth:validation.confirmEmailRequired"),
    password: hasPassword ? z.string().min(1, "auth:validation.passwordRequired") : z.string(),
  })
}

// Default (has-password) variant, kept as a named export for the inferred
// type and for any consumer that doesn't need the conditional shape.
export const deleteAccountSchema = makeDeleteAccountSchema(true)
export type DeleteAccountInput = z.infer<typeof deleteAccountSchema>

export { makeDeleteAccountSchema }
