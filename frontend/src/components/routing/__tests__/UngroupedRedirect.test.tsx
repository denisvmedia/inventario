import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { UngroupedRedirect } from "@/components/routing/UngroupedRedirect"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname + loc.search + loc.hash} />
}

function renderAt(path: string) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: path,
    routes: (
      <>
        <Route
          element={
            <AuthProvider>
              <GroupProvider>
                <UngroupedRedirect />
              </GroupProvider>
            </AuthProvider>
          }
          path="/files"
        />
        <Route
          element={
            <AuthProvider>
              <GroupProvider>
                <UngroupedRedirect />
              </GroupProvider>
            </AuthProvider>
          }
          path="/locations/:id"
        />
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

describe("<UngroupedRedirect />", () => {
  it("redirects 0-group users to /no-group", async () => {
    server.use(
      msw.get(api("/auth/me"), () => HttpResponse.json({ id: "u1", email: "alex@example.com" })),
      msw.get(api("/groups"), () => HttpResponse.json({ data: [] }))
    )
    renderAt("/files")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/no-group")
    )
  })

  it("rewrites a bare /files to /g/<active-slug>/files when groups exist", async () => {
    server.use(
      msw.get(api("/auth/me"), () => HttpResponse.json({ id: "u1", email: "alex@example.com" })),
      msw.get(api("/groups"), () =>
        HttpResponse.json({
          data: [
            {
              id: "g1",
              type: "groups",
              attributes: {
                id: "g1",
                slug: "household",
                name: "Household",
                main_currency: "USD",
              },
            },
          ],
        })
      )
    )
    renderAt("/files")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/household/files")
    )
  })

  it("preserves nested paths and search/hash on rewrite", async () => {
    server.use(
      msw.get(api("/auth/me"), () => HttpResponse.json({ id: "u1", email: "alex@example.com" })),
      msw.get(api("/groups"), () =>
        HttpResponse.json({
          data: [
            {
              id: "g1",
              type: "groups",
              attributes: {
                id: "g1",
                slug: "household",
                name: "Household",
                main_currency: "USD",
              },
            },
          ],
        })
      )
    )
    renderAt("/locations/abc-123?foo=bar#anchor")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe(
        "/g/household/locations/abc-123?foo=bar#anchor"
      )
    )
  })

  it("prefers the user's default_group_id when it resolves to a slug", async () => {
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({ id: "u1", email: "alex@example.com", default_group_id: "g2" })
      ),
      msw.get(api("/groups"), () =>
        HttpResponse.json({
          data: [
            {
              id: "g1",
              type: "groups",
              attributes: {
                id: "g1",
                slug: "household",
                name: "Household",
                main_currency: "USD",
              },
            },
            {
              id: "g2",
              type: "groups",
              attributes: {
                id: "g2",
                slug: "office",
                name: "Office",
                main_currency: "USD",
              },
            },
          ],
        })
      )
    )
    renderAt("/files")
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/office/files")
    )
  })
})
