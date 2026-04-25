/**
 * Schema for the reset-password form (#1326 PR 1.6). Both fields must be
 * filled with a value at least 8 characters long, and the two values
 * must match. The cross-field check is implemented as a `superRefine`
 * so the error is attached to `confirmPassword` — that is where the
 * legacy view surfaced the "Passwords do not match" notice.
 */
import { z } from 'zod'

export const resetPasswordFormSchema = z
  .object({
    password: z
      .string({ required_error: 'Password is required' })
      .min(8, 'Password must be at least 8 characters'),
    confirmPassword: z
      .string({ required_error: 'Please confirm your password' })
      .min(1, 'Please confirm your password'),
  })
  .superRefine((value, ctx) => {
    if (value.password !== value.confirmPassword) {
      ctx.addIssue({
        code: z.ZodIssueCode.custom,
        path: ['confirmPassword'],
        message: 'Passwords do not match',
      })
    }
  })

export type ResetPasswordFormInput = z.infer<typeof resetPasswordFormSchema>
