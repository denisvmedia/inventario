import { describe, expect, it } from "vitest"

import {
  COMMODITY_QUERY_URL_PARAMS,
  DEFAULT_COMMODITY_QUERY,
  commodityListHref,
  commodityQueryToListOptions,
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
  unreadable: {},
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

  it("keeps an unreadable flag or tag mode, so the server can refuse it", () => {
    const q = parseCommodityQuery(
      new URLSearchParams(
        "unassigned=invalid&inactive=maybe&lent_out=yes&no_tags=x&tags=red&tag_mode=some"
      )
    )
    expect(q).toMatchObject({
      unassigned: false,
      includeInactive: false,
      lentOut: false,
      tags: { include: ["red"], mode: "all", exclude: [], untagged: false },
      unreadable: {
        unassigned: "invalid",
        inactive: "maybe",
        lent_out: "yes",
        no_tags: "x",
        tag_mode: "some",
      },
    })
    // Written back as they were, so the URL keeps them.
    const written = writeCommodityQuery(new URLSearchParams(), q)
    expect(parseCommodityQuery(written)).toEqual(q)
    expect(hasCommodityFilters({ ...DEFAULT_COMMODITY_QUERY, unreadable: q.unreadable })).toBe(true)
  })

  it("reads booleans and tag modes the way the server does", () => {
    const q = parseCommodityQuery(
      new URLSearchParams("unassigned=TRUE&inactive=0&lent_out=false&no_tags=&tag_mode=all")
    )
    expect(q).toMatchObject({ unassigned: true, includeInactive: false, lentOut: false })
    expect(q.tags.untagged).toBe(false)
    expect(q.unreadable).toEqual({})
  })

  it("drops an unreadable value once the parameter gets a readable one", () => {
    const q = parseCommodityQuery(new URLSearchParams("lent_out=yes"))
    const next = writeCommodityQuery(new URLSearchParams(), { ...q, lentOut: true })
    expect(next.toString()).toBe("lent_out=1")
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

  it("sends unreadable values as written, under their API names", () => {
    const q = parseCommodityQuery(
      new URLSearchParams("unassigned=invalid&inactive=maybe&lent_out=yes&no_tags=x&tag_mode=some")
    )
    const params = listOptionsToParams(commodityQueryToListOptions(q))
    expect(params.get("unassigned")).toBe("invalid")
    expect(params.getAll("include_inactive")).toEqual(["maybe"])
    expect(params.get("lent_out")).toBe("yes")
    expect(params.get("no_tags")).toBe("x")
    expect(params.get("tag_mode")).toBe("some")
  })

  it("sends tag_mode=any with tags and nothing for the default mode", () => {
    const any = parseCommodityQuery(new URLSearchParams("tags=a&tag_mode=any"))
    expect(listOptionsToParams(commodityQueryToListOptions(any)).get("tag_mode")).toBe("any")
    const all = parseCommodityQuery(new URLSearchParams("tags=a&tag_mode=all"))
    expect(listOptionsToParams(commodityQueryToListOptions(all)).has("tag_mode")).toBe(false)
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
