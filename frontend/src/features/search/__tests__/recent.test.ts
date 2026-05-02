import { afterEach, describe, expect, it } from "vitest"

import { clearRecent, getRecent, pushRecent } from "@/features/search/recent"

afterEach(() => {
  // Stay tidy: each test sets up its own scope; clearing all known
  // ones keeps a stale entry from leaking into the next case.
  clearRecent("scope-a")
  clearRecent("scope-b")
  clearRecent("default")
})

describe("recent items helper", () => {
  it("returns an empty list when storage has nothing", () => {
    expect(getRecent("scope-a")).toEqual([])
  })

  it("pushes entries newest-first and sets visitedAt", () => {
    pushRecent("scope-a", {
      type: "commodity",
      id: "c1",
      title: "Drill",
      url: "/g/x/commodities/c1",
    })
    pushRecent("scope-a", { type: "location", id: "l1", title: "Garage", url: "/g/x/locations/l1" })
    const out = getRecent("scope-a")
    expect(out).toHaveLength(2)
    expect(out[0]).toMatchObject({ type: "location", id: "l1" })
    expect(out[1]).toMatchObject({ type: "commodity", id: "c1" })
    expect(typeof out[0].visitedAt).toBe("number")
  })

  it("dedupes by (type, id) on push, keeping the newest visit at the front", () => {
    pushRecent("scope-a", { type: "commodity", id: "c1", title: "Drill v1", url: "/x" })
    pushRecent("scope-a", { type: "location", id: "l1", title: "Garage", url: "/y" })
    pushRecent("scope-a", { type: "commodity", id: "c1", title: "Drill v2", url: "/z" })
    const out = getRecent("scope-a")
    expect(out).toHaveLength(2)
    expect(out[0]).toMatchObject({ type: "commodity", id: "c1", title: "Drill v2", url: "/z" })
  })

  it("caps the list at 10 entries", () => {
    for (let i = 0; i < 15; i++) {
      pushRecent("scope-a", { type: "commodity", id: `c${i}`, title: `#${i}`, url: `/x/${i}` })
    }
    const out = getRecent("scope-a")
    expect(out).toHaveLength(10)
    // Oldest (c0..c4) get evicted; newest (c14) stays at the head.
    expect(out[0].id).toBe("c14")
    expect(out[out.length - 1].id).toBe("c5")
  })

  it("scopes recents per group slug", () => {
    pushRecent("scope-a", { type: "commodity", id: "c1", title: "A", url: "/a" })
    pushRecent("scope-b", { type: "location", id: "l1", title: "B", url: "/b" })
    expect(getRecent("scope-a")).toHaveLength(1)
    expect(getRecent("scope-b")).toHaveLength(1)
    expect(getRecent("scope-a")[0].id).toBe("c1")
    expect(getRecent("scope-b")[0].id).toBe("l1")
  })

  it("clearRecent wipes the scope", () => {
    pushRecent("scope-a", { type: "commodity", id: "c1", title: "A", url: "/a" })
    expect(getRecent("scope-a")).toHaveLength(1)
    clearRecent("scope-a")
    expect(getRecent("scope-a")).toEqual([])
  })

  it("ignores corrupt JSON in storage", () => {
    localStorage.setItem("inventario_recent_v1:bad", "{not-json")
    expect(getRecent("bad")).toEqual([])
  })

  it("ignores entries missing required fields", () => {
    localStorage.setItem(
      "inventario_recent_v1:bad",
      JSON.stringify([{ type: "commodity", id: "c1" }, { not: "valid" }])
    )
    expect(getRecent("bad")).toEqual([])
  })
})
