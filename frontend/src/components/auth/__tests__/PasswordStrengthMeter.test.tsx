import { afterEach, beforeEach, describe, expect, it } from "vitest"
import { render, screen, waitFor } from "@testing-library/react"
import { axe } from "jest-axe"

import {
  PasswordStrengthMeter,
  scorePassword,
  __resetZxcvbnLoader,
} from "@/components/auth/PasswordStrengthMeter"
import { zxcvbnSpies } from "@/test/global-mocks"

// test/setup.ts mocks the zxcvbn-ts modules for the whole suite: the real
// package would load the ~150 KB English dictionary on every test that mounts
// the meter, and leak microtasks across tests through its module-level loader
// cache. The spies behind that mock are programmable, which is what these
// tests need — scoring and load timing, per test.
//
// v4 configures scoring through the ZxcvbnFactory constructor rather than the
// old zxcvbnOptions.setOptions singleton, so `factory` stands in for the
// construction step and a test can make it throw to simulate a load failure.
const zxcvbnMock = zxcvbnSpies.check
const factoryMock = zxcvbnSpies.factory

beforeEach(() => {
  zxcvbnMock.mockReset()
  factoryMock.mockReset()
  __resetZxcvbnLoader()
})
afterEach(() => {
  __resetZxcvbnLoader()
})

describe("scorePassword (heuristic fallback)", () => {
  it("returns 0 for empty input", () => {
    expect(scorePassword("")).toBe(0)
  })

  it("rewards length and character-class diversity", () => {
    expect(scorePassword("short")).toBe(0)
    expect(scorePassword("eightchr")).toBeGreaterThanOrEqual(1)
    expect(scorePassword("Eightch1")).toBeGreaterThanOrEqual(2)
    expect(scorePassword("LongerOne1")).toBeGreaterThanOrEqual(3)
    expect(scorePassword("LongerOne1!")).toBeGreaterThanOrEqual(3)
    expect(scorePassword("LongerOne123!")).toBe(4)
  })
})

describe("<PasswordStrengthMeter />", () => {
  it("renders the empty hint and omits the bars row when password is blank", () => {
    render(<PasswordStrengthMeter password="" />)
    expect(screen.getByText(/8\+ characters/i)).toBeInTheDocument()
    // Per the #1381 AC ("empty password = no bars rendered") the meter
    // role is only present once the user starts typing.
    expect(screen.queryByRole("meter")).not.toBeInTheDocument()
  })

  it("upgrades to the zxcvbn score and renders the first suggestion", async () => {
    // zxcvbn scores this 2; the synchronous heuristic scores "hunter2" a 1
    // (one length point + one for two character classes). The divergent
    // scores are deliberate: the meter starts at the heuristic 1 and only
    // flips to 2 once the async loader resolves, so waiting on "2" proves we
    // observed the zxcvbn upgrade rather than the heuristic already on screen.
    // Waiting on "1" would race — it's satisfied before the dynamic import
    // settles, so the call assertion below would still see 0 calls.
    zxcvbnMock.mockReturnValue({
      score: 2,
      feedback: { suggestions: ["Add another word or two.", "Avoid repeated patterns."] },
    })
    render(
      <PasswordStrengthMeter
        password="hunter2"
        userInputs={["alex@example.com"]}
        testId="t-meter"
      />
    )
    await waitFor(() => expect(screen.getByRole("meter")).toHaveAttribute("aria-valuenow", "2"))
    expect(zxcvbnMock).toHaveBeenCalledWith("hunter2", ["alex@example.com"])
    expect(screen.getByTestId("t-meter-suggestion")).toHaveTextContent(/another word/i)
  })

  it("falls back to the heuristic when the zxcvbn dynamic import fails", async () => {
    // Constructing the factory throws, simulating a chunk-download failure.
    factoryMock.mockImplementation(() => {
      throw new Error("network offline")
    })
    render(<PasswordStrengthMeter password="LongerOne1" />)
    // Heuristic gives this string a score of 3 — assert it stays put even
    // after the loader rejects.
    await waitFor(() => {
      expect(screen.getByRole("meter")).toHaveAttribute("aria-valuenow", "3")
    })
  })

  it("has no axe violations", async () => {
    zxcvbnMock.mockReturnValue({ score: 2, feedback: { suggestions: [] } })
    const { container } = render(<PasswordStrengthMeter password="LongerOne1" />)
    const results = await axe(container)
    expect(results).toHaveNoViolations()
  })
})
