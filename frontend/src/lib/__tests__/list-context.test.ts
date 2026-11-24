import { describe, expect, it } from "vitest"

import {
  LIST_CONTEXT_PARAM,
  listContextFromLocation,
  listContextHref,
  listContextPage,
  parseListContext,
  readListContext,
  withListContext,
  withListContextPage,
} from "@/lib/list-context"

const SCOPE = "/g/home"

describe("list context", () => {
  it("round-trips a list location through a record URL", () => {
    const ctx = listContextFromLocation({
      pathname: "/g/home/files",
      search: "?category=documents&tags=invoice,receipt&page=2&view=grid",
    })
    const recordUrl = withListContext("/g/home/files/f1/edit", ctx)
    const [, query] = recordUrl.split("?")
    const back = readListContext(new URLSearchParams(query), SCOPE)

    expect(back).toEqual(ctx)
    expect(listContextHref(back!)).toBe(
      "/g/home/files?category=documents&tags=invoice,receipt&page=2&view=grid"
    )
  })

  it("replaces an existing context and keeps the record URL's own query and hash", () => {
    const ctx = { pathname: "/g/home/files", search: "q=lamp" }
    const url = withListContext("/g/home/files/f1/edit?tab=details&from=%2Fold#top", ctx)
    const parsed = new URL(url, "http://app.test")

    expect(parsed.pathname).toBe("/g/home/files/f1/edit")
    expect(parsed.searchParams.get("tab")).toBe("details")
    expect(parsed.searchParams.getAll(LIST_CONTEXT_PARAM)).toEqual(["/g/home/files?q=lamp"])
    expect(parsed.hash).toBe("#top")
  })

  it("leaves the URL alone without a context", () => {
    expect(withListContext("/g/home/files/f1/edit", null)).toBe("/g/home/files/f1/edit")
  })

  it("keeps a list's own context, so a chain of record pages unwinds step by step", () => {
    const list = { pathname: "/g/home/commodities", search: "sort=-price" }
    const itemUrl = withListContext("/g/home/commodities/c1", list)
    const item = listContextFromLocation(new URL(itemUrl, "http://app.test"))
    const fileUrl = withListContext("/g/home/files/f1/edit", item)

    const fromFile = readListContext(new URL(fileUrl, "http://app.test").searchParams, SCOPE)
    expect(fromFile?.pathname).toBe("/g/home/commodities/c1")
    const fromItem = readListContext(new URLSearchParams(fromFile!.search), SCOPE)
    expect(fromItem).toEqual(list)
  })

  it.each([
    ["an absolute URL", "https://evil.example/g/home/files"],
    ["a protocol-relative URL", "//evil.example/g/home/files"],
    ["a backslash host", "/\\evil.example"],
    ["a path outside the group", "/g/other/files"],
    ["a group whose slug only starts the same", "/g/homeless/files"],
    ["a path that climbs out", "/g/home/../admin"],
    ["a relative path", "g/home/files"],
    ["an empty value", "   "],
  ])("rejects %s", (_label, raw) => {
    expect(parseListContext(raw, SCOPE)).toBeNull()
  })

  it("accepts the group root and drops a hash", () => {
    expect(parseListContext("/g/home", SCOPE)).toEqual({ pathname: "/g/home", search: "" })
    expect(parseListContext("/g/home/files?q=a#x", SCOPE)).toEqual({
      pathname: "/g/home/files",
      search: "q=a",
    })
  })

  it("reads no context when the parameter is missing", () => {
    expect(readListContext(new URLSearchParams("tab=files"), SCOPE)).toBeNull()
  })

  it.each([
    ["", 1],
    ["page=3", 3],
    ["page=0", 1],
    ["page=-2", 1],
    ["page=abc", 1],
    ["page=2x", 2],
  ])("reads the page from %j as %i", (search, page) => {
    expect(listContextPage({ pathname: "/g/home/files", search })).toBe(page)
  })

  it("moves the context to another page and drops the parameter on page 1", () => {
    const ctx = { pathname: "/g/home/files", search: "q=lamp&page=2" }

    expect(withListContextPage(ctx, 3)).toEqual({
      pathname: "/g/home/files",
      search: "q=lamp&page=3",
    })
    expect(withListContextPage(ctx, 1)).toEqual({ pathname: "/g/home/files", search: "q=lamp" })
  })

  it("returns the same context object when the page does not change", () => {
    const ctx = { pathname: "/g/home/files", search: "tags=a,b&page=2" }
    expect(withListContextPage(ctx, 2)).toBe(ctx)
  })
})
