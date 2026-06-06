import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { screen } from "@testing-library/react"
import userEvent from "@testing-library/user-event"
import { http, HttpResponse } from "msw"

import {
  AiScanStep,
  type ScanAcceptedValues,
  type ScanAcceptMeta,
} from "@/components/items/AiScanStep"
import { server } from "@/test/server"
import { apiUrl, commodityScanHandlers } from "@/test/handlers"
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
  onAccept?: (values: ScanAcceptedValues, meta: ScanAcceptMeta, files: File[]) => void
  onSkip?: () => void
}) {
  setCurrentGroupSlug(SLUG)
  const onAccept = overrides?.onAccept ?? vi.fn()
  const onSkip = overrides?.onSkip ?? vi.fn()
  renderWithProviders({
    children: <AiScanStep slug={SLUG} defaultCurrency="USD" onAccept={onAccept} onSkip={onSkip} />,
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
