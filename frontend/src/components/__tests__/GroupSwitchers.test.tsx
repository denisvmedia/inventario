import type { ReactNode } from "react"
import { beforeEach, describe, expect, it } from "vitest"
import { http, HttpResponse } from "msw"
import { Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { GroupRoleCluster } from "@/components/GroupRoleCluster"
import { GroupSelector } from "@/components/GroupSelector"
import { SidebarProvider } from "@/components/ui/sidebar"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { apiUrl, authHandlers, groupHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

// Both group switchers, the sidebar GroupSelector and the top bar
// GroupRoleCluster, go through useSwitchGroup (#2910): a switch opens
// /g/<slug> and saves the group as default_group_id with one PUT /auth/me.

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} />
}

function ownerOf(groupId: string) {
  return [
    {
      id: `m-${groupId}`,
      type: "memberships",
      attributes: { group_id: groupId, member_user_id: "u1", role: "owner" },
    },
  ]
}

type ProfilePut = { name: string; default_group_id: string }

// captureProfilePuts records every PUT /auth/me body.
function captureProfilePuts(): ProfilePut[] {
  const puts: ProfilePut[] = []
  server.use(
    http.put(apiUrl("/auth/me"), async ({ request }) => {
      const body = (await request.json()) as ProfilePut
      puts.push(body)
      return HttpResponse.json({ id: "u1", email: "denis@example.com", ...body })
    })
  )
  return puts
}

function renderAt(path: string, switcher: ReactNode) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: path,
    routes: (
      <Route
        path="/g/:groupSlug"
        element={
          <AuthProvider>
            <GroupProvider>
              <SidebarProvider>
                {switcher}
                <LocationProbe />
              </SidebarProvider>
            </GroupProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  server.use(
    ...authHandlers.signedIn(),
    ...groupHandlers.list(),
    http.get(apiUrl("/groups/g1/members"), () => HttpResponse.json({ data: ownerOf("g1") })),
    http.get(apiUrl("/groups/g2/members"), () => HttpResponse.json({ data: ownerOf("g2") }))
  )
})

describe("group switchers (#2910)", () => {
  it("the sidebar selector opens the group and saves it as the default group", async () => {
    const puts = captureProfilePuts()
    const user = userEvent.setup()
    renderAt("/g/household", <GroupSelector />)

    await user.click(await screen.findByRole("button", { name: "Switch group" }))
    await user.click(await screen.findByRole("menuitem", { name: /Office/ }))

    await waitFor(() =>
      expect(screen.getByTestId("loc")).toHaveAttribute("data-pathname", "/g/office")
    )
    await waitFor(() => expect(puts).toEqual([{ name: "Denis", default_group_id: "g2" }]))
  })

  it("the top bar switcher saves the same way", async () => {
    const puts = captureProfilePuts()
    const user = userEvent.setup()
    renderAt("/g/household", <GroupRoleCluster />)

    await user.click(await screen.findByRole("button", { name: "Switch group" }))
    await user.click(await screen.findByRole("menuitem", { name: /Office/ }))

    await waitFor(() =>
      expect(screen.getByTestId("loc")).toHaveAttribute("data-pathname", "/g/office")
    )
    await waitFor(() => expect(puts).toEqual([{ name: "Denis", default_group_id: "g2" }]))
  })

  it("sends the save when the switcher unmounts before the delay ends", async () => {
    // The mobile sidebar is a sheet that closes, and unmounts the selector,
    // as soon as the switch navigates.
    const puts = captureProfilePuts()
    const user = userEvent.setup()
    const { unmount } = renderAt("/g/household", <GroupRoleCluster />)

    await user.click(await screen.findByRole("button", { name: "Switch group" }))
    await user.click(await screen.findByRole("menuitem", { name: /Office/ }))
    unmount()

    await waitFor(() => expect(puts).toEqual([{ name: "Denis", default_group_id: "g2" }]))
  })

  it("does not save when the active group is picked again", async () => {
    const puts = captureProfilePuts()
    const user = userEvent.setup()
    renderAt("/g/household", <GroupSelector />)

    await user.click(await screen.findByRole("button", { name: "Switch group" }))
    await user.click(await screen.findByRole("menuitem", { name: /Household/ }))

    // Longer than the save delay.
    await new Promise((resolve) => setTimeout(resolve, 600))
    expect(puts).toEqual([])
    expect(screen.getByTestId("loc")).toHaveAttribute("data-pathname", "/g/household")
  })
})

describe("top bar role badge (#2910)", () => {
  it("shows the translated role label with the role's icon and tint", async () => {
    renderAt("/g/household", <GroupRoleCluster />)

    const badge = await screen.findByTestId("current-role")
    expect(badge).toHaveTextContent(/^Owner$/)
    expect(badge).toHaveClass("role-indicator", "role-indicator--owner", "bg-accent")
    expect(badge.querySelector("svg")).not.toBeNull()
  })
})
