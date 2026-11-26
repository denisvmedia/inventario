import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import {
  AiScanStep,
  type ScanAcceptedValues,
  type ScanAcceptMeta,
  type ScanEnteredValues,
} from "@/components/items/AiScanStep"
import type { FileSuggestion } from "@/features/commodities/fileSuggestions"
import { server } from "@/test/server"
import { apiUrl, commodityScanHandlers, customFieldHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"

const SLUG = "g"

// Deterministic File factories so MSW matching + the dropzone's staging-time
// MIME/extension guard both stay predictable. jsdom's URL.createObjectURL is
// a no-op stub, so previews are irrelevant — we only assert behaviour.
function makeImage(name = "front.jpg"): File {
  return new File([new Uint8Array([0xff, 0xd8, 0xff])], name, { type: "image/jpeg" })
}
function makePdf(name = "receipt.pdf"): File {
  return new File([new Uint8Array([0x25, 0x50, 0x44, 0x46])], name, { type: "application/pdf" })
}

// AiScanStep reads the active group slug off its prop, not a GroupProvider,
// so it renders standalone — but useScanCommodityPhotos routes through the
// http wrapper's group-rewrite, which needs the module-level slug set.
function renderStep(overrides?: {
  onAccept?: (
    values: ScanAcceptedValues,
    meta: ScanAcceptMeta,
    files: File[],
    fileSuggestions: FileSuggestion[]
  ) => void
  onSkip?: () => void
  readEntered?: () => ScanEnteredValues
}) {
  setCurrentGroupSlug(SLUG)
  // Wrapped rather than defaulted: an override would widen the type to a plain
  // function and the callers read .mock.calls off it.
  const onAccept = vi.fn(overrides?.onAccept)
  const onSkip = vi.fn(overrides?.onSkip)
  renderWithProviders({
    children: (
      <AiScanStep
        slug={SLUG}
        defaultCurrency="USD"
        onAccept={onAccept}
        onSkip={onSkip}
        readEntered={overrides?.readEntered}
      />
    ),
  })
  return { onAccept, onSkip }
}

describe("<AiScanStep /> source-file retention (#1983 Part A)", () => {
  afterEach(() => {
    __resetGroupContextForTests()
  })

  beforeEach(() => {
    // The currency-validation query fires once a file is staged.
    server.use(
      http.get(apiUrl(`/currencies`), () => HttpResponse.json(["USD", "EUR", "GBP", "CZK"]))
    )
  })

  it("hands the scanned image back to onAccept when the user accepts", async () => {
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: { name: { value: "Sony WH-1000XM5", confidence: 0.92 } },
      })
    )
    const user = userEvent.setup()
    const { onAccept } = renderStep()

    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), makeImage())
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")

    // The review phase advertises that the source file will be kept.
    expect(screen.getByTestId("commodity-form-ai-attach-note")).toHaveTextContent(
      /will be attached/i
    )

    await user.click(screen.getByTestId("commodity-form-ai-use-values"))

    expect(onAccept).toHaveBeenCalledTimes(1)
    const [values, , files] = onAccept.mock.calls[0]
    expect(values.name).toBe("Sony WH-1000XM5")
    expect(files).toHaveLength(1)
    expect(files[0].name).toBe("front.jpg")
    expect(files[0].type).toBe("image/jpeg")
  })

  it("hands back every staged source file (image + PDF)", async () => {
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: { name: { value: "Mixed", confidence: 0.9 } },
      })
    )
    const user = userEvent.setup()
    const { onAccept } = renderStep()

    const input = await screen.findByTestId("commodity-form-ai-file-input")
    await user.upload(input, [makeImage("a.jpg"), makePdf("b.pdf")])
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")
    await user.click(screen.getByTestId("commodity-form-ai-use-values"))

    const [, , files] = onAccept.mock.calls[0]
    expect(files.map((f: File) => f.name).sort()).toEqual(["a.jpg", "b.pdf"])
  })

  it("does not retain files when the user picks Fill manually instead of accepting", async () => {
    const user = userEvent.setup()
    const { onAccept, onSkip } = renderStep()

    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), makeImage())
    // Skip straight from the offer phase — no scan, no accept.
    await user.click(screen.getByTestId("commodity-form-ai-fill-manually"))

    expect(onSkip).toHaveBeenCalledTimes(1)
    expect(onAccept).not.toHaveBeenCalled()
  })
})

