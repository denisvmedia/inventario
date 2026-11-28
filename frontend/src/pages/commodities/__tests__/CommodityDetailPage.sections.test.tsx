import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import { CommodityDetailPage } from "@/pages/commodities/CommodityDetailPage"
import { GroupProvider } from "@/features/group/GroupContext"
import type { GroupSection } from "@/features/group/sections"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { WhenRoleKnown } from "@/test/role-known"
import { server } from "@/test/server"
import {
  apiUrl,
  areaHandlers,
  commodityHandlers,
  groupHandlers,
  loanHandlers,
  maintenanceHandlers,
  memberHandlers,
  serviceHandlers,
} from "@/test/handlers"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const SLUG = "household"
const ID = "c1"

function item(attrs: Record<string, unknown> = {}) {
  return {
    id: ID,
    type: "commodities",
    attributes: {
      id: ID,
      name: "Cordless drill",
      short_name: "Drill",
      type: "equipment",
      area_id: "a1",
      status: "in_use",
      count: 1,
      draft: false,
      ...attrs,
    },
  }
}

type ItemData = {
  loans?: Parameters<typeof loanHandlers.listForCommodity>[2]
  services?: Parameters<typeof serviceHandlers.listForCommodity>[2]
  schedules?: Parameters<typeof maintenanceHandlers.listForCommodity>[2]
}

// The caller is a member with the user role, who may record loans, services
// and maintenance (#3038).
function serve(hidden: GroupSection[], attrs: Record<string, unknown> = {}, data: ItemData = {}) {
  server.use(
    ...memberHandlers.asRole("g1", "user"),
    ...groupHandlers.list([
      { id: "g1", slug: SLUG, name: "Household", group_currency: "USD", hidden_sections: hidden },
    ]),
    ...areaHandlers.list(SLUG, [
      { id: "a1", type: "areas", attributes: { id: "a1", name: "Garage", location_id: "l1" } },
    ]),
    ...commodityHandlers.detail(SLUG, ID, item(attrs)),
    ...loanHandlers.listForCommodity(SLUG, ID, data.loans ?? []),
    ...serviceHandlers.listForCommodity(SLUG, ID, data.services ?? []),
    ...maintenanceHandlers.listForCommodity(SLUG, ID, data.schedules ?? [])
  )
}

function renderDetail(search = "") {
  setAccessToken("good-token")
  return renderWithProviders({
    withAuth: true,
    initialPath: `/g/${SLUG}/commodities/${ID}${search}`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id"
        element={
          <GroupProvider>
            <ConfirmProvider>
              <WhenRoleKnown>
                <CommodityDetailPage />
              </WhenRoleKnown>
            </ConfirmProvider>
          </GroupProvider>
        }
      />
    ),
  })
}

