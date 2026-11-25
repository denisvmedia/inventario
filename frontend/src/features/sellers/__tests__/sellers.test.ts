import { beforeEach, describe, expect, it } from "vitest"

import {
  findSellerByName,
  listSellers,
  SELLER_NAME_MAX,
  sellerNameKey,
  type Seller,
} from "@/features/sellers/api"
import { isHttpUrl, sellerFormSchema } from "@/features/sellers/schemas"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { sellerHandlers } from "@/test/handlers"
import { server } from "@/test/server"

const sellers: Seller[] = [
  { id: "s1", name: "Hardware Store", item_count: 2 },
  { id: "s2", name: "Café Ré", item_count: 0 },
]

describe("sellerNameKey / findSellerByName", () => {
  it("ignores case, outer spaces and runs of spaces", () => {
    expect(sellerNameKey("  Hardware \t  STORE ")).toBe("hardware store")
    expect(findSellerByName(sellers, "hardware   store")?.id).toBe("s1")
  })

  it("matches a composed name against a decomposed one", () => {
    // "e" + combining acute, as some keyboards and scans produce it.
    expect(findSellerByName(sellers, "café ré")?.id).toBe("s2")
  })

  it("folds case the way the server does", () => {
    // Full case folding expands ß and ẞ and treats every sigma alike.
    expect(sellerNameKey("Straße")).toBe(sellerNameKey("STRASSE"))
    expect(sellerNameKey("GROẞ")).toBe("gross")
    expect(sellerNameKey("ΟΔΟΣ")).toBe(sellerNameKey("οδος"))
    expect(sellerNameKey("ﬁne")).toBe("fine")
    // The dotless ı has no folding of its own.
    expect(sellerNameKey("Bıçak")).not.toBe(sellerNameKey("Bicak"))
  })

  it("splits on the white space the server knows", () => {
    expect(sellerNameKey("Corner\u00a0Market")).toBe("corner market")
    expect(sellerNameKey("Corner\u0085Market")).toBe("corner market")
    // U+FEFF is not white space to the server.
    expect(sellerNameKey("Corner\ufeffMarket")).toBe("corner\ufeffmarket")
  })

  it("finds nothing for an empty name or an unknown one", () => {
    expect(findSellerByName(sellers, "   ")).toBeUndefined()
    expect(findSellerByName(sellers, "Corner Market")).toBeUndefined()
  })
})

describe("listSellers", () => {
  beforeEach(() => {
    clearAuth()
    __resetGroupContextForTests()
    __resetHttpForTests()
    setAccessToken("good-token")
    setCurrentGroupSlug("household")
  })

  it("unwraps the JSON:API resources into sellers with their ids", async () => {
    server.use(...sellerHandlers.list("household", sellers))
    expect(await listSellers()).toEqual(sellers)
  })
})

describe("sellerFormSchema", () => {
  const valid = { name: " Shop ", website: "", contact: "", note: "" }

  it("trims the values", () => {
    expect(sellerFormSchema.parse({ ...valid, website: " https://shop.example " })).toEqual({
      name: "Shop",
      website: "https://shop.example",
      contact: "",
      note: "",
    })
  })

  it.each([
    ["no name", { ...valid, name: "  " }, "sellers:validation.nameRequired"],
    [
      "a name over the limit",
      { ...valid, name: "a".repeat(SELLER_NAME_MAX + 1) },
      "sellers:validation.nameTooLong",
    ],
    [
      "a website without a scheme",
      { ...valid, website: "shop.example" },
      "sellers:validation.websiteInvalid",
    ],
    [
      "an ftp website",
      { ...valid, website: "ftp://shop.example" },
      "sellers:validation.websiteInvalid",
    ],
  ])("rejects %s", (_name, input, message) => {
    const result = sellerFormSchema.safeParse(input)
    expect(result.success).toBe(false)
    expect(result.error?.issues.map((i) => i.message)).toContain(message)
  })

  it("counts code points, as the server counts runes", () => {
    // 100 emoji are 200 UTF-16 units and still fit.
    expect(
      sellerFormSchema.safeParse({ ...valid, name: "🛒".repeat(SELLER_NAME_MAX) }).success
    ).toBe(true)
  })

  it("accepts http and https addresses only", () => {
    expect(isHttpUrl("http://shop.example")).toBe(true)
    expect(isHttpUrl("https://shop.example/a?b=c")).toBe(true)
    expect(isHttpUrl("mailto:ann@shop.example")).toBe(false)
    expect(isHttpUrl("https://")).toBe(false)
  })
})
