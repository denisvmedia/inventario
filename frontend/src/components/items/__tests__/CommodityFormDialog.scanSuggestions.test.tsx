import { afterEach, beforeEach, describe, expect, it, onTestFinished, vi } from "vitest"
import { Route } from "react-router-dom"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import type { Commodity } from "@/features/commodities/api"
import { GroupProvider } from "@/features/group/GroupContext"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import {
  apiUrl,
  commodityScanHandlers,
  customFieldHandlers,
  draftHandlers,
  groupHandlers,
} from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"

// What an AI scan suggests for each file, and how a scan treats what the
// form already holds (#1626).

const SLUG = "g"
const areas = [{ id: "a1", name: "Garage", location_id: "l1" }]
const locations = [{ id: "l1", name: "Home" }]

type User = ReturnType<typeof userEvent.setup>
type ScanAttrs = Parameters<typeof commodityScanHandlers.ok>[1]

// Every field the form requires, so Next walks to the Files step.
const COMPLETE = {
  name: { value: "Bosch drill", confidence: 0.95 },
  short_name: { value: "Drill", confidence: 0.95 },
  type: { value: "electronics", confidence: 0.95 },
  original_price: { value: 99, confidence: 0.95 },
  original_price_currency: { value: "USD", confidence: 0.95 },
  purchase_date: { value: "2024-01-15", confidence: 0.95 },
}

// The same File object for every scan: the form knows a file by its name,
// size and time, as it does when the user picks the same file again.
const photo = new File([new Uint8Array([0xff, 0xd8, 0xff])], "IMG_0001.jpg", {
  type: "image/jpeg",
  lastModified: 1,
})
const receipt = new File([new Uint8Array([0x25, 0x50, 0x44, 0x46])], "scan-17.pdf", {
  type: "application/pdf",
  lastModified: 1,
})

function renderDialog() {
  setCurrentGroupSlug(SLUG)
  renderWithProviders({
    children: (
      <CommodityFormDialog
        open
        onOpenChange={() => {}}
        mode="create"
        areas={areas}
        locations={locations}
        defaultCurrency="USD"
        onSubmit={async () => {}}
      />
    ),
  })
}

async function scan(user: User, files: File[], attrs: ScanAttrs, tick: string[] = []) {
  server.use(...commodityScanHandlers.ok(SLUG, attrs))
  await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), files)
  await user.click(screen.getByTestId("commodity-form-ai-scan"))
  await screen.findByTestId("commodity-form-ai-review")
  for (const key of tick) await user.click(screen.getByTestId(`commodity-form-ai-row-${key}-check`))
  await user.click(screen.getByTestId("commodity-form-ai-use-values"))
  await screen.findByTestId("commodity-form-basics-step")
}

async function walkToFiles(user: User) {
  for (let i = 0; i < 4; i++) await user.click(screen.getByTestId("commodity-form-next"))
  return screen.findByTestId("commodity-form-files-step")
}

// backToScan goes back to the AI step from a later step.
async function backToScan(user: User) {
  await user.click(screen.getByRole("button", { name: "Basics" }))
  await screen.findByTestId("commodity-form-basics-step")
  await user.click(screen.getByRole("button", { name: /^back$/i }))
  await screen.findByTestId("commodity-form-ai-dropzone")
}

function rowOf(name: string): HTMLElement {
  const row = screen
    .getAllByTestId("commodity-files-pending")
    .find((r) => within(r).queryByText(name))
  if (!row) throw new Error(`no staged row for ${name}`)
  return row
}

function chipsOf(row: HTMLElement) {
  return within(row).queryAllByTestId(/^commodity-files-tags-.*-chip$/)
}

function titleOf(row: HTMLElement) {
  return within(row).getByTestId(/^commodity-files-title-(?!suggested)/) as HTMLInputElement
}

