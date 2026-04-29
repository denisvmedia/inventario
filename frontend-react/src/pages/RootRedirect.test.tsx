import { describe, expect, it, beforeEach } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { server } from "@/test/server"
import { renderWithProviders } from "@/test/render"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { RootRedirect } from "@/pages/RootRedirect"
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

const groupsPayload = {
  data: [
    { id: "g1", type: "groups", attributes: { id: "g1", slug: "household", name: "Household" } },
    { id: "g2", type: "groups", attributes: { id: "g2", slug: "office", name: "Office" } },
  ],
}

function LocationEcho() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} />
}

function renderRoot(initial = "/") {
  return renderWithProviders({
    initialPath: initial,
    routes: (
      <>
        <Route
          path="/"
          element={
            <AuthProvider>
              <GroupProvider>
                <RootRedirect />
              </GroupProvider>
            </AuthProvider>
          }
        />
        <Route
          path="/g/:groupSlug"
          element={
            <AuthProvider>
              <GroupProvider>
                <LocationEcho />
              </GroupProvider>
            </AuthProvider>
          }
        />
        <Route path="/no-group" element={<LocationEcho />} />
      </>
    ),
  })
}

describe("RootRedirect", () => {
  it("redirects to /no-group when the user has zero groups", async () => {
    server.use(
      msw.get(api("/auth/me"), () => HttpResponse.json({ id: "u1", email: "x@y.z", name: "X" })),
      msw.get(api("/groups"), () => HttpResponse.json({ data: [] }))
    )
    renderRoot("/")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/no-group")
    )
  })

  it("redirects to the first group when the user has no default_group_id", async () => {
    server.use(
      msw.get(api("/auth/me"), () => HttpResponse.json({ id: "u1", email: "x@y.z", name: "X" })),
      msw.get(api("/groups"), () => HttpResponse.json(groupsPayload))
    )
    renderRoot("/")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/household")
    )
  })

  it("honors user.default_group_id when the user is a member of that group", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "x@y.z", name: "X", default_group_id: "g2" })
      ),
      msw.get(api("/groups"), () => HttpResponse.json(groupsPayload))
    )
    renderRoot("/")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/office")
    )
  })

  it("falls back to first group when default_group_id points at a group the user is not in", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "x@y.z", name: "X", default_group_id: "g999" })
      ),
      msw.get(api("/groups"), () => HttpResponse.json(groupsPayload))
    )
    renderRoot("/")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/household")
    )
  })

  it("redirects to /no-group on a /groups error rather than a half-broken landing", async () => {
    server.use(
      msw.get(api("/auth/me"), () => HttpResponse.json({ id: "u1", email: "x@y.z", name: "X" })),
      msw.get(api("/groups"), () => HttpResponse.json({ error: "boom" }, { status: 500 }))
    )
    renderRoot("/")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/no-group")
    )
  })
})
