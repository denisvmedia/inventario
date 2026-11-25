import { beforeEach, describe, expect, it } from "vitest"
import { Route, useLocation, type Location } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { delay, http, HttpResponse } from "msw"

import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { InboxPage } from "@/pages/inbox/InboxPage"
import { toastSpies } from "@/test/global-mocks"
import { apiUrl, authHandlers, groupHandlers, inboxHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groups: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function members(role: "viewer" | "user" | "admin" | "owner") {
  return http.get(apiUrl("/groups/g1/members"), () =>
    HttpResponse.json({
      data: [
        {
          id: "m1",
          type: "memberships",
          attributes: { group_id: "g1", member_user_id: "u1", role },
        },
      ],
    })
  )
}

function Probe() {
  const loc = useLocation()
  const background = (loc.state as { background?: Location } | null)?.background
  return (
    <div data-testid="loc" data-background={background?.pathname ?? ""}>
      {`${loc.pathname}${loc.search}`}
    </div>
  )
}

function renderPage(path = `/g/${SLUG}/inbox`) {
  return renderWithProviders({
    initialPath: path,
    routes: (
      <>
        <Route
          path="/g/:groupSlug/inbox"
          element={
            <AuthProvider>
              <ConfirmProvider>
                <GroupProvider>
                  <InboxPage />
                </GroupProvider>
              </ConfirmProvider>
            </AuthProvider>
          }
        />
        <Route path="/g/:groupSlug/commodities/new" element={<Probe />} />
        <Route path="/g/:groupSlug/commodities/:id" element={<Probe />} />
        <Route path="/g/:groupSlug/commodities" element={<Probe />} />
      </>
    ),
  })
}

function seed(role: "viewer" | "user" | "admin" | "owner" = "user") {
  const fake = inboxHandlers.backend(SLUG, [
    inboxHandlers.draft("d1", "Cordless drill", { file_count: 2 }),
    inboxHandlers.draft("d2", ""),
    inboxHandlers.itemDraft("c1", "Bike"),
  ])
  server.use(
    ...authHandlers.signedIn(),
    ...groupHandlers.list(groups),
    members(role),
    ...fake.handlers
  )
  return fake
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  setCurrentGroupSlug(SLUG)
})

