import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { GroupSettingsPage } from "@/pages/groups/GroupSettingsPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import type { GroupSection } from "@/features/group/sections"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function renderSettings(initial = "/groups/g1/settings?section=sections") {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: initial,
    routes: (
      <Route
        path="/groups/:groupId/settings"
        element={
          <AuthProvider>
            <GroupProvider>
              <ConfirmProvider>
                <GroupSettingsPage />
              </ConfirmProvider>
            </GroupProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

// A server holding one group; PUT /hidden-sections replaces its list the
// way the backend does (known sections once each, in settings order).
function groupBackend(role: "admin" | "user", hidden: GroupSection[] = []) {
  const state = { hidden: [...hidden], puts: [] as GroupSection[][] }
  const attrs = () => ({
    id: "g1",
    slug: "household",
    name: "Household",
    group_currency: "USD",
    hidden_sections: state.hidden,
  })
  const order: GroupSection[] = ["warranties", "maintenance", "loans", "upcoming"]
  const handlers = [
    msw.get(api("/auth/me"), () =>
      HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
    ),
    msw.get(api("/groups"), () =>
      HttpResponse.json({ data: [{ id: "g1", type: "groups", attributes: attrs() }] })
    ),
    msw.get(api("/groups/g1"), () =>
      HttpResponse.json({ data: { id: "g1", type: "groups", attributes: attrs() } })
    ),
    msw.get(api("/groups/g1/members"), () =>
      HttpResponse.json({
        data: [
          {
            id: "m1",
            type: "memberships",
            attributes: { id: "m1", group_id: "g1", member_user_id: "u1", role },
          },
        ],
      })
    ),
    msw.put(api("/groups/g1/hidden-sections"), async ({ request }) => {
      const body = (await request.json()) as {
        data: { attributes: { hidden_sections: GroupSection[] } }
      }
      const asked = body.data.attributes.hidden_sections
      state.puts.push(asked)
      state.hidden = order.filter((s) => asked.includes(s))
      return HttpResponse.json({ data: { id: "g1", type: "groups", attributes: attrs() } })
    }),
    msw.get(api("/g/household/notifications"), () =>
      HttpResponse.json({ warranty_expiring_alerts: true, weekly_digest: false })
    ),
  ]
  return { state, handlers }
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<GroupSettingsPage /> — Sections (#1607)", () => {
  it("opens on the Sections card from ?section=sections with every section shown", async () => {
    const backend = groupBackend("admin")
    server.use(...backend.handlers)
    renderSettings()

    expect(await screen.findByTestId("sections-card")).toBeInTheDocument()
    for (const section of ["warranties", "maintenance", "loans", "upcoming"]) {
      const toggle = screen.getByTestId(`sections-toggle-${section}`)
      expect(toggle).toBeChecked()
      expect(toggle).toBeEnabled()
    }
    expect(screen.queryByTestId("sections-card-admin-only")).not.toBeInTheDocument()
  })

  it("hides a section for the group and shows it again", async () => {
    const backend = groupBackend("admin", ["loans"])
    server.use(...backend.handlers)
    const user = userEvent.setup()
    renderSettings()

    const warranties = await screen.findByTestId("sections-toggle-warranties")
    expect(screen.getByTestId("sections-toggle-loans")).not.toBeChecked()

    await user.click(warranties)
    await waitFor(() => expect(backend.state.puts).toEqual([["warranties", "loans"]]))
    await waitFor(() => expect(screen.getByTestId("sections-toggle-warranties")).not.toBeChecked())

    await user.click(screen.getByTestId("sections-toggle-loans"))
    await waitFor(() => expect(backend.state.puts).toHaveLength(2))
    expect(backend.state.puts[1]).toEqual(["warranties"])
    await waitFor(() => expect(screen.getByTestId("sections-toggle-loans")).toBeChecked())
  })

  it("shows a member without the admin role what is hidden, without letting them change it", async () => {
    const backend = groupBackend("user", ["maintenance"])
    server.use(...backend.handlers)
    renderSettings()

    expect(await screen.findByTestId("sections-card-admin-only")).toBeInTheDocument()
    expect(screen.getByTestId("sections-toggle-maintenance")).not.toBeChecked()
    expect(screen.getByTestId("sections-toggle-warranties")).toBeChecked()
    for (const section of ["warranties", "maintenance", "loans", "upcoming"]) {
      expect(screen.getByTestId(`sections-toggle-${section}`)).toBeDisabled()
    }
  })

  it("drops the warranty alert switch while the group hides warranties", async () => {
    const backend = groupBackend("admin", ["warranties"])
    server.use(...backend.handlers)
    renderSettings("/groups/g1/settings?section=notifications")

    expect(await screen.findByTestId("notifications-toggle-weekly-digest")).toBeInTheDocument()
    expect(screen.queryByTestId("notifications-toggle-warranty")).not.toBeInTheDocument()
  })
})
