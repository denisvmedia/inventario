import { z } from 'zod'

/**
 * Schema for the login form. Mirrors the legacy non-empty-trim
 * preconditions of `LoginForm.vue` (#1326 PR 1.6) so the e2e suite that
 * checks "Sign In" enables only when both fields have content keeps
 * matching. Email format is validated server-side as well — keeping the
 * client-side rule loose (`min(1)`) avoids surfacing validation chrome
 * for a value the user has not finished typing.
 */
export const loginFormSchema = z.object({
  email: z
    .string({ required_error: 'Email is required' })
    .min(1, 'Email is required'),
  password: z
    .string({ required_error: 'Password is required' })
    .min(1, 'Password is required'),
})

export type LoginFormInput = z.infer<typeof loginFormSchema>
