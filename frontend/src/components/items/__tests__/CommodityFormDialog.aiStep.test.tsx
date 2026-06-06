import { afterEach, beforeEach, describe, expect, it } from "vitest"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { server } from "@/test/server"
import { apiUrl, commodityScanHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"

const SLUG = "g"

// Helper: build a File with a deterministic name + size so MSW
// matching is predictable and the dropzone preview survives the
// staging-time mime + extension checks. `jsdom`'s `URL.createObjectURL`
// is a no-op stub in vitest (it returns "blob:" + a random uuid),
// which is fine — we only assert testids, never the actual preview.
function makeImage(name = "photo.jpg", type = "image/jpeg"): File {
  return new File([new Uint8Array([0xff, 0xd8, 0xff])], name, { type })
}

// Helper: build a PDF File (#1983) so the document-staging path can be
// asserted. The "%PDF" magic bytes keep any content sniffing happy;
// jsdom never reads the contents.
function makePdf(name = "receipt.pdf"): File {
  return new File([new Uint8Array([0x25, 0x50, 0x44, 0x46])], name, { type: "application/pdf" })
}

// Helper: install the active group slug so `useScanCommodityPhotos`
// hits the right /g/<slug>/commodities/scan route inside the http
// wrapper's group-rewrite logic.
function withGroupSlug() {
  setCurrentGroupSlug(SLUG)
}

const areas = [{ id: "a1", name: "Garage", location_id: "l1" }]
const locations = [{ id: "l1", name: "Home" }]

function renderDialog() {
  withGroupSlug()
  return renderWithProviders({
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

describe("<CommodityFormDialog /> AI scan step", () => {
  // `setCurrentGroupSlug` mutates a module-level singleton; without an
  // explicit reset later test files might inherit `g` as the active
  // slug and route their requests through the wrong /g/<slug>/... path.
  afterEach(() => {
    __resetGroupContextForTests()
  })

  beforeEach(() => {
    // /api/v1/currencies feeds the AI step's currency-validation set
    // and the CurrencyCombobox the Purchase step uses later. Register
    // a default so MSW's "error on unhandled" mode doesn't crash the
    // first render — each test can still override with a tighter
    // handler via `server.use(...)`.
    server.use(
      http.get(apiUrl(`/currencies`), () => HttpResponse.json(["USD", "EUR", "GBP", "CZK"]))
    )
  })

  it("skips the AI step and opens on Basics when enableAiScan is false", async () => {
    withGroupSlug()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          enableAiScan={false}
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    // No AI offer surface — create mode lands directly on a form step
    // (signalled by the footer Next button, which the AI step hides).
    expect(await screen.findByTestId("commodity-form-next")).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-form-ai-step")).not.toBeInTheDocument()
  })

  it("rewinds from Basics back to the AI scan step via Back", async () => {
    const user = userEvent.setup()
    renderDialog()
    // "Fill manually" hands off from the AI offer to Basics.
    await user.click(await screen.findByTestId("commodity-form-ai-fill-manually"))
    expect(await screen.findByLabelText(/^Name$/i)).toBeInTheDocument()
    expect(screen.queryByTestId("commodity-form-ai-step")).not.toBeInTheDocument()
    // Back on the first form step rewinds to the AI offer surface
    // instead of being a dead no-op (the AI step is the create-mode
    // entry, so it's a place the user can return to).
    await user.click(screen.getByRole("button", { name: /^back$/i }))
    expect(await screen.findByTestId("commodity-form-ai-step")).toHaveAttribute(
      "data-ai-phase",
      "offer"
    )
    expect(screen.queryByLabelText(/^Name$/i)).not.toBeInTheDocument()
  })

  it("keeps Back disabled on Basics when the AI step is unavailable", async () => {
    withGroupSlug()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="create"
          enableAiScan={false}
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    // Lands straight on Basics with no scanner to rewind to, so Back has
    // nothing to do and stays disabled.
    expect(await screen.findByTestId("commodity-form-next")).toBeInTheDocument()
    expect(screen.getByRole("button", { name: /^back$/i })).toBeDisabled()
  })

  it("keeps Back active on Basics in edit mode (rewinds to AI scan)", async () => {
    withGroupSlug()
    renderWithProviders({
      children: (
        <CommodityFormDialog
          open
          onOpenChange={() => {}}
          mode="edit"
          initialValues={{
            id: "c1",
            name: "Draft item",
            short_name: "Draft",
            type: "electronics",
            area_id: "a1",
            status: "in_use",
            count: 1,
          }}
          areas={areas}
          locations={locations}
          defaultCurrency="USD"
          onSubmit={async () => {}}
        />
      ),
    })
    // Edit opens on Basics. Back is active because AI vision is enabled and
    // rewinds to the AI scan surface — so a reopened draft can be finished
    // with a scan (the previous create-only gate left Back dead here).
    const back = await screen.findByRole("button", { name: /^back$/i })
    expect(back).toBeEnabled()
    const user = userEvent.setup()
    await user.click(back)
    expect(await screen.findByTestId("commodity-form-ai-step")).toBeInTheDocument()
  })

  it("renders the offer phase by default with no thumbnails", async () => {
    renderDialog()
    expect(await screen.findByTestId("commodity-form-ai-step")).toHaveAttribute(
      "data-ai-phase",
      "offer"
    )
    expect(screen.queryByTestId("commodity-form-ai-thumb")).not.toBeInTheDocument()
    // Scan button is disabled until at least one photo is staged.
    expect(screen.getByTestId("commodity-form-ai-scan")).toBeDisabled()
  })

  it("stages JPG photos and enables the Scan button", async () => {
    const user = userEvent.setup()
    renderDialog()
    const input = await screen.findByTestId("commodity-form-ai-file-input")
    await user.upload(input, makeImage("alpha.jpg"))
    expect(await screen.findByTestId("commodity-form-ai-thumb")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-form-ai-scan")).toBeEnabled()
  })

  it("stages a PDF document with a document tile and enables Scan", async () => {
    const user = userEvent.setup()
    renderDialog()
    const input = await screen.findByTestId("commodity-form-ai-file-input")
    await user.upload(input, makePdf("receipt.pdf"))
    // A PDF can't render as an <img> thumbnail, so it stages as a
    // document tile carrying the filename — but it still counts as a
    // scannable source, so the Scan button enables (#1983 Part B).
    const tile = await screen.findByTestId("commodity-form-ai-thumb-pdf")
    expect(tile).toHaveTextContent("receipt.pdf")
    expect(screen.getByTestId("commodity-form-ai-scan")).toBeEnabled()
  })

  it("rejects an EXE with a typed staging error", async () => {
    // `applyAccept: false` so userEvent doesn't drop the file pre-React
    // — the FE staging-time MIME check is what we're asserting, not the
    // browser's `<input accept>` filter (browsers themselves treat
    // `accept` as a hint and Android Chrome routinely returns an empty
    // MIME type that bypasses the attribute anyway).
    const user = userEvent.setup({ applyAccept: false })
    renderDialog()
    const input = await screen.findByTestId("commodity-form-ai-file-input")
    await user.upload(
      input,
      new File([new Uint8Array([0x4d, 0x5a])], "bad.exe", { type: "application/x-msdownload" })
    )
    expect(await screen.findByTestId("commodity-form-ai-staging-error")).toHaveTextContent(/JPG/i)
    expect(screen.queryByTestId("commodity-form-ai-thumb")).not.toBeInTheDocument()
  })

  it("walks through scanning into the review phase and renders one row per field", async () => {
    server.use(
      ...commodityScanHandlers.slow(
        SLUG,
        {
          fields: {
            name: { value: "Sony WH-1000XM5", confidence: 0.92 },
            short_name: { value: "Sony XM5", confidence: 0.84 },
            original_price: { value: 399, confidence: 0.4 },
            serial_number: { value: "1234ABCD", confidence: 0.15 },
          },
        },
        50
      )
    )
    const user = userEvent.setup()
    renderDialog()
    const input = await screen.findByTestId("commodity-form-ai-file-input")
    await user.upload(input, makeImage("front.jpg"))
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    // Scanning phase appears while the (delayed) mock is in flight.
    expect(await screen.findByTestId("commodity-form-ai-scanning")).toBeInTheDocument()
    // Review phase follows; one row per BE-returned field.
    expect(await screen.findByTestId("commodity-form-ai-review")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-form-ai-row-name")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-form-ai-row-short_name")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-form-ai-row-original_price")).toBeInTheDocument()
    expect(screen.getByTestId("commodity-form-ai-row-serial_number")).toBeInTheDocument()
    // Low-confidence (< 0.3) defaults to UNCHECKED so we don't smuggle a guess into the form.
    const lowRow = screen.getByTestId("commodity-form-ai-row-serial_number-check")
    expect(lowRow).toHaveAttribute("data-state", "unchecked")
    // High-confidence stays default-checked.
    expect(screen.getByTestId("commodity-form-ai-row-name-check")).toHaveAttribute(
      "data-state",
      "checked"
    )
  })

  it("applies accepted values and advances to Basics", async () => {
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: {
          name: { value: "Sony WH-1000XM5", confidence: 0.92 },
        },
      })
    )
    const user = userEvent.setup()
    renderDialog()
    await user.upload(
      await screen.findByTestId("commodity-form-ai-file-input"),
      makeImage("front.jpg")
    )
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")
    await user.click(screen.getByTestId("commodity-form-ai-use-values"))
    const nameInput = (await screen.findByLabelText(/^Name$/i)) as HTMLInputElement
    expect(nameInput.value).toBe("Sony WH-1000XM5")
  })

  it("queues the scanned source files into the Files step on accept (#1983)", async () => {
    // Return a complete, high-confidence field set so accepting prefills
    // every required field — the wizard can then walk straight to the Files
    // step without manual entry, where the retained source files surface.
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: {
          name: { value: "Sony WH-1000XM5", confidence: 0.95 },
          short_name: { value: "Sony XM5", confidence: 0.95 },
          type: { value: "electronics", confidence: 0.95 },
          original_price: { value: 399, confidence: 0.95 },
          original_price_currency: { value: "USD", confidence: 0.95 },
          purchase_date: { value: "2024-01-15", confidence: 0.95 },
        },
      })
    )
    const user = userEvent.setup()
    renderDialog()

    // Scan one image + one PDF, then accept.
    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), [
      makeImage("front.jpg"),
      makePdf("receipt.pdf"),
    ])
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")
    await user.click(screen.getByTestId("commodity-form-ai-use-values"))

    // Walk Basics → Purchase → Warranty → Extras → Files. Every required
    // field was prefilled by the scan, so each Next passes validation.
    await screen.findByTestId("commodity-form-basics-step")
    for (let i = 0; i < 4; i++) {
      await user.click(screen.getByTestId("commodity-form-next"))
    }

    const filesStep = await screen.findByTestId("commodity-form-files-step")
    // Both scanned files are now staged in the Files step — the image and the
    // PDF — ready for the post-create uploadPendingFiles() attach.
    expect(within(filesStep).getByText("front.jpg")).toBeInTheDocument()
    expect(within(filesStep).getByText("receipt.pdf")).toBeInTheDocument()
  })

  it("accepts a server-supported non-default currency (CZK) guessed from the scan", async () => {
    // defaultCurrency is USD; the /currencies mock (beforeEach) includes CZK.
    // A Czech-invoice guess must therefore be pre-fillable — not dropped as
    // "unsupported" just because it isn't the group default.
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: {
          original_price: { value: 1290, confidence: 0.9 },
          original_price_currency: { value: "CZK", confidence: 0.9 },
        },
      })
    )
    const user = userEvent.setup()
    renderDialog()
    await user.upload(
      await screen.findByTestId("commodity-form-ai-file-input"),
      makePdf("invoice.pdf")
    )
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")
    const row = screen.getByTestId("commodity-form-ai-row-original_price_currency")
    // Once /currencies resolves, CZK is recognised: no "won't be pre-filled"
    // note, and the row stays default-checked so the value carries over.
    await waitFor(() =>
      expect(within(row).queryByText(/won't be pre-filled/i)).not.toBeInTheDocument()
    )
    expect(
      screen.getByTestId("commodity-form-ai-row-original_price_currency-check")
    ).toHaveAttribute("data-state", "checked")
  })

  it("renders a warranty_expires_at review row from the scan", async () => {
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: {
          warranty_expires_at: { value: "2027-05-01", confidence: 0.7 },
        },
      })
    )
    const user = userEvent.setup()
    renderDialog()
    await user.upload(
      await screen.findByTestId("commodity-form-ai-file-input"),
      makePdf("manual.pdf")
    )
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")
    expect(screen.getByTestId("commodity-form-ai-row-warranty_expires_at-value")).toHaveTextContent(
      "2027-05-01"
    )
  })

  it("renders a tags review row from the scan", async () => {
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: { tags: { value: ["coffee", "kitchen"], confidence: 0.7 } },
      })
    )
    const user = userEvent.setup()
    renderDialog()
    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), makePdf("x.pdf"))
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")
    expect(screen.getByTestId("commodity-form-ai-row-tags-value")).toHaveTextContent(
      "coffee, kitchen"
    )
  })

  it("surfaces the multiple_items warning when the document has several products", async () => {
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: { name: { value: "First Item", confidence: 0.9 } },
        warnings: [{ code: "multiple_items" }],
      })
    )
    const user = userEvent.setup()
    renderDialog()
    await user.upload(
      await screen.findByTestId("commodity-form-ai-file-input"),
      makePdf("receipt.pdf")
    )
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")
    // The field-less warning is localized in the global review banner.
    expect(screen.getByText(/more than one item/i)).toBeInTheDocument()
  })

  it("truncates an over-long short_name guess to the 40-char form limit", async () => {
    const long = "X".repeat(60)
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: { short_name: { value: long, confidence: 0.9 } },
      })
    )
    const user = userEvent.setup()
    renderDialog()
    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), makePdf("x.pdf"))
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")
    await user.click(screen.getByTestId("commodity-form-ai-use-values"))
    const shortInput = (await screen.findByLabelText(/^Short name$/i)) as HTMLInputElement
    expect(shortInput.value).toHaveLength(40)
  })

  it("shows a chooser for a multi-item scan and the pick drives the review", async () => {
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: { name: { value: "Coffee Machine", confidence: 0.9 } },
        items: [
          { fields: { name: { value: "Coffee Machine", confidence: 0.9 } } },
          { fields: { name: { value: "Milk Frother", confidence: 0.8 } } },
        ],
      })
    )
    const user = userEvent.setup()
    renderDialog()
    await user.upload(
      await screen.findByTestId("commodity-form-ai-file-input"),
      makePdf("receipt.pdf")
    )
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    // Two candidates → the chooser, not review.
    await screen.findByTestId("commodity-form-ai-choose")
    expect(screen.getByTestId("commodity-form-ai-step")).toHaveAttribute("data-ai-phase", "choose")
    expect(screen.getByTestId("commodity-form-ai-choose-item-0")).toHaveTextContent(
      "Coffee Machine"
    )
    expect(screen.getByTestId("commodity-form-ai-choose-item-1")).toHaveTextContent("Milk Frother")
    // Pick the second → review pre-fills that item's fields.
    await user.click(screen.getByTestId("commodity-form-ai-choose-item-1"))
    await screen.findByTestId("commodity-form-ai-review")
    expect(screen.getByTestId("commodity-form-ai-row-name-value")).toHaveTextContent("Milk Frother")
  })

  it("skips the chooser when only one item is detected", async () => {
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: { name: { value: "Solo Item", confidence: 0.9 } },
        items: [{ fields: { name: { value: "Solo Item", confidence: 0.9 } } }],
      })
    )
    const user = userEvent.setup()
    renderDialog()
    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), makePdf("x.pdf"))
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    // One item → straight to review, no chooser.
    await screen.findByTestId("commodity-form-ai-review")
    expect(screen.queryByTestId("commodity-form-ai-choose")).not.toBeInTheDocument()
  })

  it("renders the rate-limited banner on 429 and keeps Fill manually usable", async () => {
    server.use(
      ...commodityScanHandlers.error(SLUG, 429, "commodity_scan.rate_limited", "slow down")
    )
    const user = userEvent.setup()
    renderDialog()
    await user.upload(
      await screen.findByTestId("commodity-form-ai-file-input"),
      makeImage("front.jpg")
    )
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    const banner = await screen.findByTestId("commodity-form-ai-error")
    expect(within(banner).getByText(/too many scans/i)).toBeInTheDocument()
    // Fill manually still routes to Basics — typed errors don't block the manual path.
    await user.click(screen.getByTestId("commodity-form-ai-fill-manually"))
    await waitFor(() => expect(screen.getByLabelText(/^Name$/i)).toBeInTheDocument())
  })

  it("shows a retry hint instead of an empty review when nothing is extracted", async () => {
    server.use(...commodityScanHandlers.ok(SLUG, { fields: {} }))
    const user = userEvent.setup()
    renderDialog()
    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), makePdf("x.pdf"))
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    // Stays on the offer with a hint, not a green-but-empty review.
    expect(await screen.findByTestId("commodity-form-ai-staging-error")).toHaveTextContent(
      /couldn't read any details/i
    )
    expect(screen.queryByTestId("commodity-form-ai-review")).not.toBeInTheDocument()
  })

  it("renders the provider-disabled banner on 503", async () => {
    server.use(
      ...commodityScanHandlers.error(SLUG, 503, "commodity_scan.provider_disabled", "provider off")
    )
    const user = userEvent.setup()
    renderDialog()
    await user.upload(
      await screen.findByTestId("commodity-form-ai-file-input"),
      makeImage("front.jpg")
    )
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    const banner = await screen.findByTestId("commodity-form-ai-error")
    // Both the AlertTitle and the AlertDescription carry the override
    // copy in this case, so anchor on the title-text alone — that's
    // what the typed `commodity_scan.provider_disabled` code maps to.
    expect(within(banner).getByText(/AI vision is unavailable/i)).toBeInTheDocument()
  })

  it("aborts the in-flight scan when Cancel is clicked", async () => {
    server.use(...commodityScanHandlers.slow(SLUG, { fields: {} }, 5_000))
    const user = userEvent.setup()
    renderDialog()
    await user.upload(
      await screen.findByTestId("commodity-form-ai-file-input"),
      makeImage("front.jpg")
    )
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-scanning")
    await user.click(screen.getByTestId("commodity-form-ai-cancel"))
    // After Cancel the offer phase should be back on screen — the
    // dropzone is the canonical offer-phase marker.
    await waitFor(() =>
      expect(screen.getByTestId("commodity-form-ai-dropzone")).toBeInTheDocument()
    )
  })
})