async function tabKeys(): Promise<string[]> {
  const tabs = await screen.findByTestId("commodity-detail-tabs")
  return within(tabs)
    .getAllByRole("tab")
    .map((tab) => (tab.getAttribute("data-testid") ?? "").replace("commodity-detail-tab-", ""))
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<CommodityDetailPage /> in a group that hides sections (#1607)", () => {
  it("shows every tab while the group hides nothing", async () => {
    serve([])
    renderDetail()
    await screen.findByTestId("commodity-detail-details")
    expect(await tabKeys()).toEqual([
      "details",
      "warranty",
      "files",
      "lend",
      "service",
      "supplies",
      "maintenance",
    ])
    // An item without a warranty says so while warranties show.
    expect(screen.getByTestId("commodity-detail-warranty-pill")).toHaveAttribute(
      "data-status",
      "none"
    )
  })

  it("drops the tabs of hidden sections for an item without data in them", async () => {
    serve(["warranties", "maintenance", "loans"])
    renderDetail()
    await screen.findByTestId("commodity-detail-details")
    await waitFor(async () => expect(await tabKeys()).toEqual(["details", "files", "supplies"]))
    // No "No warranty" pill either.
    expect(screen.queryByTestId("commodity-detail-warranty-pill")).not.toBeInTheDocument()
  })

  it("opens Details for a link to the tab of a hidden section", async () => {
    serve(["warranties"])
    renderDetail("?tab=warranty")
    expect(await screen.findByTestId("commodity-detail-details")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-detail-warranty")).not.toBeInTheDocument()
  })

  it("still shows the warranty an item holds", async () => {
    const user = userEvent.setup()
    serve(["warranties"], { warranty_expires_at: "2099-03-01", warranty_notes: "Shop warranty" })
    renderDetail()
    await screen.findByTestId("commodity-detail-details")
    expect(screen.getByTestId("commodity-detail-warranty-pill")).toHaveAttribute(
      "data-status",
      "active"
    )
    await user.click(screen.getByTestId("commodity-detail-tab-warranty"))
    expect(await screen.findByTestId("commodity-detail-warranty-notes")).toHaveTextContent(
      "Shop warranty"
    )
  })

  it("still shows the loans an item has, without the Lend button", async () => {
    const user = userEvent.setup()
    serve(
      ["loans"],
      {},
      {
        loans: [
          {
            id: "l1",
            commodity_id: ID,
            borrower_name: "Pavel",
            lent_at: "2026-05-10",
            due_back_at: null,
            returned_at: null,
          },
        ],
      }
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-details")
    await user.click(await screen.findByTestId("commodity-detail-tab-lend"))
    expect(await screen.findByTestId("lend-current")).toHaveTextContent("Pavel")
    // The open loan can still be closed; a new one cannot be started.
    expect(screen.getByTestId("lend-mark-returned")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-detail-lend-button")).not.toBeInTheDocument()
  })

  it("still shows the maintenance schedules an item has, without Add schedule", async () => {
    const user = userEvent.setup()
    serve(
      ["maintenance"],
      {},
      {
        schedules: [
          {
            id: "s1",
            commodity_id: ID,
            title: "Oil the chuck",
            interval_days: 90,
            next_due_at: "2099-01-01",
            enabled: true,
          },
        ],
      }
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-details")
    await user.click(await screen.findByTestId("commodity-detail-tab-maintenance"))
    expect(await screen.findByTestId("maintenance-list")).toHaveTextContent("Oil the chuck")
    expect(screen.queryByTestId("maintenance-add")).not.toBeInTheDocument()
    // The item has no service record, so that tab stays out.
    expect(screen.queryByTestId("commodity-detail-tab-service")).not.toBeInTheDocument()
  })

  it("keeps the tab a link opens while its data loads", async () => {
    let release = () => {}
    const gate = new Promise<void>((resolve) => {
      release = resolve
    })
    serve(["loans"])
    server.use(
      http.get(apiUrl(`/g/${SLUG}/commodities/${ID}/loans`), async () => {
        await gate
        return HttpResponse.json({
          data: [
            {
              id: "l1",
              commodity_id: ID,
              borrower_name: "Pavel",
              lent_at: "2026-05-10",
              due_back_at: null,
              returned_at: null,
            },
          ],
          meta: { loans: 1, total: 1 },
        })
      })
    )
    renderDetail("?tab=lend")
    const lendTab = await screen.findByTestId("commodity-detail-tab-lend")
    expect(lendTab).toHaveAttribute("aria-selected", "true")
    expect(screen.queryByTestId("commodity-detail-details")).not.toBeInTheDocument()

    release()
    expect(await screen.findByTestId("lend-current")).toHaveTextContent("Pavel")
    expect(screen.getByTestId("commodity-detail-tab-lend")).toHaveAttribute("aria-selected", "true")
  })

  it("keeps the tabs whose data could not be read", async () => {
    serve(["loans", "maintenance"])
    server.use(
      http.get(apiUrl(`/g/${SLUG}/commodities/${ID}/loans`), () =>
        HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 })
      ),
      http.get(apiUrl(`/g/${SLUG}/commodities/${ID}/maintenance`), () =>
        HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 })
      )
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-details")
    // The service records loaded empty, so that tab goes; the others stay.
    await waitFor(() =>
      expect(screen.queryByTestId("commodity-detail-tab-service")).not.toBeInTheDocument()
    )
    expect(screen.getByTestId("commodity-detail-tab-lend")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-detail-tab-maintenance")).toBeInTheDocument()
  })

  it("still shows the service records an item has, without Send for service", async () => {
    const user = userEvent.setup()
    serve(
      ["maintenance"],
      {},
      {
        services: [
          {
            id: "svc-1",
            commodity_id: ID,
            provider_name: "Tool repair shop",
            sent_at: "2026-03-01",
            returned_at: "2026-03-10",
          },
        ],
      }
    )
    renderDetail()
    await screen.findByTestId("commodity-detail-details")
    await user.click(await screen.findByTestId("commodity-detail-tab-service"))
    expect(await screen.findByTestId("service-history-row-svc-1")).toHaveTextContent(
      "Tool repair shop"
    )
    expect(screen.queryByTestId("commodity-detail-service-button")).not.toBeInTheDocument()
  })
})
