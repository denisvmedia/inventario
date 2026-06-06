import { beforeEach, describe, expect, it } from "vitest"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import {
  AnonymousCommodityDialog,
  ANON_DRAFT_KEY,
} from "@/components/items/AnonymousCommodityDialog"
import { clearPendingFirstItem, peekPendingFirstItem } from "@/features/auth/firstItemHandoff"
import { renderWithProviders } from "@/test/render"
import { __resetGroupContextForTests } from "@/lib/group-context"

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} data-search={loc.search} />
}

function renderDialog() {
  return renderWithProviders({
    initialPath: "/",
    routes: (
      <>
        {/* aiScanEnabled defaults false → opens on Basics (manual). */}
        <Route path="/" element={<AnonymousCommodityDialog open onOpenChange={() => {}} />} />
        <Route path="*" element={<LocationProbe />} />
      </>
    ),
  })
}

beforeEach(() => {
  window.localStorage.clear()
  clearPendingFirstItem()
  __resetGroupContextForTests()
})

describe("<AnonymousCommodityDialog /> save-as-draft hand-off (#1988)", () => {
  it("sets the pending marker and routes to register when 'Save as draft' is chosen", async () => {
    const user = userEvent.setup()
    renderDialog()
    // Enter something so the dialog is dirty, then try to dismiss it.
    await user.type(await screen.findByLabelText(/^Name$/i), "Camera")
    await user.click(screen.getByTestId("commodity-form-cancel"))
    // The dismiss-confirm appears; choose "Save as draft".
    await user.click(await screen.findByTestId("commodity-form-close-confirm-save"))
    // Anonymous save-as-draft hands off to register (the fill is new-user
    // onboarding, not back to the landing page): marker set + redirect to
    // /register?redirect=/welcome.
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/register")
    )
    expect(screen.getByTestId("loc").getAttribute("data-search")).toContain("redirect=%2Fwelcome")
    expect(peekPendingFirstItem()?.draftKey).toBe(ANON_DRAFT_KEY)
  })

  it("persists the typed draft so it can be replayed after sign-in", async () => {
    const user = userEvent.setup()
    renderDialog()
    await user.type(await screen.findByLabelText(/^Name$/i), "Camera")
    await user.click(screen.getByTestId("commodity-form-cancel"))
    await user.click(await screen.findByTestId("commodity-form-close-confirm-save"))
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/register")
    )
    const draft = JSON.parse(window.localStorage.getItem(ANON_DRAFT_KEY) ?? "{}")
    expect(draft.name).toBe("Camera")
  })
})
