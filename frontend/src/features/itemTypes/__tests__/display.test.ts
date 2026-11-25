import type { TFunction } from "i18next"
import { Car, Laptop, Package } from "lucide-react"
import { beforeAll, describe, expect, it } from "vitest"

import type { ItemType } from "@/features/itemTypes/api"
import {
  BUILTIN_ITEM_TYPE_ICON_LOOKUP,
  findItemTypeByName,
  isBuiltinItemTypeSlug,
  itemTypeIconFor,
  itemTypeLabel,
} from "@/features/itemTypes/display"
import { ITEM_TYPE_ICON_LOOKUP, itemTypeIcon } from "@/features/itemTypes/icons"
import { i18next, initI18n } from "@/i18n"
import { nameKey } from "@/lib/name-key"

function type(slug: string, name = "", icon = ""): ItemType {
  return { id: `id-${slug}`, slug, name, icon, builtin: isBuiltinItemTypeSlug(slug) }
}

const types = [type("electronics", "", "laptop"), type("vehicle", "Vehicle", "car")]

let t: TFunction

// The Czech catalogs load lazily, which the test setup does not do, so the
// Czech names come from a stub.
const CZECH: Record<string, string> = {
  "commodities:type.white_goods": "Spotřebič",
  "commodities:type.electronics": "Elektronika",
}
const czech = ((key: string) => CZECH[key] ?? key) as unknown as TFunction

beforeAll(async () => {
  await initI18n({ lng: "en" })
  t = i18next.t.bind(i18next)
})

describe("itemTypeLabel", () => {
  it("shows a member's name, else the translated default, else the slug", async () => {
    expect(itemTypeLabel(t, type("vehicle", "Vehicle"))).toBe("Vehicle")
    expect(itemTypeLabel(t, type("white_goods"))).toBe("Appliance")
    expect(itemTypeLabel(t, type("white_goods", "Kitchen machines"))).toBe("Kitchen machines")
    expect(itemTypeLabel(t, type("spaceship"))).toBe("spaceship")

    expect(itemTypeLabel(czech, type("white_goods"))).toBe("Spotřebič")
    // A custom name stays as typed in every language.
    expect(itemTypeLabel(czech, type("vehicle", "Vehicle"))).toBe("Vehicle")
  })
})

describe("item type icons", () => {
  it("maps a known key and falls back to the package icon", () => {
    expect(itemTypeIcon("car")).toBe(Car)
    expect(itemTypeIcon("no-such-icon")).toBe(Package)
    expect(itemTypeIcon("")).toBe(Package)
    // The lookup has no prototype, so inherited names find nothing.
    expect(ITEM_TYPE_ICON_LOOKUP.constructor).toBeUndefined()
    expect(itemTypeIcon("toString")).toBe(Package)
  })

  it("uses a built-in slug's default icon when the type is not at hand", () => {
    expect(itemTypeIconFor(undefined, "electronics")).toBe(Laptop)
    expect(itemTypeIconFor(undefined, "vehicle")).toBe(Package)
    expect(itemTypeIconFor(type("vehicle", "Vehicle", "car"), "vehicle")).toBe(Car)
    expect(BUILTIN_ITEM_TYPE_ICON_LOOKUP.electronics).toBe(Laptop)
    expect(BUILTIN_ITEM_TYPE_ICON_LOOKUP.vehicle).toBeUndefined()
  })
})

describe("findItemTypeByName", () => {
  it("matches names the way the server does", () => {
    expect(findItemTypeByName(types, "  VEHICLE ")?.slug).toBe("vehicle")
    // A built-in type keeps its English default name on the server.
    expect(findItemTypeByName(types, "electronics")?.slug).toBe("electronics")
    expect(findItemTypeByName(types, "Boat")).toBeUndefined()
    expect(findItemTypeByName(types, "   ")).toBeUndefined()
  })

  it("leaves a built-in type's translated name free", () => {
    // The server knows only the English default, so "Elektronika" can name
    // a type of its own.
    expect(findItemTypeByName(types, "elektronika")).toBeUndefined()
  })
})

describe("nameKey", () => {
  it("folds case and collapses white space", () => {
    expect(nameKey("  Straße  Car ")).toBe("strasse car")
    expect(nameKey("ΣΊΣΥΦΟΣ")).toBe(nameKey("σίσυφοσ"))
  })
})
