import { beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route, useNavigate } from "react-router-dom"
import { fireEvent, screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { toast } from "sonner"

import { GroupSettingsPage } from "@/pages/groups/GroupSettingsPage"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

type State = "missing" | "added" | "own" | "taken"

interface Row {
  key: string
  name: string
  state: State
  item_count: number
  // The open drafts that have the type or a value of the field.
  draft_count?: number
  icon?: string
  field_type?: string
  taken_by?: string
  // For a field: how many of the set's types the group has do not offer it.
  missing_templates?: number
}

interface RemoveBody {
  data: { type: string; attributes: { item_type_ids: string[]; custom_field_ids: string[] } }
}

const TYPES: Array<[string, string, string]> = [
  ["art", "Art", "palette"],
  ["books", "Books", "book-open"],
  ["coins", "Coins", "coins"],
  ["stamps", "Stamps", "stamp"],
  ["records", "Records", "disc-3"],
  ["watches", "Watches", "watch"],
  ["cards", "Cards", "layers"],
]
const FIELDS: Array<[string, string, string]> = [
  ["condition", "Condition", "select"],
  ["catalog_reference", "Catalog reference", "text"],
  ["edition", "Edition or variant", "text"],
  ["signed", "Signed", "boolean"],
  ["provenance", "Provenance", "long_text"],
  ["certificate", "Certificate of authenticity", "boolean"],
  ["appraised_value", "Appraised value", "money"],
  ["appraisal_date", "Appraisal date", "date"],
]

// SwitchGroup moves the settings to group g1 or g2, as the group selector
// or the browser's Back button would.
function SwitchGroup() {
  const navigate = useNavigate()
  return (
    <>
      {["g1", "g2"].map((id) => (
        <button
          key={id}
          type="button"
          data-testid={`switch-to-${id}`}
          onClick={() => navigate(`/groups/${id}/settings?section=sections`)}
        >
          {id}
        </button>
      ))}
    </>
  )
}

function renderSettings(groupId = "g1") {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: `/groups/${groupId}/settings?section=sections`,
    routes: (
      <Route
        path="/groups/:groupId/settings"
        element={
          <AuthProvider>
            <GroupProvider>
              <ConfirmProvider>
                <SwitchGroup />
                <GroupSettingsPage />
              </ConfirmProvider>
            </GroupProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

interface GroupRef {
  id: string
  slug: string
  name: string
}

const HOUSEHOLD: GroupRef = { id: "g1", slug: "household", name: "Household" }

// A server holding one group and how it holds the Collection set. POST adds
// what is missing; DELETE deletes the listed rows that are the set's and
// unused and reports the other listed ones as kept, as the backend does. A
// row the group has is identified as type-<key> or field-<key>.
function setBackend(
  role: "admin" | "user",
  override: Partial<Record<string, Partial<Row>>> = {},
  opts: {
    addError?: { status: number; code: string }
    // The group this backend holds, g1 unless given, and the other groups
    // GET /groups lists with it.
    group?: GroupRef
    alsoListed?: GroupRef[]
  } = {}
) {
  const g = opts.group ?? HOUSEHOLD
  const types: Row[] = TYPES.map(([key, name, icon]) => ({
    key,
    name,
    icon,
    state: "missing",
    item_count: 0,
    ...override[key],
  }))
  const fields: Row[] = FIELDS.map(([key, name, fieldType]) => ({
    key,
    name,
    field_type: fieldType,
    state: "missing",
    item_count: 0,
    ...override[key],
  }))
  const calls = { posts: 0, deletes: 0, gets: 0, removeBodies: [] as RemoveBody[] }
  // While held, GET /collection-set waits until release() is called.
  let held: Promise<void> | null = null
  const hold = () => {
    let release = () => {}
    held = new Promise<void>((resolve) => {
      release = () => {
        held = null
        resolve()
      }
    })
    return release
  }
  let failGets = false
  const has = (r: Row) => r.state === "added" || r.state === "own"
  const keep = (r: Row) =>
    r.state === "own"
      ? "own"
      : r.state === "added" && r.item_count + (r.draft_count ?? 0) > 0
        ? "in_use"
        : undefined
  const removable = (r: Row) => r.state === "added" && !keep(r)
  // change changes a row as another member would, between two reads.
  const change = (key: string, patch: Partial<Row>) => {
    Object.assign(
      [...types, ...fields].find((r) => r.key === key)!,
      patch
    )
  }
  const view = () => ({
    complete:
      [...types, ...fields].every((r) => r.state !== "missing") &&
      fields.every((r) => !r.missing_templates),
    item_types: types.map((r) => ({
      key: r.key,
      name: r.name,
      icon: r.icon,
      state: r.state,
      taken_by: r.taken_by,
      item_type_id: has(r) ? `type-${r.key}` : undefined,
      item_count: r.item_count,
      trashed_item_count: 0,
      draft_count: r.draft_count ?? 0,
      removable: removable(r),
      keep_reason: has(r) ? keep(r) : undefined,
    })),
    custom_fields: fields.map((r) => ({
      key: r.key,
      name: r.name,
      field_type: r.field_type,
      state: r.state,
      taken_by: r.taken_by,
      custom_field_id: has(r) ? `field-${r.key}` : undefined,
      missing_templates: r.missing_templates ?? 0,
      item_count: r.item_count,
      trashed_item_count: 0,
      draft_count: r.draft_count ?? 0,
      removable: removable(r),
      keep_reason: has(r) ? keep(r) : undefined,
    })),
  })
  const envelope = (meta?: Record<string, unknown>) => ({
    data: { id: "collection", type: "item_sets", attributes: view() },
    ...(meta ? { meta } : {}),
  })
  const groupAttrs = (ref: GroupRef) => ({ ...ref, group_currency: "USD" })
  const attrs = groupAttrs(g)
  const handlers = [
    msw.get(api("/auth/me"), () =>
      HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
    ),
    msw.get(api("/groups"), () =>
      HttpResponse.json({
        data: [g, ...(opts.alsoListed ?? [])].map((ref) => ({
          id: ref.id,
          type: "groups",
          attributes: groupAttrs(ref),
        })),
      })
    ),
    msw.get(api(`/groups/${g.id}`), () =>
      HttpResponse.json({ data: { id: g.id, type: "groups", attributes: attrs } })
    ),
    msw.get(api(`/groups/${g.id}/members`), () =>
      HttpResponse.json({
        data: [
          {
            id: "m1",
            type: "memberships",
            attributes: { id: "m1", group_id: g.id, member_user_id: "u1", role },
          },
        ],
      })
    ),
    msw.get(api(`/groups/${g.id}/collection-set`), async () => {
      calls.gets++
      if (held) await held
      if (failGets) {
        return HttpResponse.json({ errors: [{ status: "500" }] }, { status: 500 })
      }
      return HttpResponse.json(envelope())
    }),
    msw.post(api(`/groups/${g.id}/collection-set`), () => {
      calls.posts++
      if (opts.addError) {
        return HttpResponse.json(
          { errors: [{ status: String(opts.addError.status), code: opts.addError.code }] },
          { status: opts.addError.status }
        )
      }
      let createdTypes = 0
      let createdFields = 0
      for (const r of types) {
        if (r.state === "missing") {
          r.state = "added"
          createdTypes++
        }
      }
      for (const r of fields) {
        if (r.state === "missing") {
          r.state = "added"
          createdFields++
        }
        r.missing_templates = 0
      }
      return HttpResponse.json(
        envelope({
          created_item_types: createdTypes,
          created_custom_fields: createdFields,
          added_templates: 0,
          deleted_item_types: [],
          deleted_custom_fields: [],
        })
      )
    }),
    msw.delete(api(`/groups/${g.id}/collection-set`), async ({ request }) => {
      calls.deletes++
      const body = (await request.json()) as RemoveBody
      calls.removeBodies.push(body)
      const removeListed = (rows: Row[], prefix: string, ids: string[]) => {
        const deleted: string[] = []
        const kept: string[] = []
        for (const r of rows) {
          if (!has(r) || !ids.includes(`${prefix}-${r.key}`)) continue
          if (removable(r)) {
            r.state = "missing"
            deleted.push(r.name)
          } else {
            kept.push(r.name)
          }
        }
        return [deleted, kept]
      }
      const [deletedTypes, keptTypes] = removeListed(
        types,
        "type",
        body.data.attributes.item_type_ids
      )
      const [deletedFields, keptFields] = removeListed(
        fields,
        "field",
        body.data.attributes.custom_field_ids
      )
      return HttpResponse.json(
        envelope({
          created_item_types: 0,
          created_custom_fields: 0,
          added_templates: 0,
          deleted_item_types: deletedTypes,
          deleted_custom_fields: deletedFields,
          kept_item_types: keptTypes,
          kept_custom_fields: keptFields,
        })
      )
    }),
    msw.get(api(`/g/${g.slug}/notifications`), () =>
      HttpResponse.json({ warranty_expiring_alerts: true, weekly_digest: false })
    ),
  ]
  return {
    calls,
    handlers,
    hold,
    change,
    failGets: () => {
      failGets = true
    },
  }
}

const everythingAdded = Object.fromEntries(
  [...TYPES, ...FIELDS].map(([key]) => [key, { state: "added" as State }])
)

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

describe("<GroupSettingsPage /> — Collection set (#1607)", () => {
  it("lists the set's types and fields and adds the set", async () => {
    const backend = setBackend("admin")
    server.use(...backend.handlers)
    const user = userEvent.setup()
    renderSettings()

    const card = await screen.findByTestId("collection-set-card")
    expect(await within(card).findByTestId("collection-set-state")).toHaveTextContent("Not added")
    const types = within(card).getByTestId("collection-set-types")
    for (const [key, name] of TYPES) {
      const badge = within(types).getByTestId(`collection-set-type-${key}`)
      expect(badge).toHaveTextContent(name)
      expect(badge).toHaveTextContent("Not in the group")
    }
    expect(within(card).getAllByRole("listitem")).toHaveLength(TYPES.length + FIELDS.length)
    expect(within(card).queryByTestId("collection-set-remove")).not.toBeInTheDocument()

    await user.click(await within(card).findByTestId("collection-set-add"))
    await waitFor(() =>
      expect(within(card).getByTestId("collection-set-state")).toHaveTextContent("Added")
    )
    expect(backend.calls.posts).toBe(1)
    expect(vi.mocked(toast.success).mock.calls[0]?.[0]).toBe(
      "Collection set added. New item types: 7, new custom fields: 8."
    )
    expect(within(card).queryByTestId("collection-set-add")).not.toBeInTheDocument()
    expect(within(card).getByTestId("collection-set-remove")).toBeEnabled()
    expect(within(card).getByTestId("collection-set-field-signed")).toHaveTextContent(
      "Added by the set"
    )
  })

  it("offers to add what is missing when the set is partly there", async () => {
    const backend = setBackend("admin", {
      coins: { state: "added", name: "Stamps" },
      stamps: { state: "taken", taken_by: "coins" },
      condition: { state: "own" },
    })
    server.use(...backend.handlers)
    renderSettings()

    const card = await screen.findByTestId("collection-set-card")
    expect(await within(card).findByTestId("collection-set-state")).toHaveTextContent(
      "Partly added"
    )
    expect(await within(card).findByTestId("collection-set-add")).toHaveTextContent(
      "Add what is missing"
    )
    expect(within(card).getByTestId("collection-set-type-coins")).toHaveAttribute(
      "data-state",
      "added"
    )
    expect(within(card).getByTestId("collection-set-type-art")).toHaveAttribute(
      "data-state",
      "missing"
    )
    // Each state is spelled out, not only shown by color.
    expect(within(card).getByTestId("collection-set-type-coins")).toHaveTextContent(
      "Added by the set"
    )
    expect(within(card).getByTestId("collection-set-type-art")).toHaveTextContent(
      "Not in the group"
    )
    expect(within(card).getByTestId("collection-set-field-condition")).toHaveTextContent(
      "The group's own"
    )
    expect(within(card).getByTestId("collection-set-type-stamps")).toHaveTextContent(
      "Not added: another type of the set has this name"
    )
  })

  it("is partly added while a field is not offered on every type of the set", async () => {
    const backend = setBackend(
      "admin",
      Object.fromEntries([
        ...TYPES.map(([key]) => [key, { state: "own" as State }]),
        ...FIELDS.map(([key]) => [key, { state: "own" as State }]),
        ["signed", { state: "own" as State, missing_templates: 7 }],
      ])
    )
    server.use(...backend.handlers)
    const user = userEvent.setup()
    renderSettings()

    const card = await screen.findByTestId("collection-set-card")
    expect(await within(card).findByTestId("collection-set-state")).toHaveTextContent(
      "Partly added"
    )
    expect(within(card).getByTestId("collection-set-links-missing")).toHaveTextContent(
      "Some of the set's fields are not offered on all of its types yet."
    )
    expect(within(card).getByTestId("collection-set-field-signed")).toHaveTextContent(
      "The group's own; not offered on 7 of the set's types"
    )
    expect(within(card).getByTestId("collection-set-field-condition")).not.toHaveTextContent(
      "not offered"
    )

    await user.click(await within(card).findByTestId("collection-set-add"))
    await waitFor(() =>
      expect(within(card).getByTestId("collection-set-state")).toHaveTextContent("Added")
    )
    expect(backend.calls.posts).toBe(1)
    expect(within(card).queryByTestId("collection-set-links-missing")).not.toBeInTheDocument()
  })

  it("lists what removing deletes and what stays, with the reason, and removes on confirm", async () => {
    const backend = setBackend("admin", {
      ...everythingAdded,
      coins: { state: "added", item_count: 2 },
      books: { state: "own" },
      condition: { state: "added", item_count: 1 },
    })
    server.use(...backend.handlers)
    const user = userEvent.setup()
    renderSettings()

    const card = await screen.findByTestId("collection-set-card")
    await user.click(await within(card).findByTestId("collection-set-remove"))
    const dialog = await screen.findByTestId("collection-set-remove-dialog")
    await waitFor(() =>
      expect(within(dialog).getByTestId("collection-set-remove-confirm")).toBeEnabled()
    )

    const goes = within(dialog).getByTestId("collection-set-remove-goes")
    expect(within(goes).getAllByRole("listitem")).toHaveLength(5 + 7)
    expect(goes).toHaveTextContent("Art")
    expect(goes).toHaveTextContent("Catalog reference")
    expect(goes).not.toHaveTextContent("Coins")

    const stays = within(dialog).getByTestId("collection-set-remove-stays")
    expect(within(stays).getAllByRole("listitem")).toHaveLength(3)
    expect(within(stays).getByTestId("collection-set-stays-type-coins")).toHaveTextContent(
      "2 items have this type, Trash included."
    )
    expect(within(stays).getByTestId("collection-set-stays-type-books")).toHaveTextContent(
      "The group's own type. The set uses it but did not create it."
    )
    expect(within(stays).getByTestId("collection-set-stays-field-condition")).toHaveTextContent(
      "1 item has a value of this field, Trash included."
    )
    expect(backend.calls.deletes).toBe(0)

    await user.click(within(dialog).getByTestId("collection-set-remove-confirm"))
    await waitFor(() =>
      expect(screen.queryByTestId("collection-set-remove-dialog")).not.toBeInTheDocument()
    )
    expect(backend.calls.deletes).toBe(1)
    expect(backend.calls.removeBodies[0]).toEqual({
      data: {
        type: "item_sets",
        attributes: {
          item_type_ids: ["type-art", "type-stamps", "type-records", "type-watches", "type-cards"],
          custom_field_ids: [
            "field-catalog_reference",
            "field-edition",
            "field-signed",
            "field-provenance",
            "field-certificate",
            "field-appraised_value",
            "field-appraisal_date",
          ],
        },
      },
    })
    expect(vi.mocked(toast.success).mock.calls[0]?.[0]).toBe(
      "Deleted Art, Stamps, Records, Watches, Cards, Catalog reference, Edition or variant, " +
        "Signed, Provenance, Certificate of authenticity, Appraised value, and Appraisal date."
    )
    expect(toast.warning).not.toHaveBeenCalled()
    await waitFor(() =>
      expect(within(card).getByTestId("collection-set-state")).toHaveTextContent("Partly added")
    )
    expect(within(card).getByTestId("collection-set-type-coins")).toHaveAttribute(
      "data-state",
      "added"
    )
  })

  it("keeps the rows that drafts use and says how many drafts use them", async () => {
    const backend = setBackend("admin", {
      ...everythingAdded,
      coins: { state: "added", item_count: 2, draft_count: 3 },
      stamps: { state: "added", item_count: 0, draft_count: 1 },
      condition: { state: "added", item_count: 0, draft_count: 2 },
    })
    server.use(...backend.handlers)
    const user = userEvent.setup()
    renderSettings()

    const card = await screen.findByTestId("collection-set-card")
    await user.click(await within(card).findByTestId("collection-set-remove"))
    const dialog = await screen.findByTestId("collection-set-remove-dialog")
    await waitFor(() =>
      expect(within(dialog).getByTestId("collection-set-remove-confirm")).toBeEnabled()
    )
    const stays = within(dialog).getByTestId("collection-set-remove-stays")
    expect(within(stays).getByTestId("collection-set-stays-type-coins")).toHaveTextContent(
      "2 items have this type, Trash included. 3 drafts have this type."
    )
    expect(within(stays).getByTestId("collection-set-stays-type-stamps")).toHaveTextContent(
      "1 draft has this type."
    )
    expect(within(stays).getByTestId("collection-set-stays-type-stamps")).not.toHaveTextContent(
      "items"
    )
    expect(within(stays).getByTestId("collection-set-stays-field-condition")).toHaveTextContent(
      "2 drafts have a value of this field."
    )
    const goes = within(dialog).getByTestId("collection-set-remove-goes")
    expect(goes).not.toHaveTextContent("Stamps")
  })

  it("says nothing can be deleted when every row of the set stays", async () => {
    const backend = setBackend("admin", {
      ...everythingAdded,
      ...Object.fromEntries(
        [...TYPES, ...FIELDS].map(([key]) => [key, { state: "added" as State, item_count: 1 }])
      ),
    })
    server.use(...backend.handlers)
    const user = userEvent.setup()
    renderSettings()

    await user.click(await screen.findByTestId("collection-set-remove"))
    const dialog = await screen.findByTestId("collection-set-remove-dialog")
    expect(dialog).toHaveTextContent("Nothing can be deleted")
    expect(within(dialog).queryByTestId("collection-set-remove-goes")).not.toBeInTheDocument()
    expect(within(dialog).queryByTestId("collection-set-remove-confirm")).not.toBeInTheDocument()
    await user.click(within(dialog).getByTestId("collection-set-remove-cancel"))
    expect(backend.calls.deletes).toBe(0)
  })

  it("checks the status again when the confirmation opens and waits for it", async () => {
    const backend = setBackend("admin", everythingAdded)
    server.use(...backend.handlers)
    const user = userEvent.setup()
    renderSettings()

    const card = await screen.findByTestId("collection-set-card")
    const remove = await within(card).findByTestId("collection-set-remove")
    // An item gets the Coins type after the card loaded.
    backend.change("coins", { item_count: 1 })
    const release = backend.hold()
    await user.click(remove)
    const dialog = await screen.findByTestId("collection-set-remove-dialog")
    expect(within(dialog).getByTestId("collection-set-remove-checking")).toBeInTheDocument()
    expect(within(dialog).getByTestId("collection-set-remove-confirm")).toBeDisabled()

    release()
    await waitFor(() =>
      expect(within(dialog).getByTestId("collection-set-remove-confirm")).toBeEnabled()
    )
    expect(within(dialog).queryByTestId("collection-set-remove-checking")).not.toBeInTheDocument()
    expect(within(dialog).getByTestId("collection-set-stays-type-coins")).toHaveTextContent(
      "1 item has this type, Trash included."
    )

    await user.click(within(dialog).getByTestId("collection-set-remove-confirm"))
    await waitFor(() => expect(backend.calls.removeBodies).toHaveLength(1))
    expect(backend.calls.removeBodies[0]?.data.attributes.item_type_ids).not.toContain("type-coins")
  })

  it("names what the server kept because it changed after the check", async () => {
    const backend = setBackend("admin", everythingAdded)
    server.use(...backend.handlers)
    const user = userEvent.setup()
    renderSettings()

    await user.click(await screen.findByTestId("collection-set-remove"))
    const dialog = await screen.findByTestId("collection-set-remove-dialog")
    await waitFor(() =>
      expect(within(dialog).getByTestId("collection-set-remove-confirm")).toBeEnabled()
    )
    // Listed as deleted, then an item gets the type before the confirm.
    backend.change("art", { item_count: 1 })
    await user.click(within(dialog).getByTestId("collection-set-remove-confirm"))

    await waitFor(() => expect(toast.warning).toHaveBeenCalledTimes(1))
    expect(backend.calls.removeBodies).toHaveLength(1)
    expect(backend.calls.removeBodies[0]?.data.attributes.item_type_ids).toContain("type-art")
    expect(vi.mocked(toast.warning).mock.calls[0]?.[0]).toBe(
      "Kept Art: it changed after you opened the confirmation."
    )
    expect(vi.mocked(toast.success).mock.calls[0]?.[0]).toMatch(/^Deleted Books, Coins, /)
  })

  it("drops an open confirmation when the settings move to another group", async () => {
    const collection: GroupRef = { id: "g2", slug: "collection", name: "Collection" }
    const other = setBackend("admin", everythingAdded, { group: collection })
    const backend = setBackend("admin", everythingAdded, { alsoListed: [collection] })
    server.use(...other.handlers, ...backend.handlers)
    const user = userEvent.setup()
    // Start on g2 so its group, members and set are cached, as after a visit.
    renderSettings("g2")
    await screen.findByTestId("collection-set-remove")
    await user.click(screen.getByTestId("switch-to-g1"))
    await waitFor(() => expect(backend.calls.gets).toBeGreaterThan(0))

    await user.click(await screen.findByTestId("collection-set-remove"))
    const dialog = await screen.findByTestId("collection-set-remove-dialog")
    await waitFor(() =>
      expect(within(dialog).getByTestId("collection-set-remove-confirm")).toBeEnabled()
    )

    // Back to the cached g2. The open modal blocks pointer events on the
    // page, as the browser's Back button is not blocked, so the move is
    // fired directly.
    fireEvent.click(screen.getByTestId("switch-to-g2"))
    await waitFor(() =>
      expect(screen.queryByTestId("collection-set-remove-dialog")).not.toBeInTheDocument()
    )
    expect(await screen.findByTestId("collection-set-remove")).toBeEnabled()
    expect(backend.calls.deletes + other.calls.deletes).toBe(0)
  })

  it("keeps Confirm disabled when the check fails", async () => {
    const backend = setBackend("admin", everythingAdded)
    server.use(...backend.handlers)
    const user = userEvent.setup()
    renderSettings()

    const remove = await screen.findByTestId("collection-set-remove")
    backend.failGets()
    await user.click(remove)
    const dialog = await screen.findByTestId("collection-set-remove-dialog")
    expect(
      await within(dialog).findByTestId("collection-set-remove-check-failed")
    ).toBeInTheDocument()
    expect(within(dialog).getByTestId("collection-set-remove-confirm")).toBeDisabled()
    expect(backend.calls.deletes).toBe(0)
  })

  it("names the limit when the group has too many item types", async () => {
    const backend = setBackend("admin", {}, { addError: { status: 409, code: "item_type.limit" } })
    server.use(...backend.handlers)
    const user = userEvent.setup()
    renderSettings()

    await user.click(await screen.findByTestId("collection-set-add"))
    await waitFor(() => expect(toast.error).toHaveBeenCalledTimes(1))
    expect(vi.mocked(toast.error).mock.calls[0]?.[0]).toBe(
      "The group has the most item types it can have. Delete or merge some, then add the set again."
    )
  })

  it("shows a member without the admin role the set, without the buttons", async () => {
    const backend = setBackend("user", { coins: { state: "added" } })
    server.use(...backend.handlers)
    renderSettings()

    const card = await screen.findByTestId("collection-set-card")
    expect(await within(card).findByTestId("collection-set-admin-only")).toBeInTheDocument()
    expect(within(card).queryByTestId("collection-set-add")).not.toBeInTheDocument()
    expect(within(card).queryByTestId("collection-set-remove")).not.toBeInTheDocument()
  })
})
