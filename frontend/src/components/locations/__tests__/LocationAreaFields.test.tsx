import { useState, type FormEvent } from "react"
import { beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { fireEvent, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import {
  LocationAreaFields,
  type AreaOption,
  type LocationOption,
} from "@/components/locations/LocationAreaFields"
import { GroupProvider } from "@/features/group/GroupContext"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import {
  apiUrl,
  areaHandlers,
  authHandlers,
  groupHandlers,
  locationHandlers,
  memberHandlers,
} from "@/test/handlers"

const SLUG = "household"
const GROUP_ID = "g1"

const locations: LocationOption[] = [{ id: "l1", name: "Home" }]
const areas: AreaOption[] = [{ id: "a1", name: "Study", location_id: "l1" }]

interface RenderOptions {
  role?: string
  initialAreaId?: string
}

// Mounts the picker inside a form of its own, as the item forms do, with a
// notes field standing for the rest of the form's input. The form sits in a
// file drop zone, as the item page is one.
function renderPicker({ role = "admin", initialAreaId = "" }: RenderOptions = {}) {
  setAccessToken("good-token")
  server.use(
    ...authHandlers.signedIn(),
    ...groupHandlers.list([{ id: GROUP_ID, slug: SLUG, name: "Household", group_currency: "USD" }]),
    ...memberHandlers.list(GROUP_ID, [{ id: "m1", attributes: { member_user_id: "u1", role } }])
  )
  const onAreaChange = vi.fn()
  const onOuterSubmit = vi.fn((e: FormEvent) => e.preventDefault())
  const onOuterDrop = vi.fn()

  function Host() {
    const [areaId, setAreaId] = useState(initialAreaId)
    return (
      <div onDragOver={onOuterDrop} onDrop={onOuterDrop}>
        <form onSubmit={onOuterSubmit} aria-label="Item">
          <label htmlFor="notes">Notes</label>
          <input id="notes" />
          <LocationAreaFields
            areas={areas}
            locations={locations}
            areaId={areaId}
            onAreaChange={(next) => {
              setAreaId(next)
              onAreaChange(next)
            }}
            idPrefix="test"
          />
        </form>
      </div>
    )
  }

  const utils = renderWithProviders({
    withAuth: true,
    initialPath: `/g/${SLUG}/x`,
    routes: (
      <Route
        path="/g/:groupSlug/x"
        element={
          <GroupProvider>
            <Host />
          </GroupProvider>
        }
      />
    ),
  })
  return { ...utils, onAreaChange, onOuterSubmit, onOuterDrop }
}

async function openSelect(user: ReturnType<typeof userEvent.setup>, name: RegExp) {
  const trigger = await screen.findByRole("combobox", { name })
  await user.click(trigger)
  return screen.findByRole("listbox")
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<LocationAreaFields />", () => {
  it("offers no create entries to a member who may not manage locations", async () => {
    const user = userEvent.setup()
    renderPicker({ role: "user", initialAreaId: "a1" })
    // Wait for the role to load: the area of the item shows once the
    // group context is in.
    expect(await screen.findByRole("combobox", { name: /^area/i })).toHaveTextContent("Study")
    const listbox = await openSelect(user, /^location/i)
    expect(within(listbox).getByRole("option", { name: "Home" })).toBeInTheDocument()
    expect(within(listbox).queryByRole("option", { name: /new location/i })).toBeNull()
  })

  it("creates a location and an area under it in one flow", async () => {
    const user = userEvent.setup()
    let locationBody: unknown
    let areaBody: unknown
    server.use(
      http.post(apiUrl(`/g/${SLUG}/locations`), async ({ request }) => {
        locationBody = await request.json()
        return HttpResponse.json(
          { data: { id: "l2", type: "locations", attributes: { name: "Cottage" } } },
          { status: 201 }
        )
      }),
      http.post(apiUrl(`/g/${SLUG}/areas`), async ({ request }) => {
        areaBody = await request.json()
        return HttpResponse.json(
          {
            data: {
              id: "a2",
              type: "areas",
              attributes: { name: "Attic", location_id: "l2" },
            },
          },
          { status: 201 }
        )
      })
    )
    const { onAreaChange, onOuterSubmit } = renderPicker()
    await user.type(await screen.findByLabelText("Notes"), "Bought at the fair")

    let listbox = await openSelect(user, /^location/i)
    await user.click(await within(listbox).findByRole("option", { name: /new location/i }))
    const locationDialog = await screen.findByTestId("location-form-dialog")
    await user.type(within(locationDialog).getByTestId("location-name-input"), "Cottage")
    // Enter submits the dialog's own form, not the one around the picker.
    await user.keyboard("{Enter}")

    await waitFor(() => expect(screen.queryByTestId("location-form-dialog")).toBeNull())
    expect(locationBody).toMatchObject({ data: { attributes: { name: "Cottage" } } })
    const locationTrigger = screen.getByRole("combobox", { name: /^location/i })
    expect(locationTrigger).toHaveTextContent("Cottage")
    // Focus is back on the select the dialog was opened from.
    await waitFor(() => expect(locationTrigger).toHaveFocus())

    listbox = await openSelect(user, /^area/i)
    expect(within(listbox).queryByRole("option", { name: "Study" })).toBeNull()
    await user.click(await within(listbox).findByRole("option", { name: /new area/i }))
    const areaDialog = await screen.findByTestId("area-form-dialog")
    await user.type(within(areaDialog).getByTestId("area-name-input"), "Attic")
    await user.click(within(areaDialog).getByTestId("area-form-submit"))

    await waitFor(() => expect(screen.queryByTestId("area-form-dialog")).toBeNull())
    expect(areaBody).toMatchObject({ data: { attributes: { name: "Attic", location_id: "l2" } } })
    expect(screen.getByRole("combobox", { name: /^area/i })).toHaveTextContent("Attic")
    expect(onAreaChange).toHaveBeenLastCalledWith("a2")
    expect(screen.getByLabelText("Notes")).toHaveValue("Bought at the fair")
    expect(onOuterSubmit).not.toHaveBeenCalled()
  })

  it("keeps the typed name and shows the error when the create fails", async () => {
    const user = userEvent.setup()
    server.use(
      http.post(apiUrl(`/g/${SLUG}/locations`), () =>
        HttpResponse.json({ error: "Group admin access required" }, { status: 403 })
      )
    )
    const { onAreaChange } = renderPicker({ initialAreaId: "a1" })
    expect(await screen.findByRole("combobox", { name: /^area/i })).toHaveTextContent("Study")

    const listbox = await openSelect(user, /^location/i)
    await user.click(await within(listbox).findByRole("option", { name: /new location/i }))
    const dialog = await screen.findByTestId("location-form-dialog")
    await user.type(within(dialog).getByTestId("location-name-input"), "Cottage")
    await user.click(within(dialog).getByTestId("location-form-submit"))

    expect(await within(dialog).findByTestId("location-form-server-error")).toBeInTheDocument()
    expect(within(dialog).getByTestId("location-name-input")).toHaveValue("Cottage")
    // Nothing changed behind the dialog.
    expect(onAreaChange).not.toHaveBeenCalled()
    expect(screen.getByRole("combobox", { name: /^location/i, hidden: true })).toHaveTextContent(
      "Home"
    )
  })

  it("leaves the selection alone when the dialog is canceled", async () => {
    const user = userEvent.setup()
    server.use(...locationHandlers.create(SLUG, { id: "l9", attributes: { name: "Unused" } }))
    server.use(...areaHandlers.create(SLUG, { id: "a9", attributes: { name: "Unused" } }))
    const { onAreaChange } = renderPicker({ initialAreaId: "a1" })
    expect(await screen.findByRole("combobox", { name: /^area/i })).toHaveTextContent("Study")

    const listbox = await openSelect(user, /^area/i)
    await user.click(await within(listbox).findByRole("option", { name: /new area/i }))
    const dialog = await screen.findByTestId("area-form-dialog")
    await user.click(within(dialog).getByTestId("area-form-cancel"))

    await waitFor(() => expect(screen.queryByTestId("area-form-dialog")).toBeNull())
    expect(screen.getByRole("combobox", { name: /^area/i })).toHaveTextContent("Study")
    expect(onAreaChange).not.toHaveBeenCalled()
  })

  it("does not select a location whose create ends after its dialog closed", async () => {
    const user = userEvent.setup()
    let respond!: () => void
    const responded = new Promise<void>((resolve) => {
      respond = resolve
    })
    server.use(
      http.post(apiUrl(`/g/${SLUG}/locations`), async () => {
        await responded
        return HttpResponse.json(
          { data: { id: "l2", type: "locations", attributes: { name: "Cottage" } } },
          { status: 201 }
        )
      })
    )
    const { onAreaChange } = renderPicker({ initialAreaId: "a1" })
    expect(await screen.findByRole("combobox", { name: /^area/i })).toHaveTextContent("Study")

    let listbox = await openSelect(user, /^location/i)
    await user.click(await within(listbox).findByRole("option", { name: /new location/i }))
    let dialog = await screen.findByTestId("location-form-dialog")
    await user.type(within(dialog).getByTestId("location-name-input"), "Cottage")
    await user.click(within(dialog).getByTestId("location-form-submit"))
    await waitFor(() => expect(within(dialog).getByTestId("location-form-submit")).toBeDisabled())
    // Escape closes the dialog while the create runs.
    await user.keyboard("{Escape}")
    await waitFor(() => expect(screen.queryByTestId("location-form-dialog")).toBeNull())

    // The dialog opens again before the create ends.
    listbox = await openSelect(user, /^location/i)
    await user.click(await within(listbox).findByRole("option", { name: /new location/i }))
    dialog = await screen.findByTestId("location-form-dialog")
    respond()
    await waitFor(() => expect(within(dialog).getByTestId("location-form-submit")).toBeEnabled())
    // The create that ended belongs to the closed opening: it neither
    // selects nor closes the dialog open now.
    expect(screen.getByTestId("location-form-dialog")).toBeInTheDocument()
    await user.click(within(dialog).getByTestId("location-form-cancel"))
    await waitFor(() => expect(screen.queryByTestId("location-form-dialog")).toBeNull())

    expect(screen.getByRole("combobox", { name: /^location/i })).toHaveTextContent("Home")
    expect(screen.getByRole("combobox", { name: /^area/i })).toHaveTextContent("Study")
    expect(onAreaChange).not.toHaveBeenCalled()
    // The location was created, so it can be picked.
    listbox = await openSelect(user, /^location/i)
    expect(within(listbox).getByRole("option", { name: "Cottage" })).toBeInTheDocument()
  })

  it("keeps a choice made while an area's create ran after its dialog closed", async () => {
    const user = userEvent.setup()
    let respond!: () => void
    const responded = new Promise<void>((resolve) => {
      respond = resolve
    })
    server.use(
      http.post(apiUrl(`/g/${SLUG}/areas`), async () => {
        await responded
        return HttpResponse.json(
          { data: { id: "a2", type: "areas", attributes: { name: "Attic", location_id: "l1" } } },
          { status: 201 }
        )
      })
    )
    const { onAreaChange } = renderPicker({ initialAreaId: "a1" })
    expect(await screen.findByRole("combobox", { name: /^area/i })).toHaveTextContent("Study")

    let listbox = await openSelect(user, /^area/i)
    await user.click(await within(listbox).findByRole("option", { name: /new area/i }))
    const dialog = await screen.findByTestId("area-form-dialog")
    await user.type(within(dialog).getByTestId("area-name-input"), "Attic")
    await user.click(within(dialog).getByTestId("area-form-submit"))
    await waitFor(() => expect(within(dialog).getByTestId("area-form-submit")).toBeDisabled())
    await user.keyboard("{Escape}")
    await waitFor(() => expect(screen.queryByTestId("area-form-dialog")).toBeNull())

    // A choice made while the create runs.
    listbox = await openSelect(user, /^area/i)
    expect(within(listbox).queryByRole("option", { name: "Attic" })).toBeNull()
    await user.keyboard("{Escape}")
    await user.click(screen.getByTestId("test-location-clear"))
    expect(onAreaChange).toHaveBeenLastCalledWith("")

    respond()
    // The created area joins the list of its location without being picked.
    await user.click(screen.getByRole("combobox", { name: /^location/i }))
    await user.click(await screen.findByRole("option", { name: "Home" }))
    listbox = await openSelect(user, /^area/i)
    expect(await within(listbox).findByRole("option", { name: "Attic" })).toBeInTheDocument()
    expect(onAreaChange).not.toHaveBeenCalledWith("a2")
  })

  it("opens the dialog when typing picks the entry on a closed select", async () => {
    const user = userEvent.setup()
    renderPicker({ initialAreaId: "a1" })
    // The entry shows once the role is in.
    const listbox = await openSelect(user, /^location/i)
    await within(listbox).findByRole("option", { name: /new location/i })
    await user.keyboard("{Escape}")
    await waitFor(() => expect(screen.queryByRole("listbox")).toBeNull())

    // Radix picks the entry starting with the typed letter without opening
    // the list.
    screen.getByRole("combobox", { name: /^location/i }).focus()
    await user.keyboard("n")
    const dialog = await screen.findByTestId("location-form-dialog")
    await user.click(within(dialog).getByTestId("location-form-cancel"))
    await waitFor(() => expect(screen.queryByTestId("location-form-dialog")).toBeNull())

    // Nothing is left waiting to open with the next list that closes.
    const areaList = await openSelect(user, /^area/i)
    await user.click(within(areaList).getByRole("option", { name: "Study" }))
    await waitFor(() => expect(screen.queryByRole("listbox")).toBeNull())
    await expect(
      screen.findByTestId("location-form-dialog", {}, { timeout: 300 })
    ).rejects.toThrow()
  })

  it("keeps a file dropped on the dialog from the page around it", async () => {
    const user = userEvent.setup()
    const { onOuterDrop } = renderPicker({ initialAreaId: "a1" })
    const listbox = await openSelect(user, /^location/i)
    await user.click(await within(listbox).findByRole("option", { name: /new location/i }))
    const dialog = await screen.findByTestId("location-form-dialog")

    const dataTransfer = {
      types: ["Files"],
      files: [new File(["x"], "receipt.pdf", { type: "application/pdf" })],
      dropEffect: "copy",
    }
    const over = fireEvent.dragOver(dialog, { dataTransfer })
    const dropped = fireEvent.drop(dialog, { dataTransfer })

    expect(onOuterDrop).not.toHaveBeenCalled()
    // Prevented, so the browser does not open the file in place of the page.
    expect(over).toBe(false)
    expect(dropped).toBe(false)
    expect(dataTransfer.dropEffect).toBe("none")
    expect(screen.getByTestId("location-form-dialog")).toBeInTheDocument()
  })
})
