import { beforeEach, describe, expect, it, vi } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
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

type State = "missing" | "added" | "own"

interface Row {
  key: string
  name: string
  state: State
  item_count: number
  icon?: string
  field_type?: string
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

function renderSettings() {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath: "/groups/g1/settings?section=sections",
    routes: (
      <Route
        path="/groups/:groupId/settings"
        element={
          <AuthProvider>
            <GroupProvider>
              <ConfirmProvider>
                <GroupSettingsPage />
              </ConfirmProvider>
            </GroupProvider>
          </AuthProvider>
        }
      />
    ),
  })
}

// A server holding one group and how it holds the Collection set. POST adds
// what is missing; DELETE deletes the set's rows no item uses, as the
// backend does.
function setBackend(
  role: "admin" | "user",
  override: Partial<Record<string, Partial<Row>>> = {},
  opts: { addError?: { status: number; code: string } } = {}
) {
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
  const calls = { posts: 0, deletes: 0 }
  const keep = (r: Row) =>
    r.state === "own" ? "own" : r.state === "added" && r.item_count > 0 ? "in_use" : undefined
  const view = () => ({
    complete: [...types, ...fields].every((r) => r.state !== "missing"),
    item_types: types.map((r) => ({
      key: r.key,
      name: r.name,
      icon: r.icon,
      state: r.state,
      item_count: r.item_count,
      trashed_item_count: 0,
      removable: r.state === "added" && !keep(r),
      keep_reason: r.state === "missing" ? undefined : keep(r),
    })),
    custom_fields: fields.map((r) => ({
      key: r.key,
      name: r.name,
      field_type: r.field_type,
      state: r.state,
      item_count: r.item_count,
      trashed_item_count: 0,
      removable: r.state === "added" && !keep(r),
      keep_reason: r.state === "missing" ? undefined : keep(r),
    })),
  })
  const envelope = (meta?: Record<string, unknown>) => ({
    data: { id: "collection", type: "item_sets", attributes: view() },
    ...(meta ? { meta } : {}),
  })
  const attrs = { id: "g1", slug: "household", name: "Household", group_currency: "USD" }
  const handlers = [
    msw.get(api("/auth/me"), () =>
      HttpResponse.json({ id: "u1", email: "alex@example.com", name: "Alex" })
    ),
    msw.get(api("/groups"), () =>
      HttpResponse.json({ data: [{ id: "g1", type: "groups", attributes: attrs }] })
    ),
    msw.get(api("/groups/g1"), () =>
      HttpResponse.json({ data: { id: "g1", type: "groups", attributes: attrs } })
    ),
    msw.get(api("/groups/g1/members"), () =>
      HttpResponse.json({
        data: [
          {
            id: "m1",
            type: "memberships",
            attributes: { id: "m1", group_id: "g1", member_user_id: "u1", role },
          },
        ],
      })
    ),
    msw.get(api("/groups/g1/collection-set"), () => HttpResponse.json(envelope())),
    msw.post(api("/groups/g1/collection-set"), () => {
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
    msw.delete(api("/groups/g1/collection-set"), () => {
      calls.deletes++
      const deletedTypes: string[] = []
      const deletedFields: string[] = []
      for (const r of types) {
        if (r.state === "added" && r.item_count === 0) {
          r.state = "missing"
          deletedTypes.push(r.name)
        }
      }
      for (const r of fields) {
        if (r.state === "added" && r.item_count === 0) {
          r.state = "missing"
          deletedFields.push(r.name)
        }
      }
      return HttpResponse.json(
        envelope({
          created_item_types: 0,
          created_custom_fields: 0,
          added_templates: 0,
          deleted_item_types: deletedTypes,
          deleted_custom_fields: deletedFields,
        })
      )
    }),
    msw.get(api("/g/household/notifications"), () =>
      HttpResponse.json({ warranty_expiring_alerts: true, weekly_digest: false })
    ),
  ]
  return { calls, handlers }
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
      expect(within(types).getByTestId(`collection-set-type-${key}`)).toHaveTextContent(name)
    }
    expect(within(card).getAllByRole("listitem")).toHaveLength(TYPES.length + FIELDS.length)
    expect(within(card).queryByTestId("collection-set-remove")).not.toBeInTheDocument()

    await user.click(within(card).getByTestId("collection-set-add"))
    await waitFor(() =>
      expect(within(card).getByTestId("collection-set-state")).toHaveTextContent("Added")
    )
    expect(backend.calls.posts).toBe(1)
    expect(vi.mocked(toast.success).mock.calls[0]?.[0]).toBe(
      "Collection set added. New item types: 7, new custom fields: 8."
    )
    expect(within(card).queryByTestId("collection-set-add")).not.toBeInTheDocument()
    expect(within(card).getByTestId("collection-set-remove")).toBeEnabled()
  })

  it("offers to add what is missing when the set is partly there", async () => {
    const backend = setBackend("admin", { coins: { state: "added" }, condition: { state: "own" } })
    server.use(...backend.handlers)
    renderSettings()

    const card = await screen.findByTestId("collection-set-card")
    expect(await within(card).findByTestId("collection-set-state")).toHaveTextContent(
      "Partly added"
    )
    expect(within(card).getByTestId("collection-set-add")).toHaveTextContent("Add what is missing")
    expect(within(card).getByTestId("collection-set-type-coins")).toHaveAttribute(
      "data-state",
      "added"
    )
    expect(within(card).getByTestId("collection-set-type-art")).toHaveAttribute(
      "data-state",
      "missing"
    )
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
    expect(vi.mocked(toast.success).mock.calls[0]?.[0]).toBe(
      "Collection set removed. Deleted item types: 5, deleted custom fields: 7."
    )
    await waitFor(() =>
      expect(within(card).getByTestId("collection-set-state")).toHaveTextContent("Partly added")
    )
    expect(within(card).getByTestId("collection-set-type-coins")).toHaveAttribute(
      "data-state",
      "added"
    )
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
