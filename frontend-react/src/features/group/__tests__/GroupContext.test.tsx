import { describe, expect, it, beforeEach } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"

import { server } from "@/test/server"
import { renderWithProviders } from "@/test/render"
import { GroupProvider, useCurrentGroup } from "@/features/group/GroupContext"
import { getCurrentGroupSlug, __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

const groupsResponse = {
  data: [
    { id: "g1", type: "groups", attributes: { id: "g1", slug: "household", name: "Household" } },
    { id: "g2", type: "groups", attributes: { id: "g2", slug: "office", name: "Office" } },
  ],
}

function ProbeGroup() {
  const { currentGroup, groups } = useCurrentGroup()
  return (
    <div
      data-testid="probe"
      data-current={currentGroup?.slug ?? ""}
      data-groups={(groups ?? []).map((g) => g.slug).join(",")}
    >
      probe
    </div>
  )
}

function LocationEcho() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} />
}

describe("GroupContext", () => {
  it("derives currentGroup from the URL :groupSlug", async () => {
    server.use(msw.get(api("/groups"), () => HttpResponse.json(groupsResponse)))
    renderWithProviders({
      initialPath: "/g/household",
      routes: (
        <Route
          path="/g/:groupSlug"
          element={
            <GroupProvider>
              <ProbeGroup />
            </GroupProvider>
          }
        />
      ),
    })
    await waitFor(() =>
      expect(screen.getByTestId("probe").getAttribute("data-current")).toBe("household")
    )
  })

  it("mirrors the URL slug into the http client's group-context slot", async () => {
    server.use(msw.get(api("/groups"), () => HttpResponse.json(groupsResponse)))
    renderWithProviders({
      initialPath: "/g/office",
      routes: (
        <Route
          path="/g/:groupSlug"
          element={
            <GroupProvider>
              <ProbeGroup />
            </GroupProvider>
          }
        />
      ),
    })
    await waitFor(() => expect(getCurrentGroupSlug()).toBe("office"))
  })

  it("currentGroup is null on routes without :groupSlug (e.g. /profile)", async () => {
    server.use(msw.get(api("/groups"), () => HttpResponse.json(groupsResponse)))
    renderWithProviders({
      initialPath: "/profile",
      routes: (
        <Route
          path="/profile"
          element={
            <GroupProvider>
              <ProbeGroup />
            </GroupProvider>
          }
        />
      ),
    })
    await waitFor(() =>
      expect(screen.getByTestId("probe").getAttribute("data-groups")).toBe("household,office")
    )
    expect(screen.getByTestId("probe").getAttribute("data-current")).toBe("")
    expect(getCurrentGroupSlug()).toBeNull()
  })

  it("redirects an unknown :groupSlug to the first known group", async () => {
    server.use(msw.get(api("/groups"), () => HttpResponse.json(groupsResponse)))
    renderWithProviders({
      initialPath: "/g/wrong-slug",
      routes: (
        <>
          <Route
            path="/g/:groupSlug"
            element={
              <GroupProvider>
                <ProbeGroup />
                <LocationEcho />
              </GroupProvider>
            }
          />
        </>
      ),
    })
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/household")
    )
  })

  it("redirects an unknown :groupSlug to /no-group when the user has zero groups", async () => {
    server.use(msw.get(api("/groups"), () => HttpResponse.json({ data: [] })))
    renderWithProviders({
      initialPath: "/g/wrong-slug",
      routes: (
        <>
          <Route
            path="/g/:groupSlug"
            element={
              <GroupProvider>
                <ProbeGroup />
                <LocationEcho />
              </GroupProvider>
            }
          />
          <Route path="/no-group" element={<LocationEcho />} />
        </>
      ),
    })
    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/no-group")
    )
  })
})