describe("<AiScanStep /> seller in the anonymous flow (#2804)", () => {
  beforeEach(() => {
    server.use(
      http.get(apiUrl(`/currencies`), () => HttpResponse.json(["USD", "EUR", "GBP", "CZK"]))
    )
  })

  function publicScan(fields: Record<string, { value: string; confidence: number }>) {
    server.use(
      http.post(apiUrl(`/public/commodities/scan`), () =>
        HttpResponse.json({
          data: { type: "commodity_scan", attributes: { fields, warnings: [] } },
        })
      )
    )
  }

  async function scanAnonymously() {
    const user = userEvent.setup()
    const onAccept =
      vi.fn<(values: ScanAcceptedValues, meta: ScanAcceptMeta, files: File[]) => void>()
    renderWithProviders({
      children: (
        <AiScanStep slug="" anonymous defaultCurrency="USD" onAccept={onAccept} onSkip={vi.fn()} />
      ),
    })
    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), makeImage())
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")
    await user.click(screen.getByTestId("commodity-form-ai-use-values"))
    expect(onAccept).toHaveBeenCalledTimes(1)
    return onAccept.mock.calls[0][0]
  }

  it("puts the scanned seller into the comments", async () => {
    publicScan({
      name: { value: "Kettle", confidence: 0.9 },
      seller: { value: "Corner Market", confidence: 0.8 },
    })
    const values = await scanAnonymously()
    expect(values.comments).toBe("Seller: Corner Market")
    expect(values.seller).toBeUndefined()
  })

  it("adds the seller after a comment the scan read", async () => {
    publicScan({
      name: { value: "Kettle", confidence: 0.9 },
      comments: { value: "Paid in cash", confidence: 0.6 },
      seller: { value: "Corner Market", confidence: 0.8 },
    })
    const values = await scanAnonymously()
    expect(values.comments).toBe("Paid in cash\nSeller: Corner Market")
  })
})

describe("<AiScanStep /> custom fields (#2803)", () => {
  afterEach(() => {
    __resetGroupContextForTests()
  })

  beforeEach(() => {
    server.use(
      http.get(apiUrl(`/currencies`), () => HttpResponse.json(["USD", "EUR"])),
      ...customFieldHandlers.list(SLUG, [
        customFieldHandlers.field({
          id: "cf-fuel",
          name: "Fuel",
          type: "select",
          options: [{ id: "opt-diesel", label: "Diesel" }],
        }),
        customFieldHandlers.field({ id: "cf-plate", name: "Plate" }),
        customFieldHandlers.field({ id: "cf-awd", name: "AWD", type: "boolean" }),
      ])
    )
  })

  it("reviews the values read for the group's fields and hands back the checked ones", async () => {
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: { name: { value: "Car", confidence: 0.9 } },
        custom_fields: {
          "cf-fuel": { value: "opt-diesel", confidence: 0.8 },
          "cf-plate": { value: "AB 123", confidence: 0.9 },
          "cf-awd": { value: false, confidence: 0.2 },
          "cf-gone": { value: "x", confidence: 0.9 },
        },
      })
    )
    const user = userEvent.setup()
    const { onAccept } = renderStep()
    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), makeImage())
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")

    expect(await screen.findByTestId("commodity-form-ai-row-custom-cf-fuel")).toHaveTextContent(
      "Diesel"
    )
    expect(screen.getByTestId("commodity-form-ai-row-custom-cf-awd")).toHaveTextContent("No")
    // A low confidence starts unchecked; uncheck the plate too.
    expect(screen.getByTestId("commodity-form-ai-row-custom-cf-awd-check")).not.toBeChecked()
    await user.click(screen.getByTestId("commodity-form-ai-row-custom-cf-plate-check"))
    await user.click(screen.getByTestId("commodity-form-ai-use-values"))

    const [values] = onAccept.mock.calls[0]
    expect(values.custom_fields).toEqual({ "cf-fuel": { value: "opt-diesel", currency: "" } })
  })
})

