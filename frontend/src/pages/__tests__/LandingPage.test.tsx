import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { LandingPage } from "@/pages/LandingPage"
import { ANON_DRAFT_KEY } from "@/components/items/AnonymousCommodityDialog"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} data-search={loc.search} />
}

// publicScan toggles the `public_scan` deployment flag the landing CTA is
// gated on. Both other flags stay off — they don't affect this surface.
function mockFlags(publicScan: boolean) {
  server.use(
    msw.get(api("/feature-flags"), () =>
      HttpResponse.json({
        currency_migration: false,
        magic_link_login: false,
        public_scan: publicScan,
      })
    )
  )
}

function renderLanding(initialPath = "/") {
  return renderWithProviders({
    initialPath,
    routes: (
      <>
        <Route path="/" element={<LandingPage />} />
        <Route path="*" element={<LocationProbe />} />
      </>
    ),
  })
}

beforeEach(() => {
  window.localStorage.clear()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<LandingPage />", () => {
  it("renders the hero and the Browse card", async () => {
    mockFlags(false)
    renderLanding()
    expect(await screen.findByTestId("landing-page")).toBeInTheDocument()
    expect(screen.getByText("Everything you own, organized")).toBeInTheDocument()
    expect(screen.getByTestId("landing-browse")).toBeInTheDocument()
    expect(screen.getByTestId("landing-login-link")).toBeInTheDocument()
  })

  it("shows the Add card (manual copy) when public_scan is off", async () => {
    mockFlags(false)
    renderLanding()
    await screen.findByTestId("landing-page")
    // Adding an item is the primary CTA and must always be present —
    // public_scan off only drops the AI accelerator, not the card.
    const add = await screen.findByTestId("landing-add-item")
    expect(add).toBeInTheDocument()
    // Copy reflects manual entry rather than promising AI fill-in.
    expect(add).toHaveTextContent("Add your first item in seconds")
    expect(add).not.toHaveTextContent("let AI fill in")
  })

  it("shows the Add card (AI copy) when public_scan is on", async () => {
    mockFlags(true)
    renderLanding()
    await screen.findByTestId("landing-page")
    const add = await screen.findByTestId("landing-add-item")
    expect(add).toBeInTheDocument()
    expect(add).toHaveTextContent("let AI fill in the details")
  })

  it("Browse card navigates to /login?redirect=/", async () => {
    mockFlags(false)
    const user = userEvent.setup()
    renderLanding()
    await user.click(await screen.findByTestId("landing-browse"))
    await waitFor(() => {
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/login")
    })
    expect(screen.getByTestId("loc").getAttribute("data-search")).toBe("?redirect=%2F")
  })

  it("login link navigates to /login?redirect=/", async () => {
    mockFlags(false)
    const user = userEvent.setup()
    renderLanding()
    await user.click(await screen.findByTestId("landing-login-link"))
    await waitFor(() => {
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/login")
    })
    expect(screen.getByTestId("loc").getAttribute("data-search")).toBe("?redirect=%2F")
  })

  it("hides the resume badge when there is no in-progress draft", async () => {
    mockFlags(false)
    renderLanding()
    await screen.findByTestId("landing-page")
    expect(screen.queryByTestId("resume-first-item-pill")).not.toBeInTheDocument()
  })

  it("ignores a content-less draft (defaults only) for the resume badge", async () => {
    mockFlags(false)
    // The dialog auto-saves defaults on open; an identity-field-less draft
    // must not surface the "continue" affordance.
    window.localStorage.setItem(ANON_DRAFT_KEY, JSON.stringify({ count: "1", draft: true }))
    renderLanding()
    await screen.findByTestId("landing-page")
    expect(screen.queryByTestId("resume-first-item-pill")).not.toBeInTheDocument()
  })

  it("shows the resume badge for a draft with content and reopens the dialog", async () => {
    mockFlags(false)
    window.localStorage.setItem(ANON_DRAFT_KEY, JSON.stringify({ name: "Camera" }))
    const user = userEvent.setup()
    renderLanding()
    const badge = await screen.findByTestId("resume-first-item-pill")
    await user.click(badge)
    // public_scan off ⇒ the dialog opens straight on the Basics step
    // (the footer Next button is only present off the AI surface).
    expect(await screen.findByTestId("commodity-form-next")).toBeInTheDocument()
  })

  it("auto-opens the dialog when arriving with ?addFirstItem=1 (resume from auth)", async () => {
    mockFlags(false)
    window.localStorage.setItem(ANON_DRAFT_KEY, JSON.stringify({ name: "Camera" }))
    renderLanding("/?addFirstItem=1")
    // Dialog opens directly on the form to continue editing (no AI offer),
    // signalled by the footer Next button.
    expect(await screen.findByTestId("commodity-form-next")).toBeInTheDocument()
    // ...and the floating pill is hidden while the dialog is open.
    expect(screen.queryByTestId("resume-first-item-pill")).not.toBeInTheDocument()
  })
})
