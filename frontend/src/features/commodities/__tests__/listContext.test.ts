import { afterEach, describe, expect, it } from "vitest"

import {
  COMMODITIES_PAGE_SIZE,
  commodityHref,
  commoditiesListOptions,
  commoditiesListSource,
  isCommoditiesListContext,
  parseCommoditiesListParams,
  parsePageSize,
  storeCommoditiesPageSize,
} from "@/features/commodities/listContext"
import { commodityKeys } from "@/features/commodities/keys"
import { parseListContext } from "@/lib/list-context"

const SLUG = "home"
const SCOPE = `/g/${SLUG}`

afterEach(() => {
  localStorage.removeItem("commodities:pageSize")
})

describe("parseCommoditiesListParams", () => {
  it("reads the page and every filter the Items list writes to its URL", () => {
    const params = parseCommoditiesListParams(
      new URLSearchParams(
        "page=3&q=lamp&type=electronics&type=furniture&status=sold&area=a1&seller=s1&warranty=expiring&lent_out=1&unassigned=1&inactive=1&tags=red,blue&tag_mode=any&cf=f1:set&sort=-purchase_date"
      )
    )
    expect(params).toEqual({
      page: 3,
      perPage: COMMODITIES_PAGE_SIZE,
      query: {
        text: "lamp",
        types: ["electronics", "furniture"],
        statuses: ["sold"],
        areaId: "a1",
        unassigned: true,
        sellerIds: ["s1"],
        includeInactive: true,
        warrantyStatuses: ["expiring"],
        lentOut: true,
        tags: { include: ["red", "blue"], mode: "any", exclude: [], untagged: false },
        customFields: ["f1:set"],
        sort: "purchase_date",
        sortDesc: true,
        customFieldSort: "",
        unreadable: {},
      },
    })
  })

  it("falls back to page 1 and name order for junk values", () => {
    const params = parseCommoditiesListParams(new URLSearchParams("page=abc&sort=colour"))
    expect(params.page).toBe(1)
    expect(params.query.sort).toBe("name")
    expect(params.query.sortDesc).toBe(false)
  })

  it("reads the page size from the URL, then from the last choice on this device", () => {
    expect(parseCommoditiesListParams(new URLSearchParams("per_page=96")).perPage).toBe(96)
    expect(parseCommoditiesListParams(new URLSearchParams("per_page=50")).perPage).toBe(
      COMMODITIES_PAGE_SIZE
    )
    storeCommoditiesPageSize(48)
    expect(parseCommoditiesListParams(new URLSearchParams("")).perPage).toBe(48)
    expect(parseCommoditiesListParams(new URLSearchParams("page=1")).perPage).toBe(48)
    expect(parseCommoditiesListParams(new URLSearchParams("per_page=24")).perPage).toBe(24)
  })

  it("counts pages of 24 past the first page when the URL names no size", () => {
    storeCommoditiesPageSize(48)
    expect(parseCommoditiesListParams(new URLSearchParams("page=3")).perPage).toBe(24)
    expect(parseCommoditiesListParams(new URLSearchParams("page=3&per_page=48")).perPage).toBe(48)
  })

  it("ignores the size picked on this device when asked to", () => {
    storeCommoditiesPageSize(96)
    const params = new URLSearchParams("")
    expect(parseCommoditiesListParams(params, { remembered: false }).perPage).toBe(24)
  })
})

describe("parsePageSize", () => {
  it("accepts only the offered sizes", () => {
    expect(parsePageSize("24")).toBe(24)
    expect(parsePageSize("48")).toBe(48)
    expect(parsePageSize("96")).toBe(96)
    expect(parsePageSize("100")).toBeUndefined()
    expect(parsePageSize("")).toBeUndefined()
    expect(parsePageSize(null)).toBeUndefined()
  })
})