describe("<AiScanStep /> values the form already holds (#1626)", () => {
  afterEach(() => {
    __resetGroupContextForTests()
  })

  beforeEach(() => {
    server.use(
      http.get(apiUrl(`/currencies`), () => HttpResponse.json(["USD", "EUR"])),
      ...customFieldHandlers.list(SLUG, [
        customFieldHandlers.field({ id: "cf-plate", name: "Plate" }),
      ])
    )
  })

  async function scan(readEntered: () => ScanEnteredValues) {
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: {
          name: { value: "Bosch drill", confidence: 0.9 },
          brand: { value: "Bosch", confidence: 0.9 },
          tags: { value: ["tools", "garage"], confidence: 0.8 },
        },
        custom_fields: { "cf-plate": { value: "AB 123", confidence: 0.9 } },
      })
    )
    const user = userEvent.setup()
    const { onAccept } = renderStep({ readEntered })
    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), makeImage())
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")
    return { user, onAccept }
  }

  it("shows the user's value beside the scan's and leaves the field unticked", async () => {
    const { user, onAccept } = await scan(() => ({
      fields: { name: "My drill", tags: "workshop" },
      customFields: { "cf-plate": { value: "XY 999", currency: "" } },
    }))

    expect(screen.getByTestId("commodity-form-ai-row-name-yours")).toHaveTextContent("My drill")
    expect(screen.getByTestId("commodity-form-ai-row-name-value")).toHaveTextContent("Bosch drill")
    expect(screen.getByTestId("commodity-form-ai-row-name-check")).not.toBeChecked()
    expect(screen.getByTestId("commodity-form-ai-row-tags-check")).not.toBeChecked()
    expect(screen.getByTestId("commodity-form-ai-row-tags")).toHaveTextContent(/added to yours/i)
    expect(
      await screen.findByTestId("commodity-form-ai-row-custom-cf-plate-yours")
    ).toHaveTextContent("XY 999")
    expect(screen.getByTestId("commodity-form-ai-row-custom-cf-plate-check")).not.toBeChecked()
    // A field the form leaves empty starts ticked as before.
    expect(screen.getByTestId("commodity-form-ai-row-brand-check")).toBeChecked()
    expect(screen.queryByTestId("commodity-form-ai-row-brand-yours")).toBeNull()

    await user.click(screen.getByTestId("commodity-form-ai-use-values"))
    const [values] = onAccept.mock.calls[0]
    expect(values.name).toBeUndefined()
    expect(values.tags).toBeUndefined()
    expect(values.brand).toBe("Bosch")
    expect(values.custom_fields).toBeUndefined()
  })

  it("hands back the scan's value for a field the user ticks", async () => {
    const { user, onAccept } = await scan(() => ({
      fields: { name: "My drill" },
      customFields: {},
    }))
    await user.click(screen.getByTestId("commodity-form-ai-row-name-check"))
    await user.click(screen.getByTestId("commodity-form-ai-use-values"))
    expect(onAccept.mock.calls[0][0].name).toBe("Bosch drill")
  })
})

describe("<AiScanStep /> file suggestions (#1626)", () => {
  afterEach(() => {
    __resetGroupContextForTests()
  })

  beforeEach(() => {
    server.use(http.get(apiUrl(`/currencies`), () => HttpResponse.json(["USD", "EUR"])))
  })

  it("hands back the suggestion for each file by its place in the request", async () => {
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: { name: { value: "Drill", confidence: 0.9 } },
        files: [
          {
            index: 1,
            document_type: { value: "receipt", confidence: 0.9 },
            title: { value: "Hardware store receipt", confidence: 0.8 },
          },
          { index: 0, document_type: { value: "photo", confidence: 0.9 } },
        ],
      })
    )
    const user = userEvent.setup()
    const { onAccept } = renderStep()
    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), [
      makeImage("drill.jpg"),
      makePdf("receipt.pdf"),
    ])
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    await screen.findByTestId("commodity-form-ai-review")
    expect(screen.getByTestId("commodity-form-ai-file-suggestions-note")).toHaveTextContent(
      /2 files/
    )
    await user.click(screen.getByTestId("commodity-form-ai-use-values"))

    const [, , files, suggestions] = onAccept.mock.calls[0]
    expect(files.map((f: File) => f.name)).toEqual(["drill.jpg", "receipt.pdf"])
    expect(suggestions.map((s: FileSuggestion) => [s.file.name, s.tag, s.title])).toEqual([
      ["receipt.pdf", "invoice", "Hardware store receipt"],
      ["drill.jpg", "photo", undefined],
    ])
  })

  it("reaches the review when the scan only has suggestions for the files", async () => {
    server.use(
      ...commodityScanHandlers.ok(SLUG, {
        fields: {},
        files: [{ index: 0, document_type: { value: "manual", confidence: 0.9 } }],
      })
    )
    const user = userEvent.setup()
    renderStep()
    await user.upload(await screen.findByTestId("commodity-form-ai-file-input"), makePdf())
    await user.click(screen.getByTestId("commodity-form-ai-scan"))
    expect(await screen.findByTestId("commodity-form-ai-file-suggestions-note")).toHaveTextContent(
      /1 file\b/
    )
  })
})
