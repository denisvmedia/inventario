// Focused unit tests for the commodity audit timeline (#1450). Covers
// every kind branch in `labelFor`, the show-more / show-less collapse,
// and the loading + error states the page-level test only exercises
// transitively. Pulled out of the page test so the branch coverage stays
// concentrated on the file under test.

import { afterEach, beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"

import { CommodityHistoryTimeline } from "@/features/commodities/CommodityHistoryTimeline"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { areaHandlers, commodityHandlers, groupHandlers } from "@/test/handlers"
import type { CommodityEventFixture } from "@/test/handlers/commodities"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import type { Schema } from "@/types"

const SLUG = "household"
const COMMODITY_ID = "c1"

const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
]

const areaFixture = [
  { id: "a1", type: "areas", attributes: { id: "a1", name: "Living Room", location_id: "l1" } },
  { id: "a2", type: "areas", attributes: { id: "a2", name: "Storage Unit", location_id: "l1" } },
]

const actor = { id: "u1", name: "Denis", email: "d@example.com" }

function renderTimeline() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities/${COMMODITY_ID}`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id"
        element={
          <GroupProvider>
            <CommodityHistoryTimeline commodityId={COMMODITY_ID} />
          </GroupProvider>
        }
      />
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

afterEach(() => {
  server.resetHandlers()
})

describe("<CommodityHistoryTimeline />", () => {
  it("renders kind-aware copy for each event kind", async () => {
    const rows: CommodityEventFixture[] = [
      {
        id: "e1",
        kind: "status_changed",
        occurred_at: "2026-04-25T10:30:00Z",
        before: { status: "in_use" },
        after: { status: "sold" },
        meta: { actor },
      },
      {
        id: "e2",
        kind: "moved",
        occurred_at: "2026-04-24T10:30:00Z",
        before: { area_id: "a1" },
        after: { area_id: "a2" },
        meta: { actor },
      },
      {
        id: "e3",
        kind: "price_changed",
        occurred_at: "2026-04-23T10:30:00Z",
        before: { current_price: "100" },
        after: { current_price: "50" },
        meta: { actor },
      },
      {
        id: "e4",
        kind: "cover_changed",
        occurred_at: "2026-04-22T10:30:00Z",
        before: { cover_file_id: "" },
        after: { cover_file_id: "f1" },
        meta: { actor },
      },
      {
        id: "e5",
        kind: "updated",
        occurred_at: "2026-04-21T10:30:00Z",
        before: { name: "Old" },
        after: { name: "New" },
        meta: { actor },
      },
      {
        id: "e6",
        kind: "created",
        occurred_at: "2026-04-20T10:30:00Z",
        after: { name: "New", area_id: "a1", status: "in_use" },
        meta: { actor },
      },
    ]
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.events(SLUG, COMMODITY_ID, rows)
    )

    renderTimeline()
    expect(await screen.findByTestId("history-row-status_changed")).toHaveTextContent(
      /Status: In use → Sold/i
    )
    // Moved label resolves area ids to readable names from the areas registry.
    expect(screen.getByTestId("history-row-moved")).toHaveTextContent(/Living Room → Storage Unit/i)
    expect(screen.getByTestId("history-row-price_changed")).toHaveTextContent(/Price changed/i)
    expect(screen.getByTestId("history-row-cover_changed")).toHaveTextContent(/Set cover photo/i)
    expect(screen.getByTestId("history-row-updated")).toHaveTextContent(/Edited this item/i)
    expect(screen.getByTestId("history-row-created")).toHaveTextContent(/Added this item/i)
  })

  it("renders status-only label when before is missing", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.events(SLUG, COMMODITY_ID, [
        {
          id: "e1",
          kind: "status_changed",
          occurred_at: "2026-04-25T10:30:00Z",
          after: { status: "sold" },
          meta: { actor },
        },
      ])
    )
    renderTimeline()
    expect(await screen.findByTestId("history-row-status_changed")).toHaveTextContent(
      /Marked as Sold/i
    )
  })

  it("renders kind-aware copy for loan lifecycle events", async () => {
    const rows: CommodityEventFixture[] = [
      {
        id: "l1",
        kind: "lent_out",
        occurred_at: "2026-05-01T10:30:00Z",
        after: {
          loan_id: "ln1",
          borrower_name: "Alice",
          lent_at: "2026-05-01",
          due_back_at: "2026-06-01",
        },
        meta: { actor },
      },
      {
        id: "l2",
        kind: "lent_out",
        occurred_at: "2026-04-15T10:00:00Z",
        // No due_back_at — open-ended loan path.
        after: { loan_id: "ln2", borrower_name: "Bob", lent_at: "2026-04-15" },
        meta: { actor },
      },
      {
        id: "l3",
        kind: "loan_updated",
        occurred_at: "2026-05-10T10:00:00Z",
        before: {
          loan_id: "ln1",
          borrower_name: "Alice",
          borrower_contact: "alice@old.example.com",
          borrower_note: "",
          due_back_at: "2026-06-01",
        },
        after: {
          loan_id: "ln1",
          borrower_name: "Alice",
          borrower_contact: "alice@new.example.com",
          borrower_note: "back office",
          due_back_at: "2026-06-01",
        },
        meta: { actor },
      },
      {
        id: "l4",
        kind: "returned",
        occurred_at: "2026-05-20T10:00:00Z",
        after: { loan_id: "ln1", returned_at: "2026-05-20" },
        meta: { actor },
      },
    ]
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.events(SLUG, COMMODITY_ID, rows)
    )

    renderTimeline()
    // First lent_out row carries borrower + due-back date.
    const lentOutRows = await screen.findAllByTestId("history-row-lent_out")
    expect(lentOutRows[0]).toHaveTextContent(/Lent out to Alice \(due back 2026-06-01\)/i)
    // Second lent_out row (open-ended) renders just the borrower name.
    expect(lentOutRows[1]).toHaveTextContent(/Lent out to Bob/i)
    expect(lentOutRows[1]).not.toHaveTextContent(/due back/i)
    // loan_updated lists the changed field labels.
    expect(screen.getByTestId("history-row-loan_updated")).toHaveTextContent(
      /Loan updated:.*borrower contact.*borrower note/i
    )
    // returned shows the returned_at date.
    expect(screen.getByTestId("history-row-returned")).toHaveTextContent(
      /Marked returned on 2026-05-20/i
    )
  })

  it("renders kind-aware copy for service lifecycle events", async () => {
    // Mirrors the loan lifecycle test: covers `sent_for_service` with
    // and without a reason, the `service_updated` field-diff path
    // (including the cost-pair gate), and `back_from_service` with a
    // returned_at. Locks the i18n keys (`sentForServiceLabel*`,
    // `backFromServiceLabelOn`, `serviceField.*`) against drift.
    const rows: CommodityEventFixture[] = [
      {
        id: "s1",
        kind: "sent_for_service",
        occurred_at: "2026-05-01T10:30:00Z",
        after: {
          service_id: "sv1",
          provider_name: "Apple Service",
          sent_at: "2026-05-01",
          reason: "screen replacement",
        },
        meta: { actor },
      },
      {
        id: "s2",
        kind: "sent_for_service",
        occurred_at: "2026-04-15T10:00:00Z",
        // No reason — generic-with-provider label.
        after: { service_id: "sv2", provider_name: "Bob's Repair Shop", sent_at: "2026-04-15" },
        meta: { actor },
      },
      {
        id: "s3",
        kind: "service_updated",
        occurred_at: "2026-05-10T10:00:00Z",
        before: {
          service_id: "sv1",
          provider_name: "Apple Service",
          reason: "screen replacement",
        },
        after: {
          service_id: "sv1",
          provider_name: "Apple Service",
          reason: "diagnostic + screen",
          cost_amount: "245",
          cost_currency: "EUR",
        },
        meta: { actor },
      },
      {
        id: "s4",
        kind: "back_from_service",
        occurred_at: "2026-05-20T10:00:00Z",
        after: { service_id: "sv1", provider_name: "Apple Service", returned_at: "2026-05-20" },
        meta: { actor },
      },
    ]
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.events(SLUG, COMMODITY_ID, rows)
    )

    renderTimeline()
    // sent_for_service with a reason renders the "for {{reason}}" copy.
    const sentRows = await screen.findAllByTestId("history-row-sent_for_service")
    expect(sentRows[0]).toHaveTextContent(/Sent to Apple Service for screen replacement/i)
    // The reason-less row falls through to the provider-only label —
    // i.e. it must NOT contain the "Sent to … for …" pattern.
    expect(sentRows[1]).toHaveTextContent(/Sent for service to Bob's Repair Shop/i)
    expect(sentRows[1]).not.toHaveTextContent(/Sent to .* for /i)
    // service_updated lists the changed field labels (reason + cost).
    expect(screen.getByTestId("history-row-service_updated")).toHaveTextContent(
      /Service updated:.*reason.*cost/i
    )
    // back_from_service shows the returned_at date.
    expect(screen.getByTestId("history-row-back_from_service")).toHaveTextContent(
      /Back from service on 2026-05-20/i
    )
  })

  it("renders cover-changed cleared label when after.cover_file_id is empty", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.events(SLUG, COMMODITY_ID, [
        {
          id: "e1",
          kind: "cover_changed",
          occurred_at: "2026-04-25T10:30:00Z",
          before: { cover_file_id: "f1" },
          after: { cover_file_id: "" },
          meta: { actor },
        },
      ])
    )
    renderTimeline()
    expect(await screen.findByTestId("history-row-cover_changed")).toHaveTextContent(
      /Cleared cover photo/i
    )
  })

  it("falls back to email when actor.name is empty, and shows nothing when actor missing", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.events(SLUG, COMMODITY_ID, [
        {
          id: "e1",
          kind: "created",
          occurred_at: "2026-04-25T10:30:00Z",
          after: { name: "X" },
          meta: { actor: { id: "u1", email: "fallback@example.com" } },
        },
        {
          id: "e2",
          kind: "updated",
          occurred_at: "2026-04-24T10:30:00Z",
          // No meta.actor at all — row should still render, just without
          // the trailing "by ..." suffix.
          after: { name: "Y" },
        },
      ])
    )
    renderTimeline()
    expect(await screen.findByTestId("history-row-created")).toHaveTextContent(
      /fallback@example.com/i
    )
    // The bare-no-actor row is present and doesn't crash; "by " is not appended.
    expect(screen.getByTestId("history-row-updated")).not.toHaveTextContent(/ by /i)
  })

  it("collapses after 10 rows behind a Show more toggle", async () => {
    const user = userEvent.setup()
    // 12 rows — first 10 visible, 2 hidden behind "Show more".
    const rows: CommodityEventFixture[] = Array.from({ length: 12 }).map((_, i) => ({
      id: `e${i}`,
      kind: "updated",
      occurred_at: `2026-04-${(20 - i).toString().padStart(2, "0")}T10:00:00Z`,
      after: { name: `v${i}` },
      meta: { actor },
    }))
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.events(SLUG, COMMODITY_ID, rows)
    )

    renderTimeline()
    await screen.findByTestId("history-show-more")
    expect(screen.getAllByTestId("history-row-updated")).toHaveLength(10)

    await user.click(screen.getByTestId("history-show-more"))
    expect(screen.getAllByTestId("history-row-updated")).toHaveLength(12)
    await waitFor(() => expect(screen.getByTestId("history-show-less")).toBeInTheDocument())

    await user.click(screen.getByTestId("history-show-less"))
    expect(screen.getAllByTestId("history-row-updated")).toHaveLength(10)
  })

  it("renders an error alert when the events endpoint fails", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      // Override events with a 500 — the handler module ships only the
      // happy-path fixture, so build the http handler inline.
      (await import("msw")).http.get(
        `*/api/v1/g/${encodeURIComponent(SLUG)}/commodities/${COMMODITY_ID}/events`,
        () => new Response("boom", { status: 500 })
      )
    )
    renderTimeline()
    expect(await screen.findByTestId("history-error")).toHaveTextContent(/Couldn't load activity/i)
  })

  it("is axe-clean when populated", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...areaHandlers.list(SLUG, areaFixture),
      ...commodityHandlers.events(SLUG, COMMODITY_ID, [
        {
          id: "e1",
          kind: "created",
          occurred_at: "2026-04-20T10:30:00Z",
          after: { name: "New" },
          meta: { actor },
        },
      ])
    )
    const { container } = renderTimeline()
    await screen.findByTestId("history-row-created")
    expect(await axe(container)).toHaveNoViolations()
  })
})
