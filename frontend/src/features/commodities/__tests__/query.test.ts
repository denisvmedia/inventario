import { describe, expect, it } from "vitest"

import {
  COMMODITY_QUERY_URL_PARAMS,
  DEFAULT_COMMODITY_QUERY,
  commodityListHref,
  commodityQueryToListOptions,
  commodityTagHref,
  hasCommodityFilters,
  listOptionsToParams,
  parseCommodityQuery,
  primarySearchMatch,
  writeCommodityQuery,
  type CommodityQuery,
} from "../query"

const FULL: CommodityQuery = {
  text: '"power tools" red',
  types: ["equipment", "other"],
  statuses: ["in_use", "sold"],
  areaId: "a1",
  unassigned: true,
  includeInactive: true,
  warrantyStatuses: ["active", "expired"],
  lentOut: true,
  tags: { include: ["red", "garage"], mode: "any", exclude: ["broken"], untagged: false },
  sort: "purchase_date",
  sortDesc: true,
}

describe("parseCommodityQuery / writeCommodityQuery", () => {
  it("reads an empty URL as the default query", () => {
    expect(parseCommodityQuery(new URLSearchParams())).toEqual(DEFAULT_COMMODITY_QUERY)
  })

  it("round-trips every condition through the URL", () => {
    const params = writeCommodityQuery(new URLSearchParams(), FULL)
    expect(parseCommodityQuery(new URLSearchParams(params.toString()))).toEqual(FULL)
  })

  it("leaves defaults out of the URL", () => {
    expect(writeCommodityQuery(new URLSearchParams(), DEFAULT_COMMODITY_QUERY).toString()).toBe("")
  })

  it("keeps the parameters it does not own and replaces its own", () => {
    const params = new URLSearchParams("page=3&view=list&q=old&type=furniture&tags=a")
    const next = writeCommodityQuery(params, { ...DEFAULT_COMMODITY_QUERY, text: "new" })
    expect(next.get("page")).toBe("3")
    expect(next.get("view")).toBe("list")
    expect(next.get("q")).toBe("new")
    expect(next.getAll("type")).toEqual([])
    expect(next.has("tags")).toBe(false)
    // The input is not modified.
    expect(params.get("q")).toBe("old")
  })

  it("reads tags comma-separated or repeated, without duplicates", () => {
    const q = parseCommodityQuery(new URLSearchParams("tags=a,b&tags=b, c&tags_exclude=x"))
    expect(q.tags).toEqual({
      include: ["a", "b", "c"],
      mode: "all",
      exclude: ["x"],
      untagged: false,
    })
  })

  it("keeps an unsupported filter value, so the server can refuse it", () => {
    const q = parseCommodityQuery(new URLSearchParams("status=broken&warranty=soon"))
    expect(q.statuses).toEqual(["broken"])
    expect(q.warrantyStatuses).toEqual(["soon"])
  })

  it("falls back to the name sort for an unknown sort field", () => {
    expect(parseCommodityQuery(new URLSearchParams("sort=-colour"))).toMatchObject({
      sort: "name",
      sortDesc: false,
    })
  })

  it("owns exactly the parameters it writes", () => {
    const written = [...new Set([...writeCommodityQuery(new URLSearchParams(), FULL).keys()])]
    for (const name of written) expect(COMMODITY_QUERY_URL_PARAMS).toContain(name)
  })
})

describe("commodityListHref", () => {
  it("builds the list URL for a partial query", () => {
    expect(
      commodityListHref("home base", {
        tags: { ...DEFAULT_COMMODITY_QUERY.tags, include: ["red"] },
      })
    ).toBe("/g/home%20base/commodities?tags=red")
    expect(commodityListHref("h", { text: "drill", includeInactive: true })).toBe(
      "/g/h/commodities?q=drill&inactive=1"
    )
    expect(commodityListHref("h", {})).toBe("/g/h/commodities")
  })

  it("links one tag, escaping its slug", () => {
    expect(commodityTagHref("h", "usb-кабель")).toBe(
      `/g/h/commodities?tags=${encodeURIComponent("usb-кабель")}`
    )
  })
})

describe("hasCommodityFilters", () => {
  it("ignores the sort and counts every filter", () => {
    expect(hasCommodityFilters({ ...DEFAULT_COMMODITY_QUERY, sort: "count", sortDesc: true })).toBe(
      false
    )
    expect(
      hasCommodityFilters({
        ...DEFAULT_COMMODITY_QUERY,
        tags: { ...DEFAULT_COMMODITY_QUERY.tags, untagged: true },
      })
    ).toBe(true)
    expect(hasCommodityFilters(FULL)).toBe(true)
  })
})

describe("listOptionsToParams", () => {
  it("maps the query onto the API names", () => {
    const params = listOptionsToParams({
      page: 2,
      perPage: 24,
      ...commodityQueryToListOptions(FULL),
    })
    expect(Object.fromEntries([...params.keys()].map((k) => [k, params.getAll(k)]))).toEqual({
      page: ["2"],
      per_page: ["24"],
      type: ["equipment", "other"],
      status: ["in_use", "sold"],
      area_id: ["a1"],
      q: ['"power tools" red'],
      include_inactive: ["true"],
      sort: ["-purchase_date"],
      warranty_status: ["active", "expired"],
      lent_out: ["true"],
      unassigned: ["true"],
      tags: ["garage,red"],
      tag_mode: ["any"],
      tags_exclude: ["broken"],
    })
  })

  it("sends the active-only default and no tag mode for the default query", () => {
    const params = listOptionsToParams(commodityQueryToListOptions(DEFAULT_COMMODITY_QUERY))
    expect(params.toString()).toBe("include_inactive=false&sort=name")
  })

  it("produces one string for options that select the same items", () => {
    const a = listOptionsToParams({ types: ["b", "a"], tags: ["y", "x"] }).toString()
    const b = listOptionsToParams({ types: ["a", "b"], tags: ["x", "y"] }).toString()
    expect(a).toBe(b)
  })

  it("sends no_tags", () => {
    expect(listOptionsToParams({ noTags: true }).toString()).toBe("no_tags=true")
  })
})

describe("primarySearchMatch", () => {
  it("returns nothing when the name matched, since the title shows it", () => {
    expect(
      primarySearchMatch([
        { field: "name", value: "Red drill" },
        { field: "tags", value: "red" },
      ])
    ).toBeUndefined()
  })

  it("returns the first match otherwise", () => {
    expect(
      primarySearchMatch([
        { field: "asset_id", value: "A-0042" },
        { field: "comments", value: "…" },
      ])
    ).toEqual({ field: "asset_id", value: "A-0042" })
    expect(primarySearchMatch(undefined)).toBeUndefined()
    expect(primarySearchMatch([])).toBeUndefined()
  })
})
