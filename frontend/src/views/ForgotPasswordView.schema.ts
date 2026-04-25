import { z } from 'zod'

/**
 * Schema for the forgot-password form (#1326 PR 1.6). One field; the
 * server returns a generic success regardless of whether the email is
 * known (enumeration prevention), so we only need to ensure something
 * was actually entered before letting the user submit.
 */
export const forgotPasswordFormSchema = z.object({
  email: z
    .string({ required_error: 'Email is required' })
    .min(1, 'Email is required'),
})

export type ForgotPasswordFormInput = z.infer<typeof forgotPasswordFormSchema>
