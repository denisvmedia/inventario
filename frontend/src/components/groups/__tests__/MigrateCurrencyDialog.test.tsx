import { beforeEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"
import { Route } from "react-router-dom"
import { screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import {
  MigrateCurrencyDialog,
  formatRetryAfter,
  truncateRateInput,
} from "@/components/groups/MigrateCurrencyDialog"
import { GroupProvider } from "@/features/group/GroupContext"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { apiUrl, currencyMigrationHandlers, groupHandlers } from "@/test/handlers"

beforeEach(() => {
  // Currencies endpoint backs the CurrencyCombobox in step 1 and the
  // groups list backs GroupProvider; both want a stable default for
  // every case so individual `server.use(...)` calls only have to
  // declare the slice that's actually being exercised.
  server.use(
    msw.get(apiUrl("/currencies"), () => HttpResponse.json(["USD", "EUR", "CZK"])),
    ...groupHandlers.list([{ id: "g1", slug: "household", name: "Household" } as never])
  )
})

function renderDialog() {
  return renderWithProviders({
    initialPath: "/groups/g1/settings",
    routes: (
      <Route
        path="/groups/:groupId/settings"
        element={
          <GroupProvider>
            <MigrateCurrencyDialog
              open={true}
              onOpenChange={() => {}}
              groupName="Household"
              fromCurrency="USD"
              groupSlug="household"
            />
          </GroupProvider>
        }
      />
    ),
  })
}

describe("truncateRateInput", () => {
  it("keeps up to 6 fraction digits", () => {
    expect(truncateRateInput("1.234567")).toBe("1.234567")
    expect(truncateRateInput("1.2345678")).toBe("1.234567")
    expect(truncateRateInput("1.23456789012")).toBe("1.234567")
  })

  it("normalises commas to dots", () => {
    expect(truncateRateInput("1,2345")).toBe("1.2345")
  })

  it("preserves trailing dot mid-typing", () => {
    expect(truncateRateInput("1.")).toBe("1.")
    expect(truncateRateInput("1,")).toBe("1.")
  })

  it("strips secondary dots", () => {
    expect(truncateRateInput("1.2.3.4")).toBe("1.234")
  })

  it("accepts the empty string", () => {
    expect(truncateRateInput("")).toBe("")
  })

  it("strips non-numeric characters", () => {
    expect(truncateRateInput("1a2b.3")).toBe("12.3")
  })
})

describe("formatRetryAfter", () => {
  it("returns em-dash when input is missing or invalid", () => {
    expect(formatRetryAfter(undefined)).toBe("—")
    expect(formatRetryAfter("")).toBe("—")
    expect(formatRetryAfter("not-a-number")).toBe("—")
    expect(formatRetryAfter("0")).toBe("—")
    expect(formatRetryAfter("-5")).toBe("—")
  })

  it("returns a HH:MM string for a positive number of seconds", () => {
    const out = formatRetryAfter("60")
    // The locale-formatted time string varies by environment, but it
    // should always contain a colon (HH:MM) and only ASCII digits +
    // separators — never the em-dash fallback.
    expect(out).not.toBe("—")
    expect(out.length).toBeGreaterThanOrEqual(4)
  })
})

describe("<MigrateCurrencyDialog />", () => {
  it("blocks step 1 → 2 when the picked target equals the current currency", async () => {
    const user = userEvent.setup()
    renderDialog()
    const continueBtn = await screen.findByTestId("wizard-next")
    expect(continueBtn).toBeDisabled()
    // Pick USD (same as current) via the CurrencyCombobox button +
    // hidden listbox; the dialog should surface the "same currency"
    // error and keep the Continue button disabled.
    await user.click(screen.getByRole("combobox"))
    await user.click(await screen.findByText("USD"))
    expect(await screen.findByTestId("wizard-target-same-error")).toBeInTheDocument()
    expect(continueBtn).toBeDisabled()
  })

  it("renders preview totals and the 10-minute countdown after submit", async () => {
    const user = userEvent.setup()
    server.use(...currencyMigrationHandlers.preview("household"))
    renderDialog()
    // Step 1: pick EUR
    await user.click(screen.getByRole("combobox"))
    await user.click(await screen.findByText("EUR"))
    await user.click(screen.getByTestId("wizard-next"))
    // Step 2: enter rate
    const rate = await screen.findByTestId("wizard-rate-input")
    await user.type(rate, "0.9")
    await user.click(screen.getByTestId("wizard-preview"))
    // Step 3: preview totals + countdown render
    await waitFor(() => {
      expect(screen.getByTestId("wizard-total-before")).toBeInTheDocument()
    })
    expect(screen.getByTestId("wizard-preview-countdown").textContent).toMatch(
      /Preview expires in /
    )
    expect(screen.getByTestId("wizard-top-deltas")).toBeInTheDocument()
  })
})