describe("<InboxPage />", () => {
  it("lists one row per record with its reason and counts each tab", async () => {
    seed()
    renderPage()
    const list = await screen.findByTestId("inbox-list")
    const drill = within(list).getByTestId("inbox-row-draft-d1")
    expect(drill).toHaveTextContent("Cordless drill")
    expect(within(drill).getByTestId("inbox-reason")).toHaveTextContent("Draft")
    expect(drill).toHaveTextContent("2 files")
    expect(within(list).getByTestId("inbox-row-draft-d2")).toHaveTextContent("Untitled draft")
    const bike = within(list).getByTestId("inbox-row-commodity-c1")
    expect(within(bike).getByTestId("inbox-reason")).toHaveTextContent("Marked as draft")

    expect(screen.getByTestId("inbox-tab-all")).toHaveTextContent("All (3)")
    expect(screen.getByTestId("inbox-tab-draft")).toHaveTextContent("Drafts (2)")
    expect(screen.getByTestId("inbox-tab-item_draft")).toHaveTextContent("Items to review (1)")
  })

  it("has no axe violations", async () => {
    seed()
    const { container } = renderPage()
    await screen.findByTestId("inbox-review-c1")
    expect(await axe(container)).toHaveNoViolations()
  })

  it("filters by reason on the server and keeps the totals", async () => {
    seed()
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("inbox-row-draft-d1")
    await user.click(screen.getByTestId("inbox-tab-item_draft"))
    await waitFor(() => expect(screen.queryByTestId("inbox-row-draft-d1")).not.toBeInTheDocument())
    expect(screen.getByTestId("inbox-row-commodity-c1")).toBeInTheDocument()
    expect(screen.getByTestId("inbox-tab-all")).toHaveTextContent("All (3)")
  })

  it("shows no rows of the previous tab while the new tab loads", async () => {
    seed()
    server.use(
      http.get(apiUrl(`/g/${SLUG}/inbox`), async ({ request }) => {
        if (new URL(request.url).searchParams.get("reason") === "item_draft") {
          await delay("infinite")
        }
        return undefined
      })
    )
    const user = userEvent.setup()
    renderPage()
    await screen.findByTestId("inbox-row-draft-d1")
    await user.click(screen.getByTestId("inbox-tab-item_draft"))
    const panel = screen.getByRole("tabpanel")
    expect(within(panel).getByTestId("inbox-loading")).toBeInTheDocument()
    expect(within(panel).queryByTestId("inbox-row-draft-d1")).not.toBeInTheDocument()
    expect(within(panel).queryByTestId("inbox-resume-d1")).not.toBeInTheDocument()
    // The tabs keep their counts meanwhile.
    expect(screen.getByTestId("inbox-tab-all")).toHaveTextContent("All (3)")
  })

  it("names the record in the label of each row action", async () => {
    seed()
    renderPage()
    expect(await screen.findByTestId("inbox-resume-d1")).toHaveAccessibleName(
      "Continue: Cordless drill"
    )
    expect(screen.getByTestId("inbox-resume-d2")).toHaveAccessibleName("Continue: Untitled draft")
    expect(screen.getByTestId("inbox-review-c1")).toHaveAccessibleName("Mark as reviewed: Bike")
  })

  it("opens on the tab the address names", async () => {
    seed()
    renderPage(`/g/${SLUG}/inbox?reason=draft`)
    expect(await screen.findByTestId("inbox-row-draft-d1")).toBeInTheDocument()
    expect(screen.queryByTestId("inbox-row-commodity-c1")).not.toBeInTheDocument()
    expect(screen.getByTestId("inbox-tab-draft")).toHaveAttribute("aria-selected", "true")
  })

  it("continues a draft in the item dialog over the inbox", async () => {
    seed()
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("inbox-resume-d1"))
    const loc = await screen.findByTestId("loc")
    expect(loc).toHaveTextContent(`/g/${SLUG}/commodities/new?draft=d1`)
    expect(loc).toHaveAttribute("data-background", `/g/${SLUG}/inbox`)
  })

  it("discards a draft after confirmation", async () => {
    const fake = seed()
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("inbox-discard-d1"))
    const dialog = await screen.findByTestId("confirm-dialog")
    expect(dialog).toHaveTextContent(/Trash/)
    await user.click(within(dialog).getByTestId("confirm-accept"))
    await waitFor(() => expect(screen.queryByTestId("inbox-row-draft-d1")).not.toBeInTheDocument())
    expect(fake.entries.map((e) => e.id)).toEqual(["d2", "c1"])
    expect(screen.getByTestId("inbox-tab-all")).toHaveTextContent("All (2)")
  })

  it("marks an item as reviewed and undoes it from the toast", async () => {
    const fake = seed()
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("inbox-review-c1"))
    await waitFor(() =>
      expect(screen.queryByTestId("inbox-row-commodity-c1")).not.toBeInTheDocument()
    )
    expect(fake.reviewed).toEqual(["c1"])
    expect(toastSpies.success).toHaveBeenCalledWith(
      "“Bike” marked as reviewed",
      expect.objectContaining({ action: expect.objectContaining({ label: "Undo" }) })
    )
    const options = toastSpies.success.mock.calls[0]![1] as {
      action: { onClick: () => void }
    }
    options.action.onClick()
    expect(await screen.findByTestId("inbox-row-commodity-c1")).toBeInTheDocument()
  })

  it("says why an item could not be marked as reviewed", async () => {
    const fake = seed()
    fake.failReview = inboxHandlers.refusedReview()
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("inbox-review-c1"))
    await waitFor(() => expect(toastSpies.error).toHaveBeenCalled())
    expect(toastSpies.error).toHaveBeenCalledWith("The item could not be marked as reviewed.", {
      description:
        "Enter at least one of converted price or current value so the item is reportable in group currency.",
      action: expect.objectContaining({ label: "Open item" }),
    })
    expect(screen.getByTestId("inbox-row-commodity-c1")).toBeInTheDocument()
  })

  it("names the field and the rule the server refused", async () => {
    const fake = seed()
    fake.failReview = {
      status: 422,
      body: {
        errors: [
          {
            status: "Unprocessable Entity",
            error: {
              error: { data: { attributes: { count: "must be no less than 1" } } },
              errorCodes: {
                data: {
                  attributes: {
                    count: {
                      code: "validation_min_greater_equal_than_required",
                      params: { threshold: 1 },
                    },
                  },
                },
              },
              type: "validation.Errors",
            },
          },
        ],
      },
    }
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("inbox-review-c1"))
    await waitFor(() => expect(toastSpies.error).toHaveBeenCalled())
    expect(toastSpies.error.mock.calls[0]![1]).toMatchObject({
      description: "Quantity must be at least 1.",
    })
  })

  it("falls back to the server's message when the refusal names no field", async () => {
    const fake = seed()
    fake.failReview = { status: 409, body: { errors: [{ status: "409", detail: "Busy" }] } }
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("inbox-review-c1"))
    await waitFor(() => expect(toastSpies.error).toHaveBeenCalled())
    expect(toastSpies.error.mock.calls[0]![0]).toBe("Busy")
  })

  it("opens an item over the inbox", async () => {
    seed()
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByTestId("inbox-open-c1"))
    const loc = await screen.findByTestId("loc")
    expect(loc).toHaveTextContent(`/g/${SLUG}/commodities/c1`)
    expect(loc).toHaveAttribute("data-background", `/g/${SLUG}/inbox`)
  })

  it("offers a viewer no actions", async () => {
    seed("viewer")
    renderPage()
    const row = await screen.findByTestId("inbox-row-draft-d1")
    await waitFor(() => expect(screen.queryByTestId("inbox-resume-d1")).not.toBeInTheDocument())
    expect(within(row).queryByTestId("inbox-discard-d1")).not.toBeInTheDocument()
    expect(screen.queryByTestId("inbox-review-c1")).not.toBeInTheDocument()
    // The item itself still opens.
    expect(screen.getByTestId("inbox-open-c1")).toBeInTheDocument()
  })

  it("offers no actions while the member's role is loading", async () => {
    const fake = inboxHandlers.backend(SLUG, [inboxHandlers.draft("d1", "Lamp")])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      http.get(apiUrl("/groups/g1/members"), async () => {
        await delay("infinite")
        return HttpResponse.json({ data: [] })
      }),
      ...fake.handlers
    )
    renderPage()
    const row = await screen.findByTestId("inbox-row-draft-d1")
    expect(within(row).queryByTestId("inbox-resume-d1")).not.toBeInTheDocument()
  })

  it("says when nothing needs attention and links the missing-detail lists", async () => {
    const fake = inboxHandlers.backend(SLUG)
    fake.checks = inboxHandlers
      .defaultChecks()
      .map((c) => (c.key === "no_photo" ? { ...c, count: 4 } : c))
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    renderPage()
    expect(await screen.findByTestId("inbox-empty")).toHaveTextContent("Nothing needs attention")
    // The checks are not unfinished work: four items without a photo leave
    // the inbox empty.
    expect(screen.getByTestId("inbox-tab-all")).toHaveTextContent("All (0)")
    const links: [string, string][] = [
      ["no-location", "unassigned=1"],
      ["no-photo", "no_photo=1"],
      ["no-purchase-price", "no_purchase_price=1"],
      ["no-purchase-date", "no_purchase_date=1"],
      ["untagged", "no_tags=1"],
    ]
    for (const [key, qs] of links) {
      expect(await screen.findByTestId(`inbox-missing-${key}`)).toHaveAttribute(
        "href",
        `/g/${SLUG}/commodities?${qs}`
      )
    }
    expect(screen.getByTestId("inbox-missing-no-photo-count")).toHaveTextContent("4")
    expect(screen.getByTestId("inbox-missing-no-location-count")).toHaveTextContent("0")
  })

  it("leaves out a check it does not know or would read differently", async () => {
    const fake = inboxHandlers.backend(SLUG)
    fake.checks = [
      { key: "no_photo", query: "include_inactive=false&no_photo=true", count: 1 },
      { key: "no_serial", query: "include_inactive=false&no_serial=true", count: 2 },
      { key: "untagged", query: "include_inactive=false&no_tags=true&shape=round", count: 3 },
    ]
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    renderPage()
    expect(await screen.findByTestId("inbox-missing-no-photo")).toBeInTheDocument()
    const card = screen.getByTestId("inbox-missing")
    expect(within(card).getAllByRole("link")).toHaveLength(1)
  })

  it("pages a long inbox", async () => {
    const many = Array.from({ length: 51 }, (_, i) =>
      inboxHandlers.draft(`d${i + 1}`, `Draft ${i + 1}`)
    )
    const fake = inboxHandlers.backend(SLUG, many)
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    const user = userEvent.setup()
    renderPage()
    expect(await screen.findByTestId("inbox-row-draft-d1")).toBeInTheDocument()
    const pager = screen.getByTestId("inbox-pagination")
    await user.click(within(pager).getByRole("button", { name: "2" }))
    expect(await screen.findByTestId("inbox-row-draft-d51")).toBeInTheDocument()
    expect(screen.queryByTestId("inbox-row-draft-d1")).not.toBeInTheDocument()
  })
})
