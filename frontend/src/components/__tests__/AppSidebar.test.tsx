import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { http } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"

import { AppSidebar } from "@/components/AppSidebar"
import { SidebarProvider } from "@/components/ui/sidebar"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import {
  apiUrl,
  authHandlers,
  groupHandlers,
  inboxHandlers,
  savedViewHandlers,
} from "@/test/handlers"
import { stubImageLoading } from "@/test/image"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

// Mount AppSidebar inside the same provider chain Shell uses at runtime
// (AuthProvider -> GroupProvider -> SidebarProvider) so the test sees the
// same currentGroup / groups wiring the production sidebar does.
function renderSidebar(initialPath = "/no-group") {
  setAccessToken("good-token")
  // The current cases only need the root /g/:slug shape — if a future
  // test passes /g/<slug>/<sub>, this mount will still resolve only
  // /g/:slug, won't match, and AppSidebar won't render. Widen the
  // pattern then (e.g. /g/:groupSlug/*).
  const routePath = initialPath.startsWith("/g/") ? "/g/:groupSlug" : initialPath
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        path={routePath}
        element={
          <AuthProvider>
            <GroupProvider>
              <SidebarProvider>
                <AppSidebar />
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
  // The sidebar lists the group's saved views and counts its inbox.
  server.use(
    ...savedViewHandlers.none("household"),
    ...savedViewHandlers.none("office"),
    ...inboxHandlers.empty("household"),
    ...inboxHandlers.empty("office")
  )
})

describe("<AppSidebar /> — group-section gating (#1886)", () => {
  it("hides Inventory + Group sections when the user belongs to no groups", async () => {
    server.use(...authHandlers.signedIn(), ...groupHandlers.empty())
    renderSidebar("/no-group")
    // Personal always renders unconditionally. The Inventory + Group
    // sections render during the in-flight window (groups === undefined)
    // and only retract once /groups resolves []. Wait for the retract
    // explicitly so the test isn't reading the loading-state snapshot.
    await waitFor(() =>
      expect(screen.queryByTestId("sidebar-inventory-group")).not.toBeInTheDocument()
    )
    expect(screen.queryByTestId("sidebar-manage-group")).not.toBeInTheDocument()
    expect(screen.getByText("Personal")).toBeInTheDocument()
  })

  it("renders all three sections when the user has at least one group", async () => {
    server.use(...authHandlers.signedIn(), ...groupHandlers.list())
    renderSidebar("/g/household")
    await waitFor(() => expect(screen.getByTestId("sidebar-inventory-group")).toBeInTheDocument())
    expect(screen.getByTestId("sidebar-manage-group")).toBeInTheDocument()
    expect(screen.getByText("Personal")).toBeInTheDocument()
  })

  it("keeps the group sections rendered while /groups is still loading", async () => {
    // Hang the /groups response so we observe the in-flight state and
    // verify we don't flash the sections off and back on. A never-
    // resolving handler keeps `groups === undefined` for the entirety
    // of the test, which is exactly the loading branch the gate covers.
    server.use(
      ...authHandlers.signedIn(),
      http.get(apiUrl("/groups"), () => new Promise<never>(() => {}))
    )
    renderSidebar("/no-group")
    // Personal renders unconditionally — using it as the readiness probe
    // so the assertion isn't racing the first render.
    await waitFor(() => expect(screen.getByText("Personal")).toBeInTheDocument())
    // groups hasn't resolved, so the section headers must still be in
    // the DOM. If they weren't, the user would see "no sections" briefly
    // and then "sections appear" once /groups lands.
    expect(screen.getByTestId("sidebar-inventory-group")).toBeInTheDocument()
    expect(screen.getByTestId("sidebar-manage-group")).toBeInTheDocument()
  })

  // #1384 — the help entry point. It sits in the footer rather than in the
  // Personal section because it resolves to the same page as Preferences, and
  // the nav rows match on pathname alone, so two rows pointing at /settings
  // would light up together.
  //
  // The group query is withGroupQuery's job and is covered in its own tests
  // (including the case this call site introduces: a path that already
  // carries a query).
  it("links Help at the settings help section", async () => {
    server.use(...authHandlers.signedIn(), ...groupHandlers.list())
    renderSidebar("/g/household")
    const help = await screen.findByTestId("sidebar-help")
    expect(help.getAttribute("href")).toBe("/settings?section=help")
  })

  it("links Trash as the last row of the Group section", async () => {
    server.use(...authHandlers.signedIn(), ...groupHandlers.list())
    renderSidebar("/g/household")
    const manage = await screen.findByTestId("sidebar-manage-group")
    // The rows render once the group resolves.
    const links = await within(manage).findAllByRole("link")
    const trash = links[links.length - 1]
    expect(trash).toHaveTextContent("Trash")
    expect(trash.getAttribute("href")).toBe("/g/household/trash")
  })

  it("offers Help to a user with no group", async () => {
    server.use(...authHandlers.signedIn(), ...groupHandlers.list([]))
    renderSidebar("/no-group")
    const help = await screen.findByTestId("sidebar-help")
    expect(help.getAttribute("href")).toBe("/settings?section=help")
  })
})

