import { beforeAll, beforeEach, describe, expect, it } from "vitest"
import { render, screen } from "@testing-library/react"
import { Route } from "react-router-dom"

import { WarrantyBadge } from "@/components/warranty/WarrantyBadge"
import { GroupProvider } from "@/features/group/GroupContext"
import type { GroupSection } from "@/features/group/sections"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { WhenGroupLoaded } from "@/test/group-loaded"
import { groupHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

describe("<WarrantyBadge />", () => {
  it("renders the i18n label for the precomputed status", () => {
    render(<WarrantyBadge status="active" data-testid="badge" />)
    const badge = screen.getByTestId("badge")
    expect(badge).toHaveTextContent("Active")
    expect(badge).toHaveAttribute("data-status", "active")
    expect(badge).toHaveClass("text-status-active")
  })

  it("derives the status from a raw commodity slice when `source` is given", () => {
    render(<WarrantyBadge source={{ warranty_expires_at: "1999-01-01" }} data-testid="badge" />)
    const badge = screen.getByTestId("badge")
    expect(badge).toHaveAttribute("data-status", "expired")
    expect(badge).toHaveTextContent("Expired")
  })

  it("renders the `none` bucket when the field is unset", () => {
    render(<WarrantyBadge source={{}} data-testid="badge" />)
    expect(screen.getByTestId("badge")).toHaveAttribute("data-status", "none")
    expect(screen.getByTestId("badge")).toHaveTextContent("No Warranty")
  })

  it("treats 1970-01-01 as expired, not `none` (Date.parse anchor regression)", () => {
    // parseWarrantyDate returns 0 for the epoch — a `ms ? ... : "none"`
    // truthy check would misclassify the date as `none`. Pin the
    // explicit-null behaviour so a future refactor doesn't reintroduce
    // the truthy ternary.
    render(<WarrantyBadge source={{ warranty_expires_at: "1970-01-01" }} data-testid="badge" />)
    expect(screen.getByTestId("badge")).toHaveAttribute("data-status", "expired")
  })

  it("hides the leading icon when `showIcon` is false", () => {
    const { container } = render(<WarrantyBadge status="active" showIcon={false} />)
    expect(container.querySelector("svg")).toBeNull()
  })
})

describe("<WarrantyBadge /> inside a group (#1607)", () => {
  beforeEach(() => {
    clearAuth()
    __resetGroupContextForTests()
    __resetHttpForTests()
    setAccessToken("good-token")
  })

  function renderInGroup(hidden: GroupSection[], expiresAt?: string) {
    server.use(
      ...groupHandlers.list([
        { id: "g1", slug: "household", name: "Household", hidden_sections: hidden },
      ])
    )
    renderWithProviders({
      initialPath: "/g/household",
      routes: (
        <Route
          path="/g/:groupSlug"
          element={
            <GroupProvider>
              <WhenGroupLoaded>
                <p data-testid="group-loaded" />
                <WarrantyBadge source={{ warranty_expires_at: expiresAt }} data-testid="badge" />
              </WhenGroupLoaded>
            </GroupProvider>
          }
        />
      ),
    })
  }

  it("says No Warranty while the group shows warranties", async () => {
    renderInGroup(["loans"])
    expect(await screen.findByTestId("badge")).toHaveAttribute("data-status", "none")
  })

  it("renders nothing for an item without a warranty while the group hides them", async () => {
    renderInGroup(["warranties"])
    await screen.findByTestId("group-loaded")
    expect(screen.queryByTestId("badge")).not.toBeInTheDocument()
  })

  it("still shows a warranty the item has while the group hides them", async () => {
    renderInGroup(["warranties"], "1999-01-01")
    expect(await screen.findByTestId("badge")).toHaveAttribute("data-status", "expired")
  })
})
