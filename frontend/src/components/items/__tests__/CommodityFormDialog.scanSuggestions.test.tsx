import { afterEach, beforeEach, describe, expect, it } from "vitest"
import { screen, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { apiUrl, commodityScanHandlers } from "@/test/handlers"
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
      fields: { ...COMPLETE, tags: { value: ["tools"], confidence: 0.9 } },
    })
    await backToScan(user)
    await scan(
      user,
      [photo],
      { fields: { tags: { value: ["Tools", "garage"], confidence: 0.9 } } },
      ["tags"]
    )
    for (let i = 0; i < 3; i++) await user.click(screen.getByTestId("commodity-form-next"))
    await screen.findByTestId("commodity-tags-input")
    const chips = screen.getAllByTestId("commodity-tags-chip")
    expect(chips.map((c) => c.textContent)).toEqual([
      expect.stringContaining("tools"),
      expect.stringContaining("garage"),
    ])
  })
})