describe("<AppSidebar /> — user menu avatar (#1382)", () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it("shows the initials when the user has no photo", async () => {
    server.use(
      ...authHandlers.signedIn({ user: { name: "Alex Johnson" } }),
      ...groupHandlers.empty()
    )
    renderSidebar("/no-group")
    const avatar = await screen.findByTestId("user-menu-avatar")
    await waitFor(() =>
      expect(within(avatar).getByTestId("user-avatar-initials")).toHaveTextContent("AJ")
    )
  })

  it("shows the photo when the user has one", async () => {
    stubImageLoading()
    server.use(
      ...authHandlers.signedIn({
        user: { name: "Alex Johnson", avatar_url: "/api/v1/avatars/u1?sig=a" },
      }),
      ...groupHandlers.empty()
    )
    renderSidebar("/no-group")
    const avatar = await screen.findByTestId("user-menu-avatar")
    await waitFor(() =>
      expect(within(avatar).getByTestId("user-avatar-image")).toHaveAttribute(
        "src",
        "/api/v1/avatars/u1?sig=a"
      )
    )
  })
})

describe("<AppSidebar /> — saved views", () => {
  it("lists the views pinned to the sidebar with their item counts", async () => {
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(),
      ...savedViewHandlers.list("household", [
        { id: "sv1", name: "Expiring", query: "warranty_status=expiring", show_in_sidebar: true },
        { id: "sv2", name: "Not pinned", query: "lent_out=true", show_in_sidebar: false },
      ]),
      ...savedViewHandlers.counts("household", { sv1: 7, sv2: 3 })
    )
    renderSidebar("/g/household")

    const section = await screen.findByTestId("sidebar-saved-views")
    const links = within(section).getAllByTestId("sidebar-saved-view")
    expect(links).toHaveLength(1)
    expect(links[0]).toHaveTextContent("Expiring")
    const href = links[0].getAttribute("href") ?? ""
    expect(href).toContain("/g/household/commodities?")
    expect(href).toContain("saved_view=sv1")
    expect(href).toContain("warranty=expiring")
    await waitFor(() =>
      expect(within(section).getByTestId("sidebar-saved-view-count")).toHaveTextContent("7")
    )
    expect(within(section).getByTestId("sidebar-saved-views-manage")).toHaveAttribute(
      "href",
      "/g/household/views"
    )
  })

  // The list would drop the condition it cannot read and show more than the
  // view, so the entry is not a link.
  it("lists a view whose stored query cannot run without a link", async () => {
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(),
      ...savedViewHandlers.list("household", [
        {
          id: "sv3",
          name: "Old warranties",
          query: "type=electronics&warranty_expires_before=2026-12-01",
          show_in_sidebar: true,
          query_error: { parameter: "warranty_expires_before", reason: "is not available" },
        },
      ]),
      ...savedViewHandlers.counts("household", {})
    )
    renderSidebar("/g/household")

    const section = await screen.findByTestId("sidebar-saved-views")
    const entry = within(section).getByTestId("sidebar-saved-view-unavailable")
    expect(entry).toHaveTextContent("Old warranties")
    expect(entry).toHaveAttribute("aria-disabled", "true")
    expect(entry.closest("a")).toBeNull()
    expect(within(entry).getByRole("img", { name: "Needs an update" })).toBeInTheDocument()
    expect(within(section).queryByTestId("sidebar-saved-view")).not.toBeInTheDocument()
  })

  it("leaves the section out while no view is pinned", async () => {
    server.use(...authHandlers.signedIn(), ...groupHandlers.list())
    renderSidebar("/g/household")
    await waitFor(() => expect(screen.getByTestId("sidebar-inventory-group")).toBeInTheDocument())
    expect(screen.queryByTestId("sidebar-saved-views")).not.toBeInTheDocument()
  })
})

