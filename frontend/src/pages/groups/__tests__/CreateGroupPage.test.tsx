import { lazy, Suspense } from "react"
import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Outlet, Route, useLocation } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { useOptionalAuth } from "@/features/auth/AuthContext"
import { GroupProvider, useCurrentGroup } from "@/features/group/GroupContext"
import { CreateGroupPage } from "@/pages/groups/CreateGroupPage"
import { capture } from "@/test/capture"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

function LocationProbe() {
  const loc = useLocation()
  return <div data-testid="loc" data-pathname={loc.pathname} data-search={loc.search} />
}

// Renders what GroupProvider and AuthProvider currently hold, so a test can
// wait for a refetch to reach the tree before it moves on.
function ContextProbe() {
  const { groups } = useCurrentGroup()
  const auth = useOptionalAuth()
  return (
    <div
      data-testid="ctx"
      data-groups={(groups ?? []).map((g) => g.slug).join(",")}
      data-default-group={auth?.user?.default_group_id ?? ""}
    />
  )
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
    const captured = capture<{ data?: { attributes?: Record<string, unknown> } }>()
    server.use(
      msw.get(api("/currencies"), () => HttpResponse.json(["EUR", "GBP", "USD"])),
      msw.post(api("/groups"), async ({ request }) => {
        captured.value = (await request.json()) as typeof captured.value
        return HttpResponse.json(
          {
            data: {
              id: "g1",
              type: "groups",
              attributes: {
                id: "g1",
                slug: "household",
                name: "Household",
                group_currency: "EUR",
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
    expect(captured.value?.data?.attributes).toMatchObject({
      name: "Household",
      group_currency: "EUR",
      icon: "🏠",
    })
  })

  it("lands on /g/{slug} when a user with no groups creates the first one (#2815)", async () => {
    // Mounted the way the app mounts it: AuthProvider above the routes,
    // GroupProvider as the layout route, and a lazy /g/:groupSlug route.
    // The router applies navigations in a transition, so while the lazy
    // route loads, the committed location stays /groups/new. The refetched
    // group list re-renders GroupProvider at that location in the meantime.
    // GroupProvider must not pin ?g= there: setSearchParams would resolve
    // against /groups/new and replace the /g/household entry CreateGroupPage
    // has just pushed.
    let created = false
    let releaseGroups!: () => void
    const groupsReleased = new Promise<void>((resolve) => {
      releaseGroups = resolve
    })
    let releaseGroupRoute!: () => void
    const groupRouteLoaded = new Promise<void>((resolve) => {
      releaseGroupRoute = resolve
    })
    const GroupHome = lazy(async () => {
      await groupRouteLoaded
      return { default: LocationProbe }
    })
    const household = {
      id: "g1",
      type: "groups",
      attributes: { id: "g1", slug: "household", name: "Household", group_currency: "USD" },
    }
    server.use(
      msw.get(api("/auth/me"), () =>
        HttpResponse.json({
          id: "u1",
          email: "orphan@example.com",
          name: "Orphan",
          // The backend makes the first group the user's default.
          default_group_id: created ? "g1" : null,
        })
      ),
      msw.get(api("/groups"), async () => {
        if (!created) return HttpResponse.json({ data: [] })
        await groupsReleased
        return HttpResponse.json({ data: [household] })
      }),
      msw.post(api("/groups"), () => {
        created = true
        return HttpResponse.json({ data: household }, { status: 201 })
      })
    )
    setAccessToken("good-token")
    renderWithProviders({
      initialPath: "/groups/new",
      withAuth: true,
      routes: (
        <Route
          element={
            <GroupProvider>
              <ContextProbe />
              <Suspense fallback={null}>
                <Outlet />
              </Suspense>
            </GroupProvider>
          }
        >
          <Route path="/groups/new" element={<CreateGroupPage />} />
          <Route path="/g/:groupSlug" element={<GroupHome />} />
        </Route>
      ),
    })
    const user = userEvent.setup()
    await user.type(screen.getByTestId("group-name-input"), "Household")
    await user.click(screen.getByTestId("create-group-submit"))

    // The /auth/me refetch lands first; the /groups refetch is still held,
    // so the create has not resolved and nothing has navigated yet.
    await waitFor(() =>
      expect(screen.getByTestId("ctx").getAttribute("data-default-group")).toBe("g1")
    )
    // Releasing /groups resolves the create: CreateGroupPage navigates to
    // /g/household, and that navigation waits on the lazy route while the
    // new group list renders at /groups/new.
    releaseGroups()
    await waitFor(() =>
      expect(screen.getByTestId("ctx").getAttribute("data-groups")).toBe("household")
    )
    releaseGroupRoute()

    await waitFor(() =>
      expect(screen.getByTestId("loc").getAttribute("data-pathname")).toBe("/g/household")
    )
    expect(screen.getByTestId("loc").getAttribute("data-search")).toBe("")
    expect(screen.queryByTestId("create-group-page")).not.toBeInTheDocument()
  })

  it("surfaces server errors inline on 422", async () => {
    server.use(
      msw.post(api("/groups"), () =>
        HttpResponse.json(
          { errors: [{ detail: "group_currency must be a valid ISO code" }] },
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

  it("treats a slug-less success response as an error and keeps the form mounted (#1886)", async () => {
    // The BE invariant is that every persisted group carries a slug,
    // so a 201 without one is a contract violation. We surface that as
    // an inline server error rather than silently navigating to
    // /no-group — which would also drop the user's active-group
    // context when they reached the form via GroupSelector.
    server.use(
      msw.post(api("/groups"), () =>
        HttpResponse.json(
          {
            data: {
              id: "g1",
              type: "groups",
              attributes: {
                id: "g1",
                name: "Household",
                group_currency: "USD",
                icon: "",
              },
            },
          },
          { status: 201 }
        )
      )
    )
    const user = userEvent.setup()
    renderCreate()
    await user.type(screen.getByTestId("group-name-input"), "Household")
    await user.click(screen.getByTestId("create-group-submit"))
    expect(await screen.findByTestId("create-group-server-error")).toBeInTheDocument()
    // The form must still be present — the user can retry without
    // losing context. A navigation away would unmount CreateGroupPage
    // and `*` would catch the new pathname via <LocationProbe />.
    expect(screen.getByTestId("create-group-page")).toBeInTheDocument()
    expect(screen.queryByTestId("loc")).not.toBeInTheDocument()
  })

  it("has no axe violations on the form", async () => {
    const { container } = renderCreate()
    expect(await axe(container)).toHaveNoViolations()
  })
})
