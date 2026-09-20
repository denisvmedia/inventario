import { describe, expect, it } from "vitest"
import { screen } from "@testing-library/react"

import PrivacyPage from "@/pages/legal/PrivacyPage"
import TermsPage from "@/pages/legal/TermsPage"
import { renderWithProviders } from "@/test/render"

// Registration makes accepting these a condition of signing up, so they have
// to render for someone who has no account yet (#2148).
describe("legal pages", () => {
  it("renders the privacy policy without an account", () => {
    renderWithProviders({ children: <PrivacyPage /> })

    expect(screen.getByRole("heading", { level: 1, name: "Privacy Policy" })).toBeInTheDocument()
    expect(screen.getByText(/What is stored about you/)).toBeInTheDocument()
  })

  it("renders the terms without an account", () => {
    renderWithProviders({ children: <TermsPage /> })

    expect(screen.getByRole("heading", { level: 1, name: "Terms of Service" })).toBeInTheDocument()
    expect(screen.getAllByText(/MIT License/).length).toBeGreaterThan(0)
  })
})