describe("<CommodityFormDialog /> scan suggestions for files (#1626)", () => {
  afterEach(() => {
    __resetGroupContextForTests()
  })

  beforeEach(() => {
    server.use(http.get(apiUrl(`/currencies`), () => HttpResponse.json(["USD", "EUR"])))
  })

  it("puts the suggested tag and title on each file, by its place in the request", async () => {
    const user = userEvent.setup()
    renderDialog()
    await scan(user, [photo, receipt], {
      fields: COMPLETE,
      files: [
        {
          index: 1,
          document_type: { value: "receipt", confidence: 0.9 },
          title: { value: "Hardware store receipt", confidence: 0.8 },
        },
        { index: 0, document_type: { value: "photo", confidence: 0.9 } },
      ],
    })
    await walkToFiles(user)

    const receiptRow = rowOf("scan-17.pdf")
    const [receiptChip] = chipsOf(receiptRow)
    expect(receiptChip).toHaveAttribute("data-suggested", "true")
    expect(receiptChip).toHaveTextContent("Invoice")
    expect(receiptChip).toHaveTextContent(/suggested by the AI scan/i)
    expect(titleOf(receiptRow)).toHaveValue("Hardware store receipt")
    expect(within(receiptRow).getByTestId(/^commodity-files-title-suggested-/)).toBeInTheDocument()

    const photoRow = rowOf("IMG_0001.jpg")
    expect(chipsOf(photoRow).map((c) => c.textContent)).toEqual([expect.stringContaining("Photo")])
    // No title suggested: the file name stays.
    expect(titleOf(photoRow)).toHaveValue("IMG_0001")
    expect(within(photoRow).queryByTestId(/^commodity-files-title-suggested-/)).toBeNull()
  })

  it("does not put back a suggested tag the user removed when the file is scanned again", async () => {
    const files = [{ index: 0, document_type: { value: "invoice", confidence: 0.9 } }]
    const user = userEvent.setup()
    renderDialog()
    await scan(user, [receipt], { fields: COMPLETE, files })
    await walkToFiles(user)
    await user.click(within(rowOf("scan-17.pdf")).getByRole("button", { name: /^remove invoice/i }))
    expect(chipsOf(rowOf("scan-17.pdf"))).toHaveLength(0)

    await backToScan(user)
    await scan(user, [receipt], { fields: {}, files })
    await user.click(screen.getByRole("button", { name: "Files" }))
    expect(chipsOf(rowOf("scan-17.pdf"))).toHaveLength(0)
  })

  it("puts back none of the suggested tags the user removed, over several scans", async () => {
    const user = userEvent.setup()
    renderDialog()
    const suggest = (value: string) => ({
      fields: {},
      files: [{ index: 0, document_type: { value, confidence: 0.9 } }],
    })
    const removeChip = async (label: RegExp) => {
      await user.click(within(rowOf("scan-17.pdf")).getByRole("button", { name: label }))
      expect(chipsOf(rowOf("scan-17.pdf"))).toHaveLength(0)
    }
    await scan(user, [receipt], { ...suggest("photo"), fields: COMPLETE })
    await walkToFiles(user)
    await removeChip(/^remove photo/i)

    await backToScan(user)
    await scan(user, [receipt], suggest("invoice"))
    await user.click(screen.getByRole("button", { name: "Files" }))
    expect(chipsOf(rowOf("scan-17.pdf"))[0]).toHaveTextContent("Invoice")
    await removeChip(/^remove invoice/i)

    await backToScan(user)
    await scan(user, [receipt], suggest("photo"))
    await user.click(screen.getByRole("button", { name: "Files" }))
    expect(chipsOf(rowOf("scan-17.pdf"))).toHaveLength(0)
  })

  it("marks a removed suggested tag the user adds back by hand as the user's", async () => {
    const user = userEvent.setup()
    renderDialog()
    await scan(user, [receipt], {
      fields: COMPLETE,
      files: [{ index: 0, document_type: { value: "invoice", confidence: 0.9 } }],
    })
    await walkToFiles(user)
    const row = rowOf("scan-17.pdf")
    await user.click(within(row).getByRole("button", { name: /^remove invoice/i }))
    await user.type(within(row).getByTestId(/^commodity-files-tags-.*-input$/), "invoice{Enter}")
    const chips = chipsOf(rowOf("scan-17.pdf"))
    expect(chips).toHaveLength(1)
    expect(chips[0]).not.toHaveAttribute("data-suggested")
    expect(chips[0]).not.toHaveTextContent(/suggested by the AI scan/i)
  })

  it("does not set a title again once the user changed the suggested one back to the file name", async () => {
    const files = [{ index: 0, title: { value: "Hardware store receipt", confidence: 0.9 } }]
    const user = userEvent.setup()
    renderDialog()
    await scan(user, [receipt], { fields: COMPLETE, files })
    await walkToFiles(user)
    const row = rowOf("scan-17.pdf")
    expect(titleOf(row)).toHaveValue("Hardware store receipt")
    await user.clear(titleOf(row))
    await user.type(titleOf(row), "scan-17")

    await backToScan(user)
    await scan(user, [receipt], { fields: {}, files })
    await user.click(screen.getByRole("button", { name: "Files" }))
    const again = rowOf("scan-17.pdf")
    expect(titleOf(again)).toHaveValue("scan-17")
    expect(within(again).queryByTestId(/^commodity-files-title-suggested-/)).toBeNull()
  })

  it("keeps a tag and a title the user set when the file is scanned again", async () => {
    const user = userEvent.setup()
    renderDialog()
    await scan(user, [receipt], { fields: COMPLETE })
    await walkToFiles(user)
    const row = rowOf("scan-17.pdf")
    expect(chipsOf(row)).toHaveLength(0)
    await user.clear(titleOf(row))
    await user.type(titleOf(row), "Warranty card")
    await user.type(within(row).getByTestId(/^commodity-files-tags-.*-input$/), "warranty{Enter}")

    await backToScan(user)
    await scan(user, [receipt], {
      fields: {},
      files: [
        {
          index: 0,
          document_type: { value: "invoice", confidence: 0.9 },
          title: { value: "Invoice 17", confidence: 0.9 },
        },
      ],
    })
    await user.click(screen.getByRole("button", { name: "Files" }))
    const again = rowOf("scan-17.pdf")
    expect(titleOf(again)).toHaveValue("Warranty card")
    expect(chipsOf(again).map((c) => c.getAttribute("data-suggested"))).toEqual([null])
    expect(chipsOf(again)[0]).toHaveTextContent(/warranty/i)
  })

  it("matches a suggestion to the file it names when the list holds the files in another order", async () => {
    const user = userEvent.setup()
    renderDialog()
    // The first scan stages the receipt; the second sends a photo first
    // and the receipt second, so the receipt stays first in the list.
    await scan(user, [receipt], { fields: COMPLETE })
    await backToScan(user)
    await scan(user, [photo, receipt], {
      fields: {},
      files: [
        { index: 0, document_type: { value: "photo", confidence: 0.9 } },
        { index: 1, document_type: { value: "receipt", confidence: 0.9 } },
      ],
    })
    await walkToFiles(user)
    const rows = screen.getAllByTestId("commodity-files-pending")
    expect(within(rows[0]).getByText("scan-17.pdf")).toBeInTheDocument()
    expect(chipsOf(rowOf("scan-17.pdf"))[0]).toHaveTextContent("Invoice")
    expect(chipsOf(rowOf("IMG_0001.jpg"))[0]).toHaveTextContent("Photo")
  })
})

