import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { NoGroupPage } from "@/pages/NoGroupPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} />
}

function renderNoGroup() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: "/no-group",
    routes: (
      <>
        <Route
          path="/no-group"
          element={
            <AuthProvider>
              <GroupProvider>
                <NoGroupPage />
              </GroupProvider>
            </AuthProvider>
          }
        />
        <Route path="*" element={<LocationProbe />} />
      </>
    ),
  })
}

const baseHandlers = [
  msw.get(api("/auth/me"), () =>
    HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
  ),
  msw.get(api("/groups"), () => HttpResponse.json({ data: [] })),
]

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<NoGroupPage />", () => {
  it("renders the welcome copy and the create-group CTA", async () => {
    server.use(...baseHandlers)
    renderNoGroup()
    expect(await screen.findByTestId("no-group-page")).toBeInTheDocument()
    expect(screen.getByTestId("no-group-create-button")).toBeInTheDocument()
    // The inline form is collapsed until the user clicks the CTA.
    expect(screen.queryByTestId("no-group-name-input")).not.toBeInTheDocument()
  })

  it("clicking the CTA reveals the inline create-group form", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderNoGroup()
    await user.click(await screen.findByTestId("no-group-create-button"))
    expect(await screen.findByTestId("no-group-name-input")).toBeInTheDocument()
    expect(screen.getByTestId("no-group-submit")).toBeInTheDocument()
  })

  it("submits the new group and navigates to /g/<slug>", async () => {
    let captured: { data?: { attributes?: { name?: string } } } | null = null
    server.use(
      ...baseHandlers,
      msw.post(api("/groups"), async ({ request }) => {
        captured = (await request.json()) as typeof captured
        const name = captured?.data?.attributes?.name ?? ""
        return HttpResponse.json(
          {
            data: {
              id: "g-new",
              type: "groups",
              attributes: {
                id: "g-new",
                slug: "household",
                name,
                main_currency: "USD",
                icon: "",
              },
            },
          },
          { status: 201 }
        )
      })
    )
    const user = userEvent.setup()
    renderNoGroup()
    await user.click(await screen.findByTestId("no-group-create-button"))
    await user.type(await screen.findByTestId("no-group-name-input"), "Household")
    await user.click(screen.getByTestId("no-group-submit"))
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/household")
    )
    expect(captured?.data?.attributes?.name).toBe("Household")
  })

  it("surfaces an inline server error on POST failure", async () => {
    server.use(
      ...baseHandlers,
      msw.post(api("/groups"), () =>
        HttpResponse.json({ errors: [{ detail: "Database is down" }] }, { status: 500 })
      )
    )
    const user = userEvent.setup()
    renderNoGroup()
    await user.click(await screen.findByTestId("no-group-create-button"))
    await user.type(await screen.findByTestId("no-group-name-input"), "Household")
    await user.click(screen.getByTestId("no-group-submit"))
    expect(await screen.findByTestId("no-group-server-error")).toBeInTheDocument()
  })

  it("the cancel button collapses the form back to the CTA", async () => {
    server.use(...baseHandlers)
    const user = userEvent.setup()
    renderNoGroup()
    await user.click(await screen.findByTestId("no-group-create-button"))
    expect(await screen.findByTestId("no-group-name-input")).toBeInTheDocument()
    await user.click(screen.getByRole("button", { name: /cancel/i }))
    expect(screen.queryByTestId("no-group-name-input")).not.toBeInTheDocument()
    expect(screen.getByTestId("no-group-create-button")).toBeInTheDocument()
  })

  it("sign-out triggers the logout flow", async () => {
    let logoutCalls = 0
    server.use(
      ...baseHandlers,
      msw.post(api("/auth/logout"), () => {
        logoutCalls++
        return new HttpResponse(null, { status: 204 })
      })
    )
    const user = userEvent.setup()
    renderNoGroup()
    await user.click(await screen.findByTestId("no-group-signout"))
    await waitFor(() => expect(logoutCalls).toBe(1))
  })
})
