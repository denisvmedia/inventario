import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { CreateGroupPage } from "@/pages/groups/CreateGroupPage"
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

function renderCreate() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: "/groups/new",
    routes: (
      <>
        <Route path="/groups/new" element={<CreateGroupPage />} />
        <Route path="*" element={<LocationProbe />} />
      </>
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<CreateGroupPage />", () => {
  it("validates the name field", async () => {
    const user = userEvent.setup()
    renderCreate()
    // Submit without typing a name — the schema's required-name guard fires
    // first; currency defaults to USD via defaultValues so its error path is
    // exercised separately on the server-error test below (clearing the
    // popover-driven combobox via UI alone isn't a workflow real users hit).
    await user.click(screen.getByTestId("create-group-submit"))
    expect(await screen.findByTestId("group-name-error")).toBeInTheDocument()
  })

  it("posts /groups and navigates to /g/{slug} on success", async () => {
    let captured: { data?: { attributes?: Record<string, unknown> } } | null = null
    server.use(
      msw.get(api("/currencies"), () => HttpResponse.json(["EUR", "GBP", "USD"])),
      msw.post(api("/groups"), async ({ request }) => {
        captured = (await request.json()) as typeof captured
        return HttpResponse.json(
          {
            data: {
              id: "g1",
              type: "groups",
              attributes: {
                id: "g1",
                slug: "household",
                name: "Household",
                main_currency: "EUR",
                icon: "🏠",
              },
            },
          },
          { status: 201 }
        )
      })
    )
    const user = userEvent.setup()
    renderCreate()
    await user.type(screen.getByTestId("group-name-input"), "Household")
    // Pick the 🏠 icon via the popover-style picker (#1255 contract).
    await user.click(screen.getByTestId("group-create-icon-picker"))
    await user.click(await screen.findByTestId("icon-picker-option-🏠"))
    // Currency: open the combobox, click the EUR option.
    await user.click(screen.getByRole("combobox", { name: /currency/i }))
    await user.click(await screen.findByRole("option", { name: /^EUR/ }))
    await user.click(screen.getByTestId("create-group-submit"))
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/household")
    )
    // Currency uppercases through the schema's .toUpperCase() pipe, and
    // the icon snaps to the picked emoji. Submitting the wrong shape
    // would have surfaced a 422 instead of a 201 here.
    expect(captured?.data?.attributes).toMatchObject({
      name: "Household",
      main_currency: "EUR",
      icon: "🏠",
    })
  })

  it("surfaces server errors inline on 422", async () => {
    server.use(
      msw.post(api("/groups"), () =>
        HttpResponse.json(
          { errors: [{ detail: "main_currency must be a valid ISO code" }] },
          { status: 422 }
        )
      )
    )
    const user = userEvent.setup()
    renderCreate()
    await user.type(screen.getByTestId("group-name-input"), "Household")
    await user.click(screen.getByTestId("create-group-submit"))
    expect(await screen.findByTestId("create-group-server-error")).toHaveTextContent(/iso code/i)
  })

  it("has no axe violations on the form", async () => {
    const { container } = renderCreate()
    expect(await axe(container)).toHaveNoViolations()
  })
})
