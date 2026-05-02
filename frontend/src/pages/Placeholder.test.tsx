import { describe, expect, it } from "vitest"
import { screen } from "@testing-library/react"

import { PlaceholderPage } from "@/pages/Placeholder"
import { Route, renderWithProviders } from "@/test/render"

describe("PlaceholderPage", () => {
  it("renders the resolved stub title and the 'Coming soon' line", () => {
    renderWithProviders({
      initialPath: "/",
      routes: <Route path="/" element={<PlaceholderPage titleKey="login" testId="page-test" />} />,
    })
    expect(screen.getByRole("heading", { name: /sign in/i, level: 1 })).toBeInTheDocument()
    expect(screen.getByText(/coming soon/i)).toBeInTheDocument()
  })

  it("emits the trackedBy footer when the prop is set", () => {
    renderWithProviders({
      initialPath: "/",
      routes: (
        <Route
          path="/"
          element={<PlaceholderPage titleKey="register" testId="page-test" trackedBy="#1407" />}
        />
      ),
    })
    expect(screen.getByText(/Tracked by #1407/i)).toBeInTheDocument()
  })

  it("omits the trackedBy footer when no prop is passed", () => {
    renderWithProviders({
      initialPath: "/",
      routes: <Route path="/" element={<PlaceholderPage titleKey="login" testId="page-test" />} />,
    })
    expect(screen.queryByText(/Tracked by/)).toBeNull()
  })
})
