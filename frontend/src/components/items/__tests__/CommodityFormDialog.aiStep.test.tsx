import { beforeEach, describe, expect, it } from "vitest"
import { screen, waitFor, within } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { server } from "@/test/server"
import { apiUrl, commodityScanHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { setCurrentGroupSlug } from "@/lib/group-context"

const SLUG = "g"

// Helper: build a File with a deterministic name + size so MSW
// matching is predictable and the dropzone preview survives the
// staging-time mime + extension checks. `jsdom`'s `URL.createObjectURL`
// is a no-op stub in vitest (it returns "blob:" + a random uuid),
// which is fine — we only assert testids, never the actual preview.
function makeImage(name = "photo.jpg", type = "image/jpeg"): File {
  return new File([new Uint8Array([0xff, 0xd8, 0xff])], name, { type })
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
  beforeEach(() => {
    // /api/v1/currencies feeds the AI step's currency-validation set
    // and the CurrencyCombobox the Purchase step uses later. Register
    // a default so MSW's "error on unhandled" mode doesn't crash the
    // first render — each test can still override with a tighter
    // handler via `server.use(...)`.
    server.use(
      http.get(apiUrl(`/g/${SLUG}/currencies`), () =>
        HttpResponse.json(["USD", "EUR", "GBP", "CZK"])
      )
    )
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
