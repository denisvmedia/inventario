import { describe, expect, it } from "vitest"

import {
  COMMODITIES_PAGE_SIZE,
  commodityHref,
  commoditiesListOptions,
  commoditiesListSource,
  isCommoditiesListContext,
  parseCommoditiesListParams,
} from "@/features/commodities/listContext"
import { commodityKeys } from "@/features/commodities/keys"
import { parseListContext } from "@/lib/list-context"

const SLUG = "home"
const SCOPE = `/g/${SLUG}`

describe("parseCommoditiesListParams", () => {
  it("reads the page and every filter the Items list writes to its URL", () => {
    const params = parseCommoditiesListParams(
      new URLSearchParams(
        "page=3&q=lamp&type=electronics&type=furniture&status=sold&area=a1&warranty=expiring&lent_out=1&unassigned=1&inactive=1&tags=red,blue&tag_mode=any&sort=-purchase_date"
      )
    )
    expect(params).toEqual({
      page: 3,
      query: {
        text: "lamp",
        types: ["electronics", "furniture"],
        statuses: ["sold"],
        areaId: "a1",
        unassigned: true,
        includeInactive: true,
        warrantyStatuses: ["expiring"],
        lentOut: true,
        tags: { include: ["red", "blue"], mode: "any", exclude: [], untagged: false },
        sort: "purchase_date",
        sortDesc: true,
      },
    })
  })

  it("falls back to page 1 and name order for junk values", () => {
    const params = parseCommoditiesListParams(new URLSearchParams("page=abc&sort=colour"))
    expect(params.page).toBe(1)
    expect(params.query.sort).toBe("name")
    expect(params.query.sortDesc).toBe(false)
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
