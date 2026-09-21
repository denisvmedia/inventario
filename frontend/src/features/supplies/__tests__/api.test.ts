import { beforeEach, describe, expect, it } from "vitest"
import { http, HttpResponse } from "msw"

import {
  createSupplyLink,
  deleteSupplyLink,
  listSupplyLinks,
  reorderSupplyLinks,
  updateSupplyLink,
} from "@/features/supplies/api"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"

const commodityID = "c1"
const listPath = apiUrl(`/g/g1/commodities/${commodityID}/supplies`)

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("token")
  setCurrentGroupSlug("g1")
})

describe("features/supplies/api", () => {
  describe("listSupplyLinks", () => {
    it("reads the flat list shape and the meta total", async () => {
      server.use(
        http.get(listPath, () =>
          HttpResponse.json({
            data: [
              { id: "s1", label: "Spares", url: "https://example.com/1" },
              { id: "s2", label: "Filters", url: "https://example.com/2" },
            ],
            meta: { supply_links: 2, total: 7 },
          })
        )
      )

      const result = await listSupplyLinks(commodityID)
      expect(result.links.map((l) => l.id)).toEqual(["s1", "s2"])
      // The total comes from meta, not from the page length — the list is
      // paginated and the two differ as soon as there is a second page.
      expect(result.total).toBe(7)
    })

    it("falls back to the row count when meta carries no total", async () => {
      server.use(
        http.get(listPath, () =>
          HttpResponse.json({ data: [{ id: "s1", label: "Spares", url: "https://example.com/1" }] })
        )
      )

      const result = await listSupplyLinks(commodityID)
      expect(result.total).toBe(1)
    })

    it("reads an absent data array as an empty list, not as a failure", async () => {
      server.use(http.get(listPath, () => HttpResponse.json({ meta: { total: 0 } })))

      const result = await listSupplyLinks(commodityID)
      expect(result.links).toEqual([])
      expect(result.total).toBe(0)
    })
  })

  describe("createSupplyLink", () => {
    it("sends the JSON:API envelope and lifts data.id onto the entity", async () => {
      let sent: unknown
      server.use(
        http.post(listPath, async ({ request }) => {
          sent = await request.json()
          return HttpResponse.json(
            {
              data: {
                id: "s9",
                type: "commodity_supply_links",
                attributes: { label: "Spares", url: "https://example.com/1", notes: "" },
              },
            },
            { status: 201 }
          )
        })
      )

      const created = await createSupplyLink({
        commodity_id: commodityID,
        label: "Spares",
        url: "https://example.com/1",
      })

      expect(sent).toEqual({
        data: {
          type: "commodity_supply_links",
          attributes: { label: "Spares", url: "https://example.com/1", notes: "" },
        },
      })
      expect(created.id).toBe("s9")
      expect(created.label).toBe("Spares")
    })

    it("refuses a response whose id lives under attributes", async () => {
      // Accepting a nested id would mask the backend bug a typed envelope
      // exists to surface.
      server.use(
        http.post(listPath, () =>
          HttpResponse.json(
            {
              data: {
                type: "commodity_supply_links",
                attributes: { id: "s9", label: "Spares", url: "https://example.com/1" },
              },
            },
            { status: 201 }
          )
        )
      )

      await expect(
        createSupplyLink({
          commodity_id: commodityID,
          label: "Spares",
          url: "https://example.com/1",
        })
      ).rejects.toThrow(/missing id/i)
    })
  })

  describe("updateSupplyLink", () => {
    it("sends only the fields the caller set", async () => {
      let sent: unknown
      server.use(
        http.patch(`${listPath}/s1`, async ({ request }) => {
          sent = await request.json()
          return HttpResponse.json({
            data: { id: "s1", type: "commodity_supply_links", attributes: { label: "Renamed" } },
          })
        })
      )

      const updated = await updateSupplyLink({
        commodity_id: commodityID,
        supply_id: "s1",
        label: "Renamed",
      })

      expect(sent).toEqual({
        data: { type: "commodity_supply_links", attributes: { label: "Renamed" } },
      })
      expect(updated.id).toBe("s1")
    })

    it("sends an empty string when the caller clears a field", async () => {
      // undefined means "leave it"; "" means "clear it". Folding the two
      // together would make notes impossible to erase.
      let sent: { data: { attributes: Record<string, unknown> } } | undefined
      server.use(
        http.patch(`${listPath}/s1`, async ({ request }) => {
          sent = (await request.json()) as typeof sent
          return HttpResponse.json({
            data: { id: "s1", type: "commodity_supply_links", attributes: {} },
          })
        })
      )

      await updateSupplyLink({ commodity_id: commodityID, supply_id: "s1", notes: "" })
      expect(sent?.data.attributes).toEqual({ notes: "" })
    })
  })

  it("deleteSupplyLink targets the nested path", async () => {
    let hit = false
    server.use(
      http.delete(`${listPath}/s1`, () => {
        hit = true
        return new HttpResponse(null, { status: 204 })
      })
    )

    await deleteSupplyLink({ commodity_id: commodityID, supply_id: "s1" })
    expect(hit).toBe(true)
  })

  it("reorderSupplyLinks posts the id order and reads back the new list", async () => {
    let sent: unknown
    server.use(
      http.post(`${listPath}/reorder`, async ({ request }) => {
        sent = await request.json()
        return HttpResponse.json({
          data: [
            { id: "s2", label: "Filters", url: "https://example.com/2" },
            { id: "s1", label: "Spares", url: "https://example.com/1" },
          ],
          meta: { total: 2 },
        })
      })
    )

    const result = await reorderSupplyLinks({ commodity_id: commodityID, ids: ["s2", "s1"] })
    expect(sent).toEqual({
      data: { type: "commodity_supply_links_reorder", attributes: { ids: ["s2", "s1"] } },
    })
    expect(result.links.map((l) => l.id)).toEqual(["s2", "s1"])
  })

  it("percent-encodes an id that would otherwise change the path", async () => {
    let seen = ""
    server.use(
      http.get(apiUrl("/g/g1/commodities/:id/supplies"), ({ params }) => {
        seen = String(params.id)
        return HttpResponse.json({ data: [] })
      })
    )

    await listSupplyLinks("a/b")
    expect(seen).toBe("a/b")
  })
})
