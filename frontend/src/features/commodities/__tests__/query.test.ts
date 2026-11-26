import { describe, expect, it } from "vitest"

import {
  COMMODITY_QUERY_URL_PARAMS,
  DEFAULT_COMMODITY_QUERY,
  commodityListHref,
  commodityQueryFromApiQuery,
  commodityQueryToApiQuery,
  commodityQueryToListOptions,
  commoditySellerHref,
  commodityTagHref,
  hasCommodityFilters,
  listOptionsToParams,
  parseCommodityQuery,
  primarySearchMatch,
  sameCommodityQuery,
  unreadApiQueryParam,
  unreadableTagParamCount,
  withIncludedTag,
  withTagFilter,
  writeCommodityQuery,
  type CommodityQuery,
} from "../query"

const FULL: CommodityQuery = {
  text: '"power tools" red',
  types: ["equipment", "other"],
  statuses: ["in_use", "sold"],
  areaId: "a1",
  unassigned: true,
  sellerIds: ["s2", "s1"],
  includeInactive: true,
  warrantyStatuses: ["active", "expired"],
  lentOut: true,
  tags: { include: ["red", "garage"], mode: "any", exclude: ["broken"], untagged: false },
  noPhoto: true,
  noPurchasePrice: true,
  noPurchaseDate: true,
  customFields: ["f2:in:o2,o1", "f1:gte:10"],
  sort: "purchase_date",
  sortDesc: true,
  customFieldSort: "",
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

  it("round-trips a custom field sort through the URL", () => {
    const query = { ...DEFAULT_COMMODITY_QUERY, customFieldSort: "f1", sortDesc: true }
    const params = writeCommodityQuery(new URLSearchParams(), query)
    expect(params.get("sort")).toBe("-cf.f1")
    expect(parseCommodityQuery(new URLSearchParams(params.toString()))).toEqual(query)
    expect(parseCommodityQuery(new URLSearchParams("sort=cf.")).customFieldSort).toBe("")
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
        "unassigned=invalid&inactive=maybe&lent_out=yes&no_tags=x&tags=red&tag_mode=some" +
          "&no_photo=perhaps&no_purchase_price=unknown&no_purchase_date=2"
      )
    )
    expect(q).toMatchObject({
      unassigned: false,
      includeInactive: false,
      lentOut: false,
      tags: { include: ["red"], mode: "all", exclude: [], untagged: false },
      noPhoto: false,
      noPurchasePrice: false,
      noPurchaseDate: false,
      unreadable: {
        unassigned: "invalid",
        inactive: "maybe",
        lent_out: "yes",
        no_tags: "x",
        tag_mode: "some",
        no_photo: "perhaps",
        no_purchase_price: "unknown",
        no_purchase_date: "2",
      },
    })
    // Written back as they were, so the URL keeps them.
    const written = writeCommodityQuery(new URLSearchParams(), q)
    expect(parseCommodityQuery(written)).toEqual(q)
    expect(hasCommodityFilters({ ...DEFAULT_COMMODITY_QUERY, unreadable: q.unreadable })).toBe(true)
  })

  it("reads booleans and tag modes the way the server does", () => {
    const q = parseCommodityQuery(
      new URLSearchParams(
        "unassigned=TRUE&inactive=0&lent_out=false&no_tags=&tag_mode=all" +
          "&no_photo=1&no_purchase_price=True&no_purchase_date=0"
      )
    )
    expect(q).toMatchObject({
      unassigned: true,
      includeInactive: false,
      lentOut: false,
      noPhoto: true,
      noPurchasePrice: true,
      noPurchaseDate: false,
    })
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

  it("keeps the any mode picked before the first tag", () => {
    const anyOnly: CommodityQuery = {
      ...DEFAULT_COMMODITY_QUERY,
      tags: { ...DEFAULT_COMMODITY_QUERY.tags, mode: "any" },
    }
    const params = writeCommodityQuery(new URLSearchParams(), anyOnly)
    expect(params.toString()).toBe("tag_mode=any")
    expect(parseCommodityQuery(params)).toEqual(anyOnly)
    // The mode alone selects nothing.
    expect(hasCommodityFilters(anyOnly)).toBe(false)
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

  it("links one seller with inactive items included", () => {
    expect(commoditySellerHref("h", "s-1")).toBe("/g/h/commodities?seller=s-1&inactive=1")
  })

  it("links one tag with inactive items included, escaping its slug", () => {
    expect(commodityTagHref("h", "usb-кабель")).toBe(
      `/g/h/commodities?inactive=1&tags=${encodeURIComponent("usb-кабель")}`
    )
  })
})

describe("withTagFilter / withIncludedTag", () => {
  const base: CommodityQuery = {
    ...DEFAULT_COMMODITY_QUERY,
    text: "drill",
    statuses: ["in_use"],
    tags: { include: ["red"], mode: "any", exclude: ["blue"], untagged: false },
  }

  it("adds a tag and keeps every other condition", () => {
    expect(withIncludedTag(base, "green")).toEqual({
      ...base,
      tags: { include: ["red", "green"], mode: "any", exclude: ["blue"], untagged: false },
    })
  })

  it("leaves an included tag as it is", () => {
    expect(withIncludedTag(base, "red")).toEqual(base)
  })

  it("moves an excluded tag to the included ones and turns off No tags", () => {
    expect(withIncludedTag(base, "blue").tags).toEqual({
      include: ["red", "blue"],
      mode: "any",
      exclude: [],
      untagged: false,
    })
    const untagged = {
      ...DEFAULT_COMMODITY_QUERY,
      tags: { ...DEFAULT_COMMODITY_QUERY.tags, untagged: true },
    }
    expect(withIncludedTag(untagged, "red").tags).toEqual({
      include: ["red"],
      mode: "all",
      exclude: [],
      untagged: false,
    })
  })

  it("drops unreadable tag parameters and keeps the others", () => {
    const q = parseCommodityQuery(new URLSearchParams("tag_mode=some&no_tags=x&lent_out=maybe"))
    expect(unreadableTagParamCount(q)).toBe(2)
    const next = withTagFilter(q, DEFAULT_COMMODITY_QUERY.tags)
    expect(next.unreadable).toEqual({ lent_out: "maybe" })
    expect(unreadableTagParamCount(next)).toBe(0)
    expect(withIncludedTag(q, "red").unreadable).toEqual({ lent_out: "maybe" })
    // The input is not modified.
    expect(q.unreadable).toEqual({ tag_mode: "some", no_tags: "x", lent_out: "maybe" })
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
    expect(hasCommodityFilters({ ...DEFAULT_COMMODITY_QUERY, noPhoto: true })).toBe(true)
    expect(hasCommodityFilters({ ...DEFAULT_COMMODITY_QUERY, noPurchasePrice: true })).toBe(true)
    expect(hasCommodityFilters({ ...DEFAULT_COMMODITY_QUERY, noPurchaseDate: true })).toBe(true)
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
      seller_id: ["s1", "s2"],
      tags: ["garage,red"],
      tag_mode: ["any"],
      tags_exclude: ["broken"],
      no_photo: ["true"],
      no_purchase_price: ["true"],
      no_purchase_date: ["true"],
      cf: ["f1:gte:10", "f2:in:o1,o2"],
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
    // Without tags the mode selects nothing, so the request is the same as
    // for the default mode and shares its cache entry.
    const anyOnly = parseCommodityQuery(new URLSearchParams("tag_mode=any"))
    expect(listOptionsToParams(commodityQueryToListOptions(anyOnly)).toString()).toBe(
      listOptionsToParams(commodityQueryToListOptions(DEFAULT_COMMODITY_QUERY)).toString()
    )
  })

  it("reads repeated sellers without duplicates and sends them as seller_id", () => {
    const q = parseCommodityQuery(new URLSearchParams("seller=b&seller=a&seller=b&seller="))
    expect(q.sellerIds).toEqual(["b", "a"])
    expect(hasCommodityFilters({ ...DEFAULT_COMMODITY_QUERY, sellerIds: ["a"] })).toBe(true)
    expect(listOptionsToParams(commodityQueryToListOptions(q)).getAll("seller_id")).toEqual([
      "a",
      "b",
    ])
  })

  it("sends no_tags", () => {
    expect(listOptionsToParams({ noTags: true }).toString()).toBe("no_tags=true")
  })

  it("sends trashed only when it selects the trash", () => {
    expect(listOptionsToParams({ trashed: true, search: "drill" }).toString()).toBe(
      "q=drill&trashed=true"
    )
    expect(listOptionsToParams({ trashed: false }).has("trashed")).toBe(false)
  })
})

describe("commodityQueryToApiQuery / commodityQueryFromApiQuery", () => {
  it("round-trips every condition through the API query string", () => {
    expect(commodityQueryFromApiQuery(commodityQueryToApiQuery(FULL))).toEqual({
      ...FULL,
      types: [...FULL.types].sort(),
      sellerIds: [...FULL.sellerIds].sort(),
      statuses: [...FULL.statuses].sort(),
      warrantyStatuses: [...FULL.warrantyStatuses].sort(),
      tags: { ...FULL.tags, include: [...FULL.tags.include].sort() },
      customFields: ["f1:gte:10", "f2:in:o1,o2"],
    })
  })

  it("writes the sort and the inactive switch even at their defaults", () => {
    expect(commodityQueryToApiQuery(DEFAULT_COMMODITY_QUERY)).toBe(
      "include_inactive=false&sort=name"
    )
  })

  it("reads an absent include_inactive as the server default", () => {
    expect(commodityQueryFromApiQuery("lent_out=true").includeInactive).toBe(true)
    expect(commodityQueryFromApiQuery("include_inactive=false").includeInactive).toBe(false)
  })

  it("maps the API names onto the query", () => {
    const q = commodityQueryFromApiQuery(
      "area_id=a1&warranty_status=expiring&no_tags=true&include_inactive=false&sort=-registered_date"
    )
    expect(q.areaId).toBe("a1")
    expect(q.warrantyStatuses).toEqual(["expiring"])
    expect(q.tags.untagged).toBe(true)
    expect(q.sort).toBe("registered_date")
    expect(q.sortDesc).toBe(true)
  })

  describe("unreadApiQueryParam", () => {
    // The server stores Go's url.Values encoding: keys sorted, and the
    // include_inactive default left out.
    const serverForm = (raw: string) => {
      const entries = [...new URLSearchParams(raw)]
        .filter(([name, value]) => !(name === "include_inactive" && value === "true"))
        .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
      return new URLSearchParams(entries).toString()
    }

    it("reads every condition the list writes, in the server's form", () => {
      expect(unreadApiQueryParam(serverForm(commodityQueryToApiQuery(FULL)))).toBeUndefined()
      expect(
        unreadApiQueryParam(serverForm(commodityQueryToApiQuery(DEFAULT_COMMODITY_QUERY)))
      ).toBeUndefined()
      expect(unreadApiQueryParam("")).toBeUndefined()
      expect(unreadApiQueryParam("lent_out=true")).toBeUndefined()
      expect(unreadApiQueryParam("q=+drill+&sort=name")).toBeUndefined()
      expect(unreadApiQueryParam("sort=name&tags=red%2Cblue")).toBeUndefined()
    })

    it("reads the queries of the inbox's checks", () => {
      for (const raw of [
        "include_inactive=false&unassigned=true",
        "include_inactive=false&no_photo=true",
        "include_inactive=false&no_purchase_price=true",
        "include_inactive=false&no_purchase_date=true",
        "include_inactive=false&no_tags=true",
      ]) {
        expect(unreadApiQueryParam(raw)).toBeUndefined()
      }
      const q = commodityQueryFromApiQuery("include_inactive=false&no_purchase_date=true")
      expect(q).toEqual({ ...DEFAULT_COMMODITY_QUERY, noPurchaseDate: true })
      expect(commodityListHref("home", q)).toBe("/g/home/commodities?no_purchase_date=1")
    })

    // Go's canonical form escapes the colons and sorts the conditions and
    // the options of an "in" condition.
    it("reads custom field conditions and a custom field sort", () => {
      expect(
        unreadApiQueryParam("cf=f1%3Agte%3A10&cf=f2%3Ain%3Ao1%2Co2&cf=f3%3Aset&sort=-cf.f1")
      ).toBeUndefined()
      const q = commodityQueryFromApiQuery("cf=f1%3Agte%3A10&sort=-cf.f1")
      expect(q.customFields).toEqual(["f1:gte:10"])
      expect(q.customFieldSort).toBe("f1")
      expect(q.sortDesc).toBe(true)
      expect(listOptionsToParams(commodityQueryToListOptions(q)).getAll("cf")).toEqual([
        "f1:gte:10",
      ])
    })

    it("names a parameter the list would drop", () => {
      expect(unreadApiQueryParam("item_type=abc&sort=name")).toBe("item_type")
    })

    it("names a value the list would read as something else", () => {
      expect(unreadApiQueryParam("sort=-unknown")).toBe("sort")
      expect(unreadApiQueryParam("sort=unknown")).toBe("sort")
      expect(unreadApiQueryParam("sort=cf.")).toBe("sort")
    })
  })

  it("ignores parameters outside the contract", () => {
    expect(commodityQueryFromApiQuery("page=3&per_page=5&include_inactive=false")).toEqual(
      DEFAULT_COMMODITY_QUERY
    )
  })

  it("compares queries in the stored form", () => {
    const a = { ...DEFAULT_COMMODITY_QUERY, types: ["other", "equipment"] }
    const b = { ...DEFAULT_COMMODITY_QUERY, types: ["equipment", "other"] }
    expect(sameCommodityQuery(a, b)).toBe(true)
    expect(sameCommodityQuery(a, { ...b, sortDesc: true })).toBe(false)
  })

  // The server normalizes tag slugs and drops repeats before it stores them.
  it("compares tag slugs as the server stores them", () => {
    const tags = (include: string[], exclude: string[] = []) => ({
      ...DEFAULT_COMMODITY_QUERY,
      tags: { ...DEFAULT_COMMODITY_QUERY.tags, include, exclude },
    })
    expect(sameCommodityQuery(tags(["Fragile"]), tags(["fragile"]))).toBe(true)
    expect(sameCommodityQuery(tags(["fragile", "FRAGILE"]), tags(["fragile"]))).toBe(true)
    expect(sameCommodityQuery(tags([], ["Power Tools"]), tags([], ["power-tools"]))).toBe(true)
    expect(sameCommodityQuery(tags(["fragile"]), tags(["fragile-2"]))).toBe(false)
    expect(sameCommodityQuery(tags(["fragile"]), tags([], ["fragile"]))).toBe(false)
  })

  // The server sorts the option ids of an "in" condition and drops repeats
  // before it stores them, and drops a condition that repeats another.
  it("compares select option sets as the server stores them", () => {
    const cf = (...customFields: string[]) => ({ ...DEFAULT_COMMODITY_QUERY, customFields })
    expect(sameCommodityQuery(cf("f2:in:o2,o1"), cf("f2:in:o1,o2"))).toBe(true)
    expect(sameCommodityQuery(cf("f2:in:o2, o1,o2"), cf("f2:in:o1,o2"))).toBe(true)
    expect(sameCommodityQuery(cf("f2:in:o1,o2", "f2:in:o2,o1"), cf("f2:in:o1,o2"))).toBe(true)
    expect(sameCommodityQuery(cf("f2:in:o1"), cf("f2:in:o1,o2"))).toBe(false)
    expect(commodityQueryToApiQuery(cf("f2:in:,"))).toContain("cf=f2%3Ain%3A%2C")
    expect(commodityQueryToApiQuery(cf("f1:contains:b,a"))).toContain("cf=f1%3Acontains%3Ab%2Ca")
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
