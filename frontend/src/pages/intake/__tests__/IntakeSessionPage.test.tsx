import { beforeEach, describe, expect, it } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { IntakeSessionPage } from "@/pages/intake/IntakeSessionPage"
import { toastSpies } from "@/test/global-mocks"
import {
  apiUrl,
  authHandlers,
  customFieldHandlers,
  groupHandlers,
  intakeHandlers,
} from "@/test/handlers"
import { receiptSession } from "@/test/handlers/intake"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import type { Schema } from "@/types"

const SLUG = "household"
const groups: Schema<"models.LocationGroup">[] = [
  { id: "g1", slug: SLUG, name: "Household", group_currency: "USD" },
]

function members(role: "viewer" | "user") {
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

function renderPage(id: string) {
  return renderWithProviders({
    initialPath: `/g/${SLUG}/intake/${id}`,
    routes: (
      <Route
        path="/g/:groupSlug/intake/:id"
        element={
          <AuthProvider>
            <ConfirmProvider>
              <GroupProvider>
                <IntakeSessionPage />
              </GroupProvider>
            </ConfirmProvider>
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
  setAccessToken("good-token")
  setCurrentGroupSlug(SLUG)
  toastSpies.success.mockClear()
  toastSpies.error.mockClear()
})

describe("<IntakeSessionPage />", () => {
  it("lists each purchase with its products and why a file failed", async () => {
    const session = receiptSession()
    const fake = intakeHandlers.backend(SLUG, [session])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    renderPage(session.id)
    const receipt = await screen.findByTestId(`intake-source-${session.id}-receipt`)
    expect(within(receipt).getByText("Espresso Machine")).toBeInTheDocument()
    expect(within(receipt).getByText("Milk Frother")).toBeInTheDocument()
    expect(within(receipt).getByText("Espresso Cups")).toBeInTheDocument()
    expect(within(receipt).getByTestId(`intake-quantity-${session.id}-c2`)).toHaveValue(2)
    expect(screen.getByTestId(`intake-source-status-${session.id}-photo`)).toHaveTextContent(
      "The file could not be read."
    )
    expect(await screen.findByTestId("intake-create-drafts")).toHaveTextContent("Create 3 drafts")
  })

  it("creates drafts for the kept products only and reports the outcome", async () => {
    const session = receiptSession()
    const fake = intakeHandlers.backend(SLUG, [session])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    const user = userEvent.setup()
    renderPage(session.id)
    const frother = `${session.id}-c1`
    await user.click(await screen.findByTestId(`intake-keep-${frother}`))
    await waitFor(() =>
      expect(fake.reviews).toEqual([{ id: frother, attributes: { selected: false } }])
    )
    await waitFor(() =>
      expect(screen.getByTestId("intake-create-drafts")).toHaveTextContent("Create 2 drafts")
    )
    await user.click(screen.getByTestId("intake-create-drafts"))
    await waitFor(() =>
      expect(toastSpies.success).toHaveBeenCalledWith(
        "2 drafts created, 1 file unreadable",
        undefined
      )
    )
    expect(fake.sessions[0]?.candidates.map((c) => c.outcome)).toEqual([
      "created",
      "skipped",
      "created",
    ])
    expect(await screen.findByTestId(`intake-continue-${session.id}-c0`)).toHaveAttribute(
      "href",
      `/g/${SLUG}/commodities/new?draft=draft-${session.id}-c0`
    )
    expect(screen.queryByTestId(`intake-continue-${frother}`)).not.toBeInTheDocument()
    expect(screen.queryByTestId("intake-footer")).not.toBeInTheDocument()
    // A purchase that has drafts cannot be scanned again; the failed one can.
    expect(screen.queryByTestId(`intake-rescan-${session.id}-receipt`)).not.toBeInTheDocument()
    expect(screen.getByTestId(`intake-rescan-${session.id}-photo`)).toBeInTheDocument()
  })

  it("creates the drafts only once the choices being saved have reached the server", async () => {
    const session = receiptSession()
    const fake = intakeHandlers.backend(SLUG, [session])
    fake.reviewDelayMs = 100
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    const user = userEvent.setup()
    renderPage(session.id)
    const machine = `${session.id}-c0`
    const create = await screen.findByTestId("intake-create-drafts")

    // The button stays enabled while a choice is saved: a button disabled
    // between the press and the release would lose the click.
    await user.click(screen.getByTestId(`intake-keep-${session.id}-c1`))
    expect(create).toBeEnabled()

    // A quantity is saved when its field loses focus, which the click on
    // the button does: the drafts are requested after both saves.
    const quantity = screen.getByTestId(`intake-quantity-${machine}`)
    await user.clear(quantity)
    await user.type(quantity, "3")
    await user.click(create)

    // Until the drafts are made, the button and the rescan wait.
    expect(create).toBeDisabled()
    expect(screen.getByTestId(`intake-rescan-${session.id}-photo`)).toBeDisabled()
    await waitFor(() => expect(fake.draftRequests).toBe(1))
    expect(fake.events).toEqual([`review ${session.id}-c1`, `review ${machine}`, "drafts"])
    expect(fake.sessions[0]?.candidates[0]?.quantity).toBe(3)
    await waitFor(() =>
      expect(screen.getByTestId(`intake-rescan-${session.id}-photo`)).toBeEnabled()
    )
  })

  it("says why a file is not scanned again and shows the drafts it already has", async () => {
    const session = receiptSession()
    const fake = intakeHandlers.backend(SLUG, [session])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    const user = userEvent.setup()
    renderPage(session.id)

    // Drafts were made by a request whose answer this page never got.
    fake.refuseRescan = "has_drafts"
    await user.click(await screen.findByTestId(`intake-rescan-${session.id}-receipt`))
    await waitFor(() =>
      expect(toastSpies.error).toHaveBeenCalledWith(
        "Not scanned again: drafts were already made from this file. The page now shows them.",
        undefined
      )
    )
    expect(await screen.findByTestId(`intake-continue-${session.id}-c0`)).toBeInTheDocument()
    expect(screen.queryByTestId(`intake-rescan-${session.id}-receipt`)).not.toBeInTheDocument()

    fake.refuseRescan = "busy"
    await user.click(screen.getByTestId(`intake-rescan-${session.id}-photo`))
    await waitFor(() =>
      expect(toastSpies.error).toHaveBeenCalledWith(
        "Not scanned again: the file is still being scanned. Try again when the scan has finished.",
        undefined
      )
    )
  })

  it("saves which fields a draft takes", async () => {
    const session = receiptSession()
    const fake = intakeHandlers.backend(SLUG, [session])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    const user = userEvent.setup()
    renderPage(session.id)
    const machine = `${session.id}-c0`
    await user.click(await screen.findByTestId(`intake-fields-toggle-${machine}`))
    await user.click(screen.getByTestId(`intake-field-${machine}-original_price`))
    await waitFor(() =>
      expect(fake.reviews).toEqual([
        { id: machine, attributes: { accepted_fields: ["name", "original_price_currency"] } },
      ])
    )
  })

  it("shows the custom field values the scan read and saves which ones a draft takes", async () => {
    const session = receiptSession()
    const machine = session.candidates[0]
    machine.custom_fields = {
      "cf-card": { value: "WC-123", confidence: 0.9 },
      "cf-condition": { value: "opt-used", confidence: 0.6 },
      "cf-gift": { value: true, confidence: 0.2 },
      // A field deleted since the scan is not shown.
      "cf-gone": { value: "x", confidence: 0.9 },
      // Nor is an option removed since the scan.
      "cf-finish": { value: "opt-removed", confidence: 0.9 },
    }
    machine.accepted_custom_fields = ["cf-card", "cf-condition", "cf-gone", "cf-finish"]
    const fake = intakeHandlers.backend(SLUG, [session])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...customFieldHandlers.list(SLUG, [
        customFieldHandlers.field({ id: "cf-card", name: "Warranty card number" }),
        customFieldHandlers.field({
          id: "cf-condition",
          name: "Condition",
          type: "select",
          options: [
            { id: "opt-mint", label: "Mint" },
            { id: "opt-used", label: "Used" },
          ],
        }),
        customFieldHandlers.field({ id: "cf-gift", name: "Gift", type: "boolean" }),
        customFieldHandlers.field({ id: "cf-unread", name: "Weight", type: "number" }),
        customFieldHandlers.field({
          id: "cf-finish",
          name: "Finish",
          type: "select",
          options: [{ id: "opt-matte", label: "Matte" }],
        }),
      ]),
      ...fake.handlers
    )
    const user = userEvent.setup()
    renderPage(session.id)
    const toggle = await screen.findByTestId(`intake-fields-toggle-${machine.id}`)
    // Three item fields and the two custom fields that are shown.
    await waitFor(() => expect(toggle).toHaveTextContent("5 fields used"))
    await user.click(toggle)

    const list = screen.getByTestId(`intake-fields-${machine.id}`)
    expect(within(list).getByText("Warranty card number:")).toBeInTheDocument()
    expect(within(list).getByText("WC-123")).toBeInTheDocument()
    expect(within(list).getByText("Used")).toBeInTheDocument()
    expect(within(list).getByText("Yes")).toBeInTheDocument()
    expect(within(list).queryByText("Weight:")).not.toBeInTheDocument()
    expect(within(list).queryByText("Finish:")).not.toBeInTheDocument()
    const gift = screen.getByTestId(`intake-custom-field-${machine.id}-cf-gift`)
    expect(gift).not.toBeChecked()
    expect(screen.getByTestId(`intake-custom-field-${machine.id}-cf-card`)).toBeChecked()

    await user.click(gift)
    await waitFor(() =>
      expect(fake.reviews).toEqual([
        {
          id: machine.id,
          attributes: { accepted_custom_fields: ["cf-card", "cf-condition", "cf-gift"] },
        },
      ])
    )
  })

  it("stops a batch that is still scanning and resumes it", async () => {
    const session = receiptSession()
    session.sources[1] = { ...session.sources[1]!, status: "pending", error_code: undefined }
    const fake = intakeHandlers.backend(SLUG, [session])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    const user = userEvent.setup()
    renderPage(session.id)
    expect(await screen.findByTestId("intake-processing-note")).toBeInTheDocument()
    await user.click(await screen.findByTestId("intake-cancel"))
    await waitFor(() => expect(fake.sessions[0]?.sources[1]?.status).toBe("canceled"))
    await user.click(await screen.findByTestId("intake-resume"))
    await waitFor(() => expect(fake.sessions[0]?.sources[1]?.status).toBe("pending"))
  })

  it("shows a viewer the batch without actions", async () => {
    const session = receiptSession()
    const fake = intakeHandlers.backend(SLUG, [session])
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("viewer"),
      ...fake.handlers
    )
    renderPage(session.id)
    expect(await screen.findByText("Espresso Machine")).toBeInTheDocument()
    await waitFor(() => expect(screen.getByTestId(`intake-keep-${session.id}-c0`)).toBeDisabled())
    expect(screen.queryByTestId("intake-create-drafts")).not.toBeInTheDocument()
    expect(screen.queryByTestId("intake-delete")).not.toBeInTheDocument()
  })

  it("says when the batch is gone", async () => {
    const fake = intakeHandlers.backend(SLUG)
    server.use(
      ...authHandlers.signedIn(),
      ...groupHandlers.list(groups),
      members("user"),
      ...fake.handlers
    )
    renderPage("missing")
    expect(await screen.findByTestId("page-intake-not-found")).toHaveTextContent("Batch not found")
  })
})