describe("<AppSidebar /> — inbox (#2794)", () => {
  it("links the inbox after the dashboard and shows the server's count", async () => {
    const fake = inboxHandlers.backend("household", [
      inboxHandlers.draft("d1", "Drill"),
      inboxHandlers.draft("d2", "Lamp"),
      inboxHandlers.itemDraft("c1", "Bike"),
    ])
    server.use(...authHandlers.signedIn(), ...groupHandlers.list(), ...fake.handlers)
    renderSidebar("/g/household")

    const inventory = await screen.findByTestId("sidebar-inventory-group")
    // The rows render once the groups resolve.
    // The count is part of the link's name; the badge itself is hidden
    // from screen readers.
    await within(inventory).findByRole("link", { name: "Inbox, 3 need attention" })
    const links = within(inventory).getAllByRole("link")
    expect(links[0]).toHaveTextContent("Dashboard")
    expect(links[1]).toHaveTextContent("Inbox")
    expect(links[1]).toHaveAttribute("href", "/g/household/inbox")
    expect(await within(inventory).findByTestId("sidebar-inbox-count")).toHaveTextContent("3")
  })

  it("shows no count while the inbox is empty", async () => {
    server.use(...authHandlers.signedIn(), ...groupHandlers.list())
    renderSidebar("/g/household")
    const inventory = await screen.findByTestId("sidebar-inventory-group")
    expect(await within(inventory).findByRole("link", { name: "Inbox" })).toBeInTheDocument()
    expect(within(inventory).queryByTestId("sidebar-inbox-count")).not.toBeInTheDocument()
  })
})

describe("<AppSidebar /> — hidden sections (#1607)", () => {
  function inventoryLinkTexts(inventory: HTMLElement): string[] {
    return within(inventory)
      .getAllByRole("link")
      .map((l) => l.textContent ?? "")
  }

  it("lists every section while the group hides none", async () => {
    server.use(...authHandlers.signedIn(), ...groupHandlers.list())
    renderSidebar("/g/household")
    const inventory = await screen.findByTestId("sidebar-inventory-group")
    await within(inventory).findByRole("link", { name: "Warranties" })
    expect(inventoryLinkTexts(inventory)).toEqual(
      expect.arrayContaining(["Upcoming", "Warranties", "Maintenance", "Lent out", "In service"])
    )
  })

  it("leaves out the entries of the sections the group hides", async () => {
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list([
        {
          id: "g1",
          slug: "household",
          name: "Household",
          hidden_sections: ["warranties", "maintenance"],
        },
        { id: "g2", slug: "office", name: "Office" },
      ])
    )
    renderSidebar("/g/household")
    const inventory = await screen.findByTestId("sidebar-inventory-group")
    await within(inventory).findByRole("link", { name: "Lent out" })
    expect(inventoryLinkTexts(inventory)).toEqual([
      "Dashboard",
      "Inbox",
      "Locations",
      "All Items",
      "Upcoming",
      "Lent out",
    ])
  })

  it("follows the setting of the group in the URL", async () => {
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list([
        { id: "g1", slug: "household", name: "Household", hidden_sections: ["loans"] },
        { id: "g2", slug: "office", name: "Office", hidden_sections: ["upcoming"] },
      ])
    )
    renderSidebar("/g/office")
    const inventory = await screen.findByTestId("sidebar-inventory-group")
    await within(inventory).findByRole("link", { name: "Lent out" })
    expect(inventoryLinkTexts(inventory)).not.toContain("Upcoming")
  })
})