describe("<CommodityFormDialog /> scan over values the form holds (#1626)", () => {
  afterEach(() => {
    __resetGroupContextForTests()
  })

  beforeEach(() => {
    server.use(http.get(apiUrl(`/currencies`), () => HttpResponse.json(["USD", "EUR"])))
  })

  it("shows what the user typed beside the scan and keeps it unless ticked", async () => {
    const user = userEvent.setup()
    renderDialog()
    await user.click(await screen.findByTestId("commodity-form-ai-fill-manually"))
    await user.type(await screen.findByLabelText(/^Name$/i), "My drill")
    await user.click(screen.getByRole("button", { name: /^back$/i }))

    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: {
          name: { value: "Bosch GSR 18V", confidence: 0.9 },
          short_name: { value: "GSR 18V", confidence: 0.9 },
          brand: { value: "Bosch", confidence: 0.9 },
          original_price_currency: { value: "EUR", confidence: 0.9 },
        },
      })
    )
    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), receipt)
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")

    expect(screen.getByTestId("commodity-form-ai-row-name-yours")).toHaveTextContent("My drill")
    expect(screen.getByTestId("commodity-form-ai-row-name-value")).toHaveTextContent(
      "Bosch GSR 18V"
    )
    expect(screen.getByTestId("commodity-form-ai-row-name-check")).not.toBeChecked()
    // The short name the form made from the typed name is not the user's
    // value, nor is the group currency a new form starts with.
    expect(screen.queryByTestId("commodity-form-ai-row-short_name-yours")).toBeNull()
    expect(screen.getByTestId("commodity-form-ai-row-short_name-check")).toBeChecked()
    expect(screen.queryByTestId("commodity-form-ai-row-original_price_currency-yours")).toBeNull()
    expect(screen.getByTestId("commodity-form-ai-row-original_price_currency-check")).toBeChecked()

    await user.click(screen.getByTestId("commodity-form-ai-use-values"))
    expect(await screen.findByLabelText(/^Name$/i)).toHaveValue("My drill")
    expect(screen.getByLabelText(/^Brand$/i)).toHaveValue("Bosch")
    expect(screen.getByLabelText(/^Short name$/i)).toHaveValue("GSR 18V")
  })

  it("adds ticked scanned tags to the tags the form holds", async () => {
    const user = userEvent.setup()
    renderDialog()
    await scan(user, [photo], {
      fields: { ...COMPLETE, tags: { value: ["garden-tools"], confidence: 0.9 } },
    })
    await backToScan(user)
    // The form holds the slug; the label the scan reads is the same tag.
    await scan(
      user,
      [photo],
      { fields: { tags: { value: ["Garden Tools", "garage"], confidence: 0.9 } } },
      ["tags"]
    )
    for (let i = 0; i < 3; i++) await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-tags-input")
    const chips = screen.getAllByTestId("commodity-tags-chip")
    expect(chips.map((c) => c.textContent)).toEqual([
      expect.stringContaining("garden-tools"),
      expect.stringContaining("garage"),
    ])
  })
})

