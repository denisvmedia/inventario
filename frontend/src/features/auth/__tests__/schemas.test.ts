import { describe, expect, it } from "vitest"

import {
  forgotPasswordSchema,
  loginSchema,
  registerSchema,
  resetPasswordSchema,
} from "@/features/auth/schemas"

describe("auth schemas", () => {
  it("loginSchema requires non-empty email + password", () => {
    expect(loginSchema.safeParse({ email: "", password: "x" }).success).toBe(false)
    expect(loginSchema.safeParse({ email: "a@b.c", password: "" }).success).toBe(false)
    expect(loginSchema.safeParse({ email: "a@b.c", password: "x" }).success).toBe(true)
  })

  it("registerSchema enforces name, email, password, terms acceptance", () => {
    const base = { name: "Alex", email: "a@b.c", password: "secret", acceptTerms: true }
    expect(registerSchema.safeParse(base).success).toBe(true)
    expect(registerSchema.safeParse({ ...base, acceptTerms: false }).success).toBe(false)
    expect(registerSchema.safeParse({ ...base, name: "" }).success).toBe(false)
    expect(registerSchema.safeParse({ ...base, name: "x".repeat(256) }).success).toBe(false)
  })

  it("forgotPasswordSchema only validates non-empty email", () => {
    expect(forgotPasswordSchema.safeParse({ email: "" }).success).toBe(false)
    expect(forgotPasswordSchema.safeParse({ email: "a@b.c" }).success).toBe(true)
  })

  it("resetPasswordSchema requires 8+ chars and matching confirm", () => {
    expect(
      resetPasswordSchema.safeParse({ password: "short", confirmPassword: "short" }).success
    ).toBe(false)
    expect(
      resetPasswordSchema.safeParse({ password: "longenough", confirmPassword: "different" })
        .success
    ).toBe(false)
    const result = resetPasswordSchema.safeParse({
      password: "longenough",
      confirmPassword: "longenough",
    })
    expect(result.success).toBe(true)
  })

  it("resetPasswordSchema attaches mismatch error to confirmPassword", () => {
    const result = resetPasswordSchema.safeParse({
      password: "longenough",
      confirmPassword: "different1",
    })
    if (result.success) throw new Error("expected validation failure")
    const issue = result.error.issues.find((i) => i.path.includes("confirmPassword"))
    expect(issue?.message).toBe("auth:validation.passwordsMismatch")
  })
})
