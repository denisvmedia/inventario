import { describe, expect, it, beforeEach } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { server } from "@/test/server"
import { renderWithProviders } from "@/test/render"
import { GroupRequiredRoute } from "@/components/routing/GroupRequiredRoute"
import { GroupProvider } from "@/features/group/GroupContext"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

function NoGroupStub() {
  const loc = useLocation()
  return (
    <div data-testid="no-group" data-pathname={loc.pathname}>
      no-group
    </div>
  )
}

function Inside() {
  return <div data-testid="inside">inside</div>
}

const buildRoutes = () => (
  <>
    <Route
      element={
        <GroupProvider>
          <GroupRequiredRoute>
            <Inside />
          </GroupRequiredRoute>
        </GroupProvider>
      }
      path="/"
    />
    <Route path="/no-group" element={<NoGroupStub />} />
  </>
)

describe("GroupRequiredRoute", () => {
  it("redirects a user with zero groups to /no-group", async () => {
    server.use(msw.get(api("/groups"), () => HttpResponse.json({ data: [] })))
    renderWithProviders({ initialPath: "/", routes: buildRoutes() })
    await waitFor(() => expect(screen.getByTestId("no-group")).toBeInTheDocument())
    expect(screen.queryByTestId("inside")).toBeNull()
  })

  it("renders the child when the user has at least one group", async () => {
    server.use(
      msw.get(api("/groups"), () =>
        HttpResponse.json({
          data: [
            {
              id: "g1",
              type: "groups",
              attributes: { id: "g1", slug: "household", name: "Household" },
            },
          ],
        })
      )
    )
    renderWithProviders({ initialPath: "/", routes: buildRoutes() })
    await waitFor(() => expect(screen.getByTestId("inside")).toBeInTheDocument())
    expect(screen.queryByTestId("no-group")).toBeNull()
  })

  it("fails open on /groups error — renders the child rather than redirect-loop", async () => {
    server.use(msw.get(api("/groups"), () => HttpResponse.json({ error: "boom" }, { status: 500 })))
    renderWithProviders({ initialPath: "/", routes: buildRoutes() })
    await waitFor(() => expect(screen.getByTestId("inside")).toBeInTheDocument())
  })
})
