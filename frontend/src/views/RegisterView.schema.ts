import { z } from 'zod'

/**
 * Schema for the registration form. The legacy view (#1326 PR 1.6)
 * gated submit on `name`/`email`/`password` all being non-empty; the
 * e2e test `enables submit button only when all fields are filled`
 * (registration.spec.ts) relies on that semantic. Password length and
 * complexity are validated server-side — the placeholder "At least 8
 * characters" remains as an inline hint, but the client-side rule is
 * intentionally loose (`min(1)`) so that the e2e test
 * `shows error when password is too short / weak` can submit a short
 * password and see the server's `.error-message`. Surfacing a length
 * rule client-side would short-circuit the request before the server
 * had a chance to respond.
 */
export const registerFormSchema = z.object({
  name: z
    .string({ required_error: 'Full name is required' })
    .min(1, 'Full name is required')
    .max(255, 'Name is too long'),
  email: z
    .string({ required_error: 'Email is required' })
    .min(1, 'Email is required'),
  password: z
    .string({ required_error: 'Password is required' })
    .min(1, 'Password is required'),
})

export type RegisterFormInput = z.infer<typeof registerFormSchema>
