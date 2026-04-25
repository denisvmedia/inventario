import { z } from 'zod'

/**
 * Schema for the registration form. The legacy view (#1326 PR 1.6)
 * gated submit on `name`/`email`/`password` all being non-empty; the
 * e2e test `enables submit button only when all fields are filled`
 * (registration.spec.ts) relies on that semantic. The 8-character
 * minimum on `password` mirrors the placeholder copy ("At least 8
 * characters") and matches the password policy enforced server-side.
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
    .min(8, 'Password must be at least 8 characters'),
})

export type RegisterFormInput = z.infer<typeof registerFormSchema>
