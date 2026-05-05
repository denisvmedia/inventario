import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { Route } from "react-router-dom"
import { beforeAll, beforeEach, describe, expect, it } from "vitest"

import { ServiceTab } from "@/components/services/ServiceTab"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { groupHandlers, serviceHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const COMMODITY_ID = "commodity-1"
const groupFixture: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", main_currency: "USD" },
]

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  setCurrentGroupSlug(SLUG)
})

function renderTab() {
  return renderWithProviders({
    initialPath: `/g/${SLUG}/commodities/${COMMODITY_ID}`,
    routes: (
      <Route
        path="/g/:groupSlug/commodities/:id"
        element={
          <ConfirmProvider>
            <GroupProvider>
              <main>
                <ServiceTab commodityId={COMMODITY_ID} />
              </main>
            </GroupProvider>
          </ConfirmProvider>
        }
      />
    ),
  })
}

describe("<ServiceTab />", () => {
  it("renders the empty state + Send-for-service button when there are no service rows", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...serviceHandlers.listForCommodity(SLUG, COMMODITY_ID, [])
    )
    renderTab()
    expect(await screen.findByTestId("service-empty-state")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-detail-service-button")).toBeInTheDocument()
    expect(screen.queryByTestId("service-current")).toBeNull()
    expect(screen.queryByTestId("service-history")).toBeNull()
  })

  it("renders the current-service card when an open row exists", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...serviceHandlers.listForCommodity(SLUG, COMMODITY_ID, [
        {
          id: "svc-1",
          commodity_id: COMMODITY_ID,
          provider_name: "Apple Service",
          provider_contact: "+1 800-275-2273",
          reason: "screen replacement",
          sent_at: "2026-04-01",
          expected_return_at: "2026-04-15",
          returned_at: null,
        },
      ])
    )
    renderTab()
    const current = await screen.findByTestId("service-current")
    expect(current).toHaveTextContent("Apple Service")
    expect(current).toHaveTextContent("+1 800-275-2273")
    expect(current).toHaveTextContent("screen replacement")
    expect(screen.getByTestId("service-overdue-badge")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-detail-service-button")).toBeNull()
  })

  it("renders the history list for completed services", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...serviceHandlers.listForCommodity(SLUG, COMMODITY_ID, [
        {
          id: "svc-2",
          commodity_id: COMMODITY_ID,
          provider_name: "Bob's Repair Shop",
          reason: "battery swap",
          sent_at: "2026-03-01",
          returned_at: "2026-03-10",
        },
      ])
    )
    renderTab()
    expect(await screen.findByTestId("service-history")).toBeInTheDocument()
    expect(screen.getByTestId("service-history-row-svc-2")).toHaveTextContent(
      "Bob's Repair Shop"
    )
    expect(screen.getByTestId("service-empty-state")).toBeInTheDocument()
  })

  it("opens the SendForServiceDialog when the button is clicked", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...serviceHandlers.listForCommodity(SLUG, COMMODITY_ID, [])
    )
    const user = userEvent.setup()
    renderTab()
    await user.click(await screen.findByTestId("commodity-detail-service-button"))
    expect(await screen.findByTestId("service-dialog")).toBeVisible()
  })

  it("submits the dialog and fires the start-service request", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...serviceHandlers.listForCommodity(SLUG, COMMODITY_ID, []),
      ...serviceHandlers.startService(SLUG, COMMODITY_ID, {
        id: "svc-new",
        commodity_id: COMMODITY_ID,
        provider_name: "Apple Service",
        sent_at: "2026-05-05",
        returned_at: null,
      })
    )
    const user = userEvent.setup()
    renderTab()
    await user.click(await screen.findByTestId("commodity-detail-service-button"))
    await user.type(screen.getByTestId("service-provider-name"), "Apple Service")
    await user.click(screen.getByTestId("service-submit"))
    await waitFor(() => expect(screen.queryByTestId("service-dialog")).toBeNull())
  })

  it("marks the open service as returned via the confirm dialog", async () => {
    const openSvc = {
      id: "svc-open",
      commodity_id: COMMODITY_ID,
      provider_name: "Apple Service",
      sent_at: "2026-04-01",
      returned_at: null,
    }
    let returnHit = false
    server.use(
      ...groupHandlers.list(groupFixture),
      ...serviceHandlers.listForCommodity(SLUG, COMMODITY_ID, [openSvc]),
      ...serviceHandlers.returnService(SLUG, COMMODITY_ID, "svc-open", {
        ...openSvc,
        returned_at: "2026-05-05",
      })
    )
    const realFetch = window.fetch
    window.fetch = (...args: Parameters<typeof fetch>) => {
      const url = String(args[0])
      const init = args[1] as RequestInit | undefined
      if (init?.method === "POST" && url.includes("/services/svc-open/return")) {
        returnHit = true
      }
      return realFetch(...args)
    }
    try {
      const user = userEvent.setup()
      renderTab()
      await user.click(await screen.findByTestId("service-mark-returned"))
      await user.click(await screen.findByTestId("confirm-accept"))
      await waitFor(() => expect(returnHit).toBe(true))
    } finally {
      window.fetch = realFetch
    }
  })

  it("removes the service record via the confirm dialog", async () => {
    const openSvc = {
      id: "svc-open",
      commodity_id: COMMODITY_ID,
      provider_name: "Apple Service",
      sent_at: "2026-04-01",
      returned_at: null,
    }
    let deleteHit = false
    server.use(
      ...groupHandlers.list(groupFixture),
      ...serviceHandlers.listForCommodity(SLUG, COMMODITY_ID, [openSvc]),
      ...serviceHandlers.deleteService(SLUG, COMMODITY_ID, "svc-open")
    )
    const realFetch = window.fetch
    window.fetch = (...args: Parameters<typeof fetch>) => {
      const url = String(args[0])
      const init = args[1] as RequestInit | undefined
      if (init?.method === "DELETE" && url.includes("/services/svc-open")) {
        deleteHit = true
      }
      return realFetch(...args)
    }
    try {
      const user = userEvent.setup()
      renderTab()
      await user.click(await screen.findByTestId("service-delete"))
      await user.click(await screen.findByTestId("confirm-accept"))
      await waitFor(() => expect(deleteHit).toBe(true))
    } finally {
      window.fetch = realFetch
    }
  })

  it("is axe-clean once data has loaded", async () => {
    server.use(
      ...groupHandlers.list(groupFixture),
      ...serviceHandlers.listForCommodity(SLUG, COMMODITY_ID, [])
    )
    const { baseElement } = renderTab()
    await screen.findByTestId("service-empty-state")
    await waitFor(async () => {
      const results = await axe(baseElement)
      expect(results).toHaveNoViolations()
    })
  })
})
