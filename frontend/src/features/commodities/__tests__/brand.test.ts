import { describe, expect, it } from "vitest"

import { brandedName, brandModelText } from "../brand"

describe("brandModelText", () => {
  it.each([
    [{ brand: "Bosch", model: "SMV6ZCX42E" }, "Bosch · SMV6ZCX42E"],
    [{ brand: "Bosch", model: "" }, "Bosch"],
    [{ brand: "", model: "SMV6ZCX42E" }, "SMV6ZCX42E"],
    [{ brand: "  ", model: undefined }, ""],
    [{}, ""],
  ])("renders %j as %j", (item, want) => {
    expect(brandModelText(item)).toBe(want)
  })
})

describe("brandedName", () => {
  it.each([
    [{ brand: "Bosch", name: "Dishwasher" }, "Bosch Dishwasher"],
    [{ brand: "Bosch", name: "Bosch dishwasher" }, "Bosch dishwasher"],
    [{ brand: "", name: "Dishwasher" }, "Dishwasher"],
    [{ brand: "Bosch", name: "" }, "Bosch"],
    [{ name: "Dishwasher" }, "Dishwasher"],
  ])("renders %j as %j", (item, want) => {
    expect(brandedName(item)).toBe(want)
  })
})