describe("<CommodityFormDialog /> scan while editing an item (#1626)", () => {
  afterEach(() => {
    clearAuth()
    __resetGroupContextForTests()
  })

  beforeEach(() => {
    server.use(http.get(apiUrl(`/currencies`), () => HttpResponse.json(["USD", "EUR"])))
  })

  // An item in the group currency, with a custom field at the default its
  // type gives it: in create mode neither would count as entered.
  const existing = {
    id: "c1",
    name: "Bosch drill",
    short_name: "Drill",
    type: "equipment",
    area_id: "a1",
    status: "in_use",
    count: 1,
    draft: false,
    purchase_date: "2024-03-01",
    original_price: 99,
    original_price_currency: "USD",
    custom_fields: { "cf-plate": "ZZ" },
  } as unknown as Commodity

  it("counts the item's saved currency and a custom field at its type default as the user's", async () => {
    // The item form reads the group's fields from the group in context.
    setAccessToken("good-token")
    // The scanned receipt joins the item's files; Save uploads and attaches
    // it before it saves the changes (#2916).
    const fake = draftHandlers.backend(SLUG)
    server.use(
      ...fake.handlers,
      ...groupHandlers.list([{ id: "g1", slug: SLUG, name: "Household", group_currency: "USD" }]),
      ...customFieldHandlers.list(SLUG, [
        customFieldHandlers.field({
          id: "cf-plate",
          name: "Plate",
          templates: [{ item_type_id: "type-equipment", default_value: "ZZ" }],
        }),
      ])
    )
    const onSubmit = vi.fn().mockResolvedValue(undefined)
    renderWithProviders({
      initialPath: `/g/${SLUG}/items`,
      routes: (
        <Route
          path="/g/:groupSlug/items"
          element={
            <GroupProvider>
              <CommodityFormDialog
                open
                onOpenChange={() => {}}
                mode="edit"
                initialValues={existing}
                areas={areas}
                locations={locations}
                defaultCurrency="USD"
                onSubmit={onSubmit}
              />
            </GroupProvider>
          }
        />
      ),
    })
    const user = userEvent.setup()
    await screen.findByLabelText(/^Name$/i)
    await user.click(screen.getByRole("button", { name: /^back$/i }))

    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: { original_price_currency: { value: "EUR", confidence: 0.9 } },
        custom_fields: { "cf-plate": { value: "AB 123", confidence: 0.9 } },
      })
    )
    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), receipt)
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")

    expect(
      screen.getByTestId("commodity-form-ai-row-original_price_currency-yours")
    ).toHaveTextContent("USD")
    expect(
      screen.getByTestId("commodity-form-ai-row-original_price_currency-check")
    ).not.toBeChecked()
    expect(screen.getByTestId("commodity-form-ai-row-custom-cf-plate-yours")).toHaveTextContent(
      "ZZ"
    )
    expect(screen.getByTestId("commodity-form-ai-row-custom-cf-plate-check")).not.toBeChecked()

    await user.click(screen.getByTestId("commodity-form-ai-use-values"))
    await screen.findByTestId("commodity-form-basics-step")
    await user.click(screen.getByTestId("commodity-form-submit"))
    await waitFor(() => expect(onSubmit).toHaveBeenCalledTimes(1))
    expect(onSubmit.mock.calls[0][0]).toMatchObject({
      original_price_currency: "USD",
      custom_fields: { "cf-plate": "ZZ" },
    })
    expect(fake.files.map((f) => f.links.map((l) => l.entity_id))).toEqual([[existing.id]])
  })
})

