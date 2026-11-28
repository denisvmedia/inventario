import { beforeEach, describe, expect, it } from "vitest"
import type { ReactNode } from "react"
import { http as msw, HttpResponse } from "msw"
import { Route, useLocation, useNavigate } from "react-router-dom"
import { fireEvent, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { CommoditiesListPage } from "@/pages/commodities/CommoditiesListPage"
import { useAuth } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import {
  apiUrl,
  areaHandlers,
  commodityHandlers,
  draftHandlers,
  groupHandlers,
  locationHandlers,
  memberHandlers,
} from "@/test/handlers"
import { pickRadixSelect } from "@/test/radix"
import { WhenGroupLoaded } from "@/test/group-loaded"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

// The Add item form keeps its unsaved input per user (#2920), and every
// request it sends goes to the group it opened in, even after the member
// has moved to another group.

const SLUG = "household"
const OTHER = "garage"
const groups: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
  { id: "g2", slug: OTHER, name: "Garage", group_currency: "USD" },
]
const areas = [
  { id: "a1", type: "areas", attributes: { id: "a1", name: "Hall", location_id: "l1" } },
]
const locations = [{ id: "l1", type: "locations", attributes: { id: "l1", name: "Home" } }]

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  window.localStorage.clear()
  window.sessionStorage.clear()
})

// SignedIn renders the page once the user is known, as the app's route
// guard does, and WhenGroupLoaded once the group is: the form keeps its input
// only under a key that names both.
function SignedIn({ children }: { children: ReactNode }) {
  const { user } = useAuth()
  return user ? children : null
}

function listHandlers(slug: string) {
  return [
    ...areaHandlers.list(slug, areas),
    ...locationHandlers.list(slug, locations),
    ...commodityHandlers.list(slug, []),
  ]
}

// openAddItem leaves the AI step for the form, whatever step it opens on.
async function openAddItem(user: ReturnType<typeof userEvent.setup>) {
  await waitFor(() =>
    expect(
      screen.queryByTestId("commodity-form-ai-fill-manually") ?? screen.queryByLabelText(/^Name$/i)
    ).not.toBeNull()
  )
  const manual = screen.queryByTestId("commodity-form-ai-fill-manually")
  if (manual) await user.click(manual)
  return screen.findByLabelText(/^Name$/i)
}

describe("<CommoditiesListPage /> Add item recovery (#2920)", () => {
  function renderAddItem(userId: string) {
    setAccessToken("good-token")
    server.use(
      ...memberHandlers.asRole("g1", "owner", userId),
      ...groupHandlers.list(groups),
      ...listHandlers(SLUG)
    )
    return renderWithProviders({
      withAuth: true,
      initialPath: `/g/${SLUG}/commodities/new`,
      routes: (
        <Route
          path="/g/:groupSlug/commodities/new"
          element={
            <SignedIn>
              <GroupProvider>
                <WhenGroupLoaded>
                  <ConfirmProvider>
                    <CommoditiesListPage />
                  </ConfirmProvider>
                </WhenGroupLoaded>
              </GroupProvider>
            </SignedIn>
          }
        />
      ),
    })
  }

  it("does not give one user's unsaved input to the next user of the browser", async () => {
    const user = userEvent.setup()
    const first = renderAddItem("user-a")
    await user.type(await openAddItem(user), "Alice's couch")
    // The form keeps the input on the browser as it is typed.
    await waitFor(() =>
      expect(Object.values({ ...window.localStorage }).join()).toContain("Alice's couch")
    )
    first.unmount()

    renderAddItem("user-b")
    expect(await openAddItem(user)).toHaveValue("")
  })
})

describe("<CommoditiesListPage /> Add item in the group it opened in (#2920)", () => {
  it("finishes the file in that group after Back lands in another group", async () => {
    const user = userEvent.setup()
    const drafts = draftHandlers.backend(SLUG)
    drafts.nextUploads = [{ name: "receipt.png", size: 3 }]
    let release = () => {}
    const held = new Promise<void>((resolve) => {
      release = resolve
    })
    let uploading = false
    // What the form sent to the other group besides reading its pages.
    const sentToOther: string[] = []
    setAccessToken("good-token")
    server.use(
      ...memberHandlers.asRole("g1", "owner"),
      ...memberHandlers.asRole("g2", "owner"),
      ...groupHandlers.list(groups),
      ...drafts.handlers,
      ...listHandlers(SLUG),
      ...listHandlers(OTHER)
    )
    // Registered after the fakes, so they are asked first.
    server.use(
      msw.all(apiUrl(`/g/${OTHER}/*`), ({ request }) => {
        if (request.method === "GET") return undefined
        sentToOther.push(`${request.method} ${new URL(request.url).pathname}`)
        return HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
      }),
      // The upload answers once the member has left.
      msw.post(apiUrl(`/g/${SLUG}/uploads/file`), async () => {
        uploading = true
        await held
        return undefined
      })
    )
    function Shell() {
      const location = useLocation()
      const navigate = useNavigate()
      return (
        <>
          <span data-testid="nav-sentinel-path">{location.pathname}</span>
          <button data-testid="back" type="button" onClick={() => navigate(-1)}>
            Back
          </button>
          <SignedIn>
            <GroupProvider>
              <WhenGroupLoaded>
                <ConfirmProvider>
                  <CommoditiesListPage />
                </ConfirmProvider>
              </WhenGroupLoaded>
            </GroupProvider>
          </SignedIn>
        </>
      )
    }
    renderWithProviders({
      withAuth: true,
      initialEntries: [
        { pathname: `/g/${OTHER}/commodities` },
        { pathname: `/g/${SLUG}/commodities/new` },
      ],
      routes: (
        <>
          <Route path="/g/:groupSlug/commodities" element={<Shell />} />
          <Route path="/g/:groupSlug/commodities/new" element={<Shell />} />
        </>
      ),
    })
    await user.type(await openAddItem(user), "Couch")
    await pickRadixSelect(user, /^Type$/i, { optionLabel: /^Furniture$/i })
    // Purchase, Warranty, Extras, then Files.
    for (let i = 0; i < 4; i++) await user.click(screen.getByTestId("commodity-form-next"))
    const input = (await screen.findByTestId("commodity-files-dropzone")).querySelector(
      "input[type=file]"
    ) as HTMLInputElement
    await user.upload(input, new File(["png"], "receipt.png", { type: "image/png" }))
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(uploading).toBe(true))

    // Browser Back lands in the other group; the open dialog blocks pointer
    // events on the page.
    fireEvent.click(screen.getByTestId("back"))
    await waitFor(() =>
      expect(screen.getByTestId("nav-sentinel-path")).toHaveTextContent(`/g/${OTHER}/commodities`)
    )
    await waitFor(() =>
      expect(screen.queryByTestId("commodity-form-submit")).not.toBeInTheDocument()
    )
    release()

    // The file the form was sending reaches the draft in its own group.
    await waitFor(() => expect(drafts.files[0]?.links.map((l) => l.entity_type)).toEqual(["draft"]))
    expect(drafts.files[0]?.title).toBe("receipt")
    expect(sentToOther).toEqual([])
  })
})
