import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { LandingPage } from "@/pages/LandingPage"
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

function renderLanding() {
  return renderWithProviders({
    initialPath: "/",
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

  it("hides the Add card when public_scan is off", async () => {
    mockFlags(false)
    renderLanding()
    await screen.findByTestId("landing-page")
    // Flag query resolves async; assert the Add card never appears.
    await waitFor(() => {
      expect(screen.getByTestId("landing-browse")).toBeInTheDocument()
    })
    expect(screen.queryByTestId("landing-add-item")).not.toBeInTheDocument()
  })

  it("shows the Add card when public_scan is on", async () => {
    mockFlags(true)
    renderLanding()
    await screen.findByTestId("landing-page")
    expect(await screen.findByTestId("landing-add-item")).toBeInTheDocument()
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
})