describe("<CommodityFormDialog /> files the server draft holds (#2941)", () => {
  const groups = [{ id: "g1", slug: SLUG, name: "Household", group_currency: "USD" }]
  const label = new File([new Uint8Array([1, 2])], "label.png", {
    type: "image/png",
    lastModified: 1,
  })

  beforeEach(() => {
    clearAuth()
    __resetHttpForTests()
    window.localStorage.clear()
    window.sessionStorage.clear()
    server.use(http.get(apiUrl(`/currencies`), () => HttpResponse.json(["USD", "EUR"])))
  })

  afterEach(() => {
    __resetGroupContextForTests()
  })

  // A draft saved with the receipt, titled and tagged by an earlier scan.
  function draftWithReceipt() {
    const fake = draftHandlers.backend(SLUG, [
      { id: "d1", values: { name: "Bosch drill", short_name: "Drill", type: "furniture" } },
    ])
    fake.files.push({
      id: "file-receipt",
      title: "Hardware store receipt",
      path: "scan-17",
      ext: ".pdf",
      size_bytes: receipt.size,
      category: "documents",
      tags: ["invoice"],
      links: [{ id: "link-1", file_id: "file-receipt", entity_type: "draft", entity_id: "d1" }],
    })
    server.use(...groupHandlers.list(groups), ...fake.handlers)
    return fake
  }

  // openDraft opens draft d1; a draft that has a type opens on Purchase.
  async function openDraft() {
    setAccessToken("good-token")
    renderWithProviders({
      initialPath: `/g/${SLUG}/commodities/new`,
      routes: (
        <Route
          path="/g/:groupSlug/commodities/new"
          element={
            <GroupProvider>
              <CommodityFormDialog
                open
                onOpenChange={() => {}}
                mode="create"
                areas={areas}
                locations={locations}
                defaultCurrency="USD"
                onSubmit={async () => {}}
                draftKey={`commodity-draft:${SLUG}:create`}
                serverDrafts
                resumeDraftId="d1"
              />
            </GroupProvider>
          }
        />
      ),
    })
    await screen.findByTestId("commodity-form-purchase-step")
  }

  // The names of the staged files, in their order.
  function stagedNames() {
    return screen
      .queryAllByTestId("commodity-files-pending")
      .map((row) =>
        ["scan-17.pdf", "IMG_0001.jpg", "label.png"].find((n) => within(row).queryByText(n))
      )
  }

  function savedRows() {
    return within(screen.getByTestId("draft-saved-files")).getAllByRole("listitem")
  }

  it("does not stage again a scanned file the draft holds", async () => {
    draftWithReceipt()
    const user = userEvent.setup()
    await openDraft()
    // The scan sends the receipt again, with a new photo.
    await backToScan(user)
    await scan(user, [receipt, photo], {
      fields: COMPLETE,
      files: [{ index: 0, document_type: { value: "invoice", confidence: 0.9 } }],
    })
    await walkToFiles(user)

    await waitFor(() => expect(stagedNames()).toEqual(["IMG_0001.jpg"]))
    expect(savedRows()).toHaveLength(1)
    expect(savedRows()[0]).toHaveTextContent("scan-17.pdf")
  })

  it("does not stage again a picked file the draft holds", async () => {
    draftWithReceipt()
    const user = userEvent.setup()
    await openDraft()
    for (let i = 0; i < 3; i++) await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-form-files-step")
    await waitFor(() => expect(savedRows()).toHaveLength(1))

    const input = screen
      .getByTestId("commodity-files-dropzone")
      .querySelector("input[type=file]") as HTMLInputElement
    await user.upload(input, [receipt, label])
    await waitFor(() => expect(stagedNames()).toEqual(["label.png"]))
    expect(savedRows()).toHaveLength(1)
  })

  // #3023: finding the files the draft holds reads each picked file, which
  // takes a while for a large one. An Add item clicked meanwhile still saves
  // the file.
  it("saves a file picked just before Add item while it is still being read", async () => {
    const fake = draftWithReceipt()
    let release = () => {}
    const reading = new Promise<void>((resolve) => {
      release = resolve
    })
    const digest = crypto.subtle.digest.bind(crypto.subtle)
    const spy = vi.spyOn(crypto.subtle, "digest").mockImplementation(async (algorithm, data) => {
      await reading
      return digest(algorithm, data)
    })
    onTestFinished(() => spy.mockRestore())
    const user = userEvent.setup()
    await openDraft()
    for (let i = 0; i < 3; i++) await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-form-files-step")
    await waitFor(() => expect(savedRows()).toHaveLength(1))

    const input = screen
      .getByTestId("commodity-files-dropzone")
      .querySelector("input[type=file]") as HTMLInputElement
    await user.upload(input, [label])
    await user.click(screen.getByTestId("commodity-form-submit"))
    release()

    await waitFor(() => expect(fake.uploads).toBe(1))
    await waitFor(() =>
      expect(
        fake.files.filter((f) => f.links.some((link) => link.entity_id === "d1")).map((f) => f.id)
      ).toHaveLength(2)
    )
  })
})