describe("commoditiesListOptions", () => {
  it("builds the request the list page sends", () => {
    const opts = commoditiesListOptions(
      parseCommoditiesListParams(new URLSearchParams("q=lamp&warranty=active"))
    )
    expect(opts).toEqual({
      page: 1,
      perPage: COMMODITIES_PAGE_SIZE,
      search: "lamp",
      types: [],
      statuses: [],
      areaId: undefined,
      unassigned: undefined,
      includeInactive: false,
      warrantyStatuses: ["active"],
      lentOut: undefined,
      tags: undefined,
      tagMode: undefined,
      tagsExclude: undefined,
      noTags: undefined,
      sort: "name",
      sortDesc: false,
    })
  })
})

describe("commoditiesListSource", () => {
  it("queries each page under the list page's own query key", () => {
    const ctx = parseListContext(`${SCOPE}/commodities?q=lamp&page=2`, SCOPE)!
    const source = commoditiesListSource(ctx, SLUG)
    expect(source.page).toBe(2)
    expect(source.pageSize).toBe(COMMODITIES_PAGE_SIZE)
    const listPageOpts = commoditiesListOptions(
      parseCommoditiesListParams(new URLSearchParams("q=lamp&page=3"))
    )
    expect(source.pageQuery(3).queryKey).toEqual(commodityKeys.list(SLUG, listPageOpts))
  })

  it("breaks the pages where the list did, at the list's page size", () => {
    const ctx = parseListContext(`${SCOPE}/commodities?per_page=96&page=2`, SCOPE)!
    const source = commoditiesListSource(ctx, SLUG)
    expect(source.pageSize).toBe(96)
    const listPageOpts = commoditiesListOptions(
      parseCommoditiesListParams(new URLSearchParams("per_page=96&page=2"))
    )
    expect(source.pageQuery(2).queryKey).toEqual(commodityKeys.list(SLUG, listPageOpts))
  })

  it("reads the list's size from the context, not from this device", () => {
    // Another tab may have picked a size since the list wrote its URL.
    storeCommoditiesPageSize(96)
    const ctx = parseListContext(`${SCOPE}/commodities?q=lamp`, SCOPE)!
    expect(commoditiesListSource(ctx, SLUG).pageSize).toBe(24)
  })

  it("reduces a page to its ids and the list total", () => {
    const ctx = parseListContext(`${SCOPE}/commodities`, SCOPE)!
    const query = commoditiesListSource(ctx, SLUG).pageQuery(1)
    expect(
      query.select({
        commodities: [{ id: "c1" }, { id: "c2" }],
        total: 30,
        covers: {},
      })
    ).toEqual({ ids: ["c1", "c2"], total: 30 })
  })
})

describe("isCommoditiesListContext", () => {
  it("accepts only the group's Items list", () => {
    expect(isCommoditiesListContext(parseListContext(`${SCOPE}/commodities`, SCOPE)!, SLUG)).toBe(
      true
    )
    expect(isCommoditiesListContext(parseListContext(`${SCOPE}/files`, SCOPE)!, SLUG)).toBe(false)
    expect(isCommoditiesListContext(parseListContext(`${SCOPE}/commodities`, SCOPE)!, "")).toBe(
      false
    )
  })
})

describe("commodityHref", () => {
  it("links the item page or its edit form, carrying the list", () => {
    const ctx = parseListContext(`${SCOPE}/commodities?q=lamp`, SCOPE)
    expect(commodityHref(SLUG, "c 1", null)).toBe(`${SCOPE}/commodities/c%201`)
    expect(commodityHref(SLUG, "c1", ctx)).toBe(
      `${SCOPE}/commodities/c1?from=${encodeURIComponent(`${SCOPE}/commodities?q=lamp`)}`
    )
    expect(commodityHref(SLUG, "c1", ctx, { edit: true })).toBe(
      `${SCOPE}/commodities/c1/edit?from=${encodeURIComponent(`${SCOPE}/commodities?q=lamp`)}`
    )
  })
})
