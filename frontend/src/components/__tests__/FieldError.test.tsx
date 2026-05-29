import { describe, expect, it } from "vitest"
import { screen } from "@testing-library/react"
import { axe } from "jest-axe"

import { FieldError } from "@/components/FieldError"
import { i18next } from "@/i18n"
import { renderWithProviders } from "@/test/render"

describe("<FieldError />", () => {
  it("renders nothing when there is no message", () => {
    renderWithProviders({ children: <FieldError message={undefined} testId="fe" /> })
    expect(screen.queryByTestId("fe")).not.toBeInTheDocument()
  })

  it("renders nothing for an empty-string message", () => {
    renderWithProviders({ children: <FieldError message="" testId="fe" /> })
    expect(screen.queryByTestId("fe")).not.toBeInTheDocument()
  })

  it("resolves the i18n key and carries the field-error hook + id + testId", () => {
    renderWithProviders({
      children: (
        <FieldError message="auth:validation.emailRequired" id="email-error" testId="email-error" />
      ),
    })
    const el = screen.getByTestId("email-error")
    // Baked-in class is the e2e selector hook (e2e/tests/profile.spec.ts).
    expect(el).toHaveClass("field-error")
    expect(el).toHaveClass("text-destructive")
    expect(el).toHaveAttribute("id", "email-error")
    // The component owns the translation — the key resolves to real copy.
    expect(el).toHaveTextContent(i18next.t("auth:validation.emailRequired"))
    expect(el.textContent).toBe("Email is required")
  })

  it("merges a caller className with the base classes", () => {
    renderWithProviders({
      children: <FieldError message="auth:validation.emailRequired" testId="e" className="mt-2" />,
    })
    const el = screen.getByTestId("e")
    expect(el).toHaveClass("mt-2")
    expect(el).toHaveClass("field-error")
  })

  it("has no axe violations", async () => {
    const { container } = renderWithProviders({
      children: <FieldError message="auth:validation.emailRequired" />,
    })
    expect(await axe(container)).toHaveNoViolations()
  })
})
