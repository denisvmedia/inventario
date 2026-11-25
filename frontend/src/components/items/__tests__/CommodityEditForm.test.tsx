import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { axe } from "jest-axe"
import { http, HttpResponse } from "msw"

import { CommodityEditForm } from "@/components/items/CommodityEditForm"
import type { Commodity } from "@/features/commodities/api"
import { GroupProvider } from "@/features/group/GroupContext"
import type { ListNavigation } from "@/hooks/useListNavigation"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { HttpError, __resetHttpForTests } from "@/lib/http"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { apiUrl, groupHandlers, tagHandlers } from "@/test/handlers"

const SLUG = "household"

const commodity: Commodity = {
  id: "c1",
  name: "Desk lamp",
  short_name: "Lamp",
  type: "furniture",
  area_id: "a1",
  status: "in_use",
  count: 1,
  draft: false,
  purchase_date: "2024-03-01",
  original_price: 40,
  original_price_currency: "USD",
  // An array at runtime; the generated type says string (see draft.ts).
  urls: ["https://example.com/lamp"] as unknown as string,
}

const noList: ListNavigation = {
  active: false,
  loading: false,
  error: false,
  inList: false,
  position: null,
  total: null,
  page: 1,
  previous: null,
  next: null,
}

function renderForm(
  overrides: Partial<Commodity> = {},
  props: { onSubmit?: (req: unknown) => Promise<unknown> } = {}
) {
  setAccessToken("good-token")
  server.use(
    ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household", group_currency: "USD" }]),
    ...tagHandlers.list(SLUG, []),
    http.get(apiUrl("/currencies"), () => HttpResponse.json(["USD", "EUR"]))
  )
  const handlers = {
    onSubmit: vi.fn(props.onSubmit ?? (async () => ({}))),
    onExit: vi.fn(),
    onGoTo: vi.fn(),
    onAllFields: vi.fn(),
  }
  const utils = renderWithProviders({
    initialPath: `/g/${SLUG}/x`,
    routes: (
      <Route
        path="/g/:groupSlug/x"
        element={
          <GroupProvider>
            <CommodityEditForm
              commodity={{ ...commodity, ...overrides }}
              areas={[{ id: "a1", name: "Study", location_id: "l1" }]}
              locations={[{ id: "l1", name: "Home" }]}
              groupCurrency="USD"
              nav={noList}
              hrefFor={() => "#"}
              {...handlers}
            />
          </GroupProvider>
        }
      />
    ),
  })
  return { ...utils, ...handlers }
}

function validationError(attributes: Record<string, string>) {
  return new HttpError("invalid", 422, "/commodities/c1", {
    errors: [
      {
        status: "Unprocessable Entity",
        error: { type: "validation.Errors", error: { data: { attributes } } },
      },
    ],
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<CommodityEditForm />", () => {
  it("lays out the mock's sections with the item's values", async () => {
    renderForm()
    expect(await screen.findByLabelText(/^name/i)).toHaveValue("Desk lamp")
    for (const section of ["Identity", "Location", "Purchase", "Warranty", "Notes & Tags"]) {
      expect(screen.getByText(section, { selector: "p" })).toBeInTheDocument()
    }
    expect(screen.getByLabelText(/purchase date/i)).toHaveValue("2024-03-01")
    expect(screen.getByRole("combobox", { name: /^location/i })).toHaveTextContent("Home")
    expect(screen.getByRole("combobox", { name: /^area/i })).toHaveTextContent("Study")
  })

  it("leaves at once when nothing changed", async () => {
    const user = userEvent.setup()
    const { onExit, onSubmit } = renderForm()
    await user.click(await screen.findByTestId("commodity-edit-cancel"))
    expect(onExit).toHaveBeenCalledTimes(1)
    await user.click(screen.getByTestId("commodity-edit-save"))
    // Save with no changes leaves too, without a request.
    expect(onExit).toHaveBeenCalledTimes(2)
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("asks before dropping unsaved edits", async () => {
    const user = userEvent.setup()
    const { onExit, onSubmit } = renderForm()
    await user.type(await screen.findByLabelText(/^name/i), " (old)")
    await user.click(screen.getByTestId("commodity-edit-back"))
    expect(await screen.findByTestId("unsaved-changes-dialog")).toBeInTheDocument()
    expect(onExit).not.toHaveBeenCalled()
    await user.click(screen.getByTestId("unsaved-changes-discard"))
    expect(onExit).toHaveBeenCalledTimes(1)
    expect(onSubmit).not.toHaveBeenCalled()
  })

  it("keeps the fields it does not show when saving", async () => {
    const user = userEvent.setup()
    const { onSubmit, onExit } = renderForm()
    await user.type(await screen.findByLabelText(/^notes/i), "Bulb is E27")
    await user.click(screen.getByTestId("commodity-edit-save"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      comments: "Bulb is E27",
      urls: ["https://example.com/lamp"],
      status: "in_use",
    })
    await waitFor(() => expect(onExit).toHaveBeenCalledTimes(1))
  })

  it("points to all fields when the server rejects one this form does not show", async () => {
    const user = userEvent.setup()
    const { onAllFields, onExit } = renderForm(
      {},
      {
        onSubmit: async () => {
          throw validationError({ urls: "must be a valid URL" })
        },
      }
    )
    const name = await screen.findByLabelText(/^name/i)
    await user.clear(name)
    await user.type(name, "Reading lamp")
    await user.click(screen.getByTestId("commodity-edit-save"))
    expect(await screen.findByTestId("commodity-edit-hidden-error")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-edit-error")).toBeNull()
    expect(onExit).not.toHaveBeenCalled()
    await user.click(screen.getByTestId("commodity-edit-all-fields"))
    // The dialog opens with what was typed here.
    expect(onAllFields).toHaveBeenCalledWith(expect.objectContaining({ name: "Reading lamp" }))
  })

  it("shows a server message under the field it belongs to", async () => {
    const user = userEvent.setup()
    renderForm(
      {},
      {
        onSubmit: async () => {
          throw validationError({ serial_number: "is already taken" })
        },
      }
    )
    await user.type(await screen.findByLabelText(/serial number/i), "SN-1")
    await user.click(screen.getByTestId("commodity-edit-save"))
    expect(await screen.findByText("is already taken")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-edit-hidden-error")).toBeNull()
  })

  it("disables an empty warranty for a bundle", async () => {
    renderForm({ count: 6 })
    expect(await screen.findByLabelText(/expiry date/i)).toBeDisabled()
    expect(screen.getByLabelText(/warranty notes/i)).toBeDisabled()
  })

  it("has no axe violations", async () => {
    const { container } = renderForm()
    await screen.findByLabelText(/^name/i)
    expect(await axe(container)).toHaveNoViolations()
  })
})
