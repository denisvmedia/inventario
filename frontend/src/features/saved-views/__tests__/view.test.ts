import { describe, expect, it } from "vitest"

import { DEFAULT_COMMODITY_QUERY, parseCommodityQuery } from "@/features/commodities/query"

import {
  SAVED_VIEW_ICON_NAMES,
  SUGGESTED_VIEWS,
  hasStaleReferences,
  isSavedViewChanged,
  savedViewHref,
  savedViewQuery,
  staleReferences,
} from "../view"

describe("savedViewHref", () => {
  it("opens the items list with the view's query, layout and ID", () => {
    const href = savedViewHref("home", {
      id: "sv1",
      query: "include_inactive=false&sort=-purchase_date&status=sold&tags=red",
      layout: "list",
    })
    const url = new URL(href ?? "", "http://x")
    expect(url.pathname).toBe("/g/home/commodities")
    expect(url.searchParams.get("saved_view")).toBe("sv1")
    expect(url.searchParams.get("view")).toBe("list")
    const query = parseCommodityQuery(url.searchParams)
    expect(query.statuses).toEqual(["sold"])
    expect(query.tags.include).toEqual(["red"])
    expect(query.sort).toBe("purchase_date")
    expect(query.sortDesc).toBe(true)
    expect(query.includeInactive).toBe(false)
  })

  it("leaves the layout to the viewer when the view keeps none", () => {
    const href = savedViewHref("home", { id: "sv1", query: "lent_out=true", layout: "" })
    const url = new URL(href ?? "", "http://x")
    expect(url.searchParams.has("view")).toBe(false)
    // An absent include_inactive is the server default: inactive included.
    expect(url.searchParams.get("inactive")).toBe("1")
  })

  // The list reads only the parameters it knows: a link would drop the one
  // the server could not read and open a broader list than the view.
  it("gives no link to a view whose stored query cannot run", () => {
    const href = savedViewHref("home", {
      id: "sv1",
      query: "type=electronics&warranty_expires_before=2026-12-01",
      layout: "",
      queryError: { parameter: "warranty_expires_before", reason: "unknown parameter" },
    })
    expect(href).toBeNull()
  })
})

describe("isSavedViewChanged", () => {
  const view = {
    query: "include_inactive=false&sort=name&type=other&type=equipment",
    layout: "" as const,
  }

  it("is false for the same filter in another order", () => {
    const query = { ...DEFAULT_COMMODITY_QUERY, types: ["equipment", "other"] }
    expect(isSavedViewChanged(view, query, "grid")).toBe(false)
  })

  it("is true when a condition or the sort differs", () => {
    expect(isSavedViewChanged(view, DEFAULT_COMMODITY_QUERY, "grid")).toBe(true)
    const resorted = { ...DEFAULT_COMMODITY_QUERY, types: ["equipment", "other"], sortDesc: true }
    expect(isSavedViewChanged(view, resorted, "grid")).toBe(true)
  })

  it("counts the layout only when the view keeps one", () => {
    const query = savedViewQuery(view)
    expect(isSavedViewChanged(view, query, "list")).toBe(false)
    expect(isSavedViewChanged({ ...view, layout: "grid" }, query, "list")).toBe(true)
  })

  it("tells the compact cards from the photo cards", () => {
    const query = savedViewQuery(view)
    const compact = { ...view, layout: "compact" as const }
    expect(isSavedViewChanged(compact, query, "compact")).toBe(false)
    expect(isSavedViewChanged(compact, query, "grid")).toBe(true)
    expect(savedViewHref("household", { ...compact, id: "sv1" })).toContain("view=compact")
  })
})

describe("staleReferences", () => {
  it("names a missing area and missing tags only", () => {
    const query = {
      ...DEFAULT_COMMODITY_QUERY,
      areaId: "a9",
      tags: { include: ["red", "gone"], mode: "all" as const, exclude: ["old"], untagged: false },
    }
    const stale = staleReferences(query, new Set(["a1"]), new Set(["red"]))
    expect(stale).toEqual({ area: true, tags: ["gone", "old"], sellers: [] })
    expect(hasStaleReferences(stale)).toBe(true)
  })

  it("finds nothing when everything still exists", () => {
    const query = { ...DEFAULT_COMMODITY_QUERY, areaId: "a1" }
    const stale = staleReferences(query, new Set(["a1"]), new Set())
    expect(hasStaleReferences(stale)).toBe(false)
  })

  it("names deleted sellers once the sellers are known, and none before", () => {
    const query = { ...DEFAULT_COMMODITY_QUERY, sellerIds: ["s1", "s9"] }
    const known = staleReferences(query, new Set(), new Set(), new Set(["s1"]))
    expect(known.sellers).toEqual(["s9"])
    expect(hasStaleReferences(known)).toBe(true)
    const unknown = staleReferences(query, new Set(), new Set())
    expect(unknown.sellers).toEqual([])
    expect(hasStaleReferences(unknown)).toBe(false)
  })
})

describe("SUGGESTED_VIEWS", () => {
  it("keeps queries the items list can show, with relative dates only", () => {
    for (const s of SUGGESTED_VIEWS) {
      expect(SAVED_VIEW_ICON_NAMES).toContain(s.icon)
      expect(s.query).not.toContain("warranty_expires_before")
      expect(s.query).toContain("include_inactive=false")
    }
    const expiring = SUGGESTED_VIEWS.find((s) => s.key === "expiringWarranties")
    expect(expiring?.query).toContain("warranty_status=expiring")
  })
})
