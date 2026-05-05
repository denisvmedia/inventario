import { afterEach, beforeEach, describe, expect, it } from "vitest"

import {
  daysOverdue,
  deleteService,
  getServiceCounts,
  hasCost,
  isOpen,
  listGroupServices,
  listServicesForCommodity,
  returnService,
  startService,
  updateService,
} from "@/features/services/api"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { serviceKeys } from "@/features/services/keys"
import { serviceFormSchema } from "@/features/services/schemas"
import { serviceHandlers, apiUrl } from "@/test/handlers"
import { server } from "@/test/server"
import { http, HttpResponse } from "msw"

const SLUG = "household"
const COMMODITY_ID = "commodity-1"

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  setCurrentGroupSlug(SLUG)
})

afterEach(() => {
  server.resetHandlers()
})

describe("service display helpers", () => {
  describe("isOpen", () => {
    it("treats a row with no returned_at as open", () => {
      expect(isOpen({ returned_at: undefined })).toBe(true)
      expect(isOpen({ returned_at: null as unknown as undefined })).toBe(true)
      expect(isOpen({ returned_at: "" as unknown as undefined })).toBe(true)
    })
    it("treats a returned_at value as closed", () => {
      expect(isOpen({ returned_at: "2026-05-10" as unknown as undefined })).toBe(false)
    })
  })

  describe("daysOverdue", () => {
    const now = new Date("2026-05-10T12:00:00Z")

    it("returns 0 when there is no expected_return_at", () => {
      expect(daysOverdue({ expected_return_at: undefined, returned_at: undefined }, now)).toBe(0)
    })
    it("returns 0 when the service is already returned", () => {
      expect(
        daysOverdue(
          {
            expected_return_at: "2026-04-15" as unknown as undefined,
            returned_at: "2026-05-01" as unknown as undefined,
          },
          now
        )
      ).toBe(0)
    })
    it("returns 0 when expected return is in the future", () => {
      expect(
        daysOverdue(
          {
            expected_return_at: "2026-06-01" as unknown as undefined,
            returned_at: undefined,
          },
          now
        )
      ).toBe(0)
    })
    it("returns 0 when expected_return_at is malformed", () => {
      expect(
        daysOverdue(
          {
            expected_return_at: "not-a-date" as unknown as undefined,
            returned_at: undefined,
          },
          now
        )
      ).toBe(0)
    })
    it("returns the day count when expected return is in the past", () => {
      expect(
        daysOverdue(
          {
            expected_return_at: "2026-04-15" as unknown as undefined,
            returned_at: undefined,
          },
          now
        )
      ).toBe(25)
    })
  })

  describe("hasCost", () => {
    it("returns false for a row with no cost fields", () => {
      expect(hasCost({ cost_amount: undefined, cost_currency: undefined })).toBe(false)
    })
    it("returns false when only one half is set", () => {
      expect(
        hasCost({ cost_amount: "100" as unknown as undefined, cost_currency: undefined })
      ).toBe(false)
      expect(
        hasCost({ cost_amount: undefined, cost_currency: "EUR" as unknown as undefined })
      ).toBe(false)
    })
    it("returns false for a recorded zero cost", () => {
      expect(
        hasCost({
          cost_amount: "0" as unknown as undefined,
          cost_currency: "EUR" as unknown as undefined,
        })
      ).toBe(false)
    })
    it("returns true for a non-zero cost with a currency", () => {
      expect(
        hasCost({
          cost_amount: "245.00" as unknown as undefined,
          cost_currency: "EUR" as unknown as undefined,
        })
      ).toBe(true)
    })
    it("treats malformed amounts as no cost", () => {
      expect(
        hasCost({
          cost_amount: "abc" as unknown as undefined,
          cost_currency: "EUR" as unknown as undefined,
        })
      ).toBe(false)
    })
  })
})

describe("services API functions", () => {
  it("listServicesForCommodity returns the rows + total", async () => {
    server.use(
      ...serviceHandlers.listForCommodity(SLUG, COMMODITY_ID, [
        {
          id: "svc-1",
          commodity_id: COMMODITY_ID,
          provider_name: "Apple Service",
          sent_at: "2026-04-01",
          returned_at: null,
        },
      ])
    )
    const result = await listServicesForCommodity(COMMODITY_ID)
    expect(result.services).toHaveLength(1)
    expect(result.services[0]?.provider_name).toBe("Apple Service")
    expect(result.total).toBe(1)
  })

  it("listGroupServices passes state, page, perPage as query params", async () => {
    let capturedURL = ""
    server.use(
      http.get(apiUrl(`/g/${SLUG}/services`), ({ request }) => {
        capturedURL = request.url
        return HttpResponse.json({
          data: [],
          meta: { services: 0, total: 0 },
        })
      })
    )
    const result = await listGroupServices({ state: "overdue", page: 2, perPage: 25 })
    expect(result.services).toEqual([])
    expect(capturedURL).toContain("state=overdue")
    expect(capturedURL).toContain("page=2")
    expect(capturedURL).toContain("per_page=25")
  })

  it("listGroupServices flattens commodity ref into a separate field", async () => {
    server.use(
      ...serviceHandlers.listGroup(SLUG, [
        {
          id: "svc-1",
          commodity_id: "c1",
          provider_name: "Apple Service",
          sent_at: "2026-04-01",
          returned_at: null,
          commodity: { id: "c1", name: "MacBook" },
        },
      ])
    )
    const result = await listGroupServices()
    expect(result.services).toHaveLength(1)
    expect(result.services[0]?.commodity?.name).toBe("MacBook")
    // commodity ref must NOT be inside the service entity itself.
    expect(
      (result.services[0]?.service as unknown as { commodity?: unknown }).commodity
    ).toBeUndefined()
  })

  it("getServiceCounts short-circuits on an empty input", async () => {
    let hit = false
    server.use(
      http.get(apiUrl(`/g/${SLUG}/services/counts`), () => {
        hit = true
        return HttpResponse.json({ data: {} })
      })
    )
    const result = await getServiceCounts([])
    expect(result).toEqual({})
    expect(hit).toBe(false)
  })

  it("getServiceCounts returns the BE map verbatim", async () => {
    server.use(...serviceHandlers.counts(SLUG, { c1: 1, c2: 0 }))
    const result = await getServiceCounts(["c1", "c2", "c3"])
    expect(result).toEqual({ c1: 1, c2: 0 })
  })

  it("startService strips commodity_id from attributes and posts to the per-commodity URL", async () => {
    let capturedBody: unknown = null
    server.use(
      http.post(apiUrl(`/g/${SLUG}/commodities/${COMMODITY_ID}/services`), async ({ request }) => {
        capturedBody = await request.json()
        return HttpResponse.json(
          {
            id: "svc-new",
            type: "commodity_services",
            attributes: {
              id: "svc-new",
              commodity_id: COMMODITY_ID,
              provider_name: "Apple Service",
              sent_at: "2026-05-05",
              returned_at: null,
            },
          },
          { status: 201 }
        )
      })
    )
    const result = await startService({
      commodity_id: COMMODITY_ID,
      provider_name: "Apple Service",
      sent_at: "2026-05-05",
    })
    expect(result.id).toBe("svc-new")
    const body = capturedBody as { data?: { attributes?: Record<string, unknown> } }
    expect(body.data?.attributes?.provider_name).toBe("Apple Service")
    // commodity_id lives in the URL, not the attributes object.
    expect(body.data?.attributes?.commodity_id).toBeUndefined()
  })

  it("startService throws when the response has no attributes", async () => {
    server.use(
      http.post(apiUrl(`/g/${SLUG}/commodities/${COMMODITY_ID}/services`), () =>
        HttpResponse.json({ id: "svc-new", type: "commodity_services" }, { status: 201 })
      )
    )
    await expect(
      startService({
        commodity_id: COMMODITY_ID,
        provider_name: "Apple Service",
        sent_at: "2026-05-05",
      })
    ).rejects.toThrow(/Malformed POST/)
  })

  it("updateService PATCHes with id+type+attributes envelope", async () => {
    let capturedBody: unknown = null
    server.use(
      http.patch(
        apiUrl(`/g/${SLUG}/commodities/${COMMODITY_ID}/services/svc-1`),
        async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({
            id: "svc-1",
            type: "commodity_services",
            attributes: {
              id: "svc-1",
              commodity_id: COMMODITY_ID,
              provider_name: "Apple Service",
              reason: "diagnostic + screen",
              sent_at: "2026-04-01",
              returned_at: null,
            },
          })
        }
      )
    )
    const result = await updateService(COMMODITY_ID, "svc-1", { reason: "diagnostic + screen" })
    expect(result.id).toBe("svc-1")
    const body = capturedBody as {
      data?: { id?: string; type?: string; attributes?: Record<string, unknown> }
    }
    expect(body.data?.id).toBe("svc-1")
    expect(body.data?.type).toBe("commodity_services")
    expect(body.data?.attributes?.reason).toBe("diagnostic + screen")
  })

  it("updateService throws on a malformed response", async () => {
    server.use(
      http.patch(apiUrl(`/g/${SLUG}/commodities/${COMMODITY_ID}/services/svc-1`), () =>
        HttpResponse.json({ id: "svc-1", type: "commodity_services" })
      )
    )
    await expect(updateService(COMMODITY_ID, "svc-1", { reason: "diagnostic" })).rejects.toThrow(
      /Malformed PATCH/
    )
  })

  it("returnService sends no body when no options are passed", async () => {
    let capturedBody = "non-empty-sentinel"
    server.use(
      http.post(
        apiUrl(`/g/${SLUG}/commodities/${COMMODITY_ID}/services/svc-1/return`),
        async ({ request }) => {
          capturedBody = await request.text()
          return HttpResponse.json({
            id: "svc-1",
            type: "commodity_services",
            attributes: {
              id: "svc-1",
              commodity_id: COMMODITY_ID,
              provider_name: "Apple Service",
              sent_at: "2026-04-01",
              returned_at: "2026-05-05",
            },
          })
        }
      )
    )
    const result = await returnService(COMMODITY_ID, "svc-1")
    expect(result.returned_at).toBe("2026-05-05")
    // Empty body means the BE fills returned_at with today.
    expect(capturedBody).toBe("")
  })

  it("returnService bundles returnedAt + cost into the body when supplied", async () => {
    let capturedBody: unknown = null
    server.use(
      http.post(
        apiUrl(`/g/${SLUG}/commodities/${COMMODITY_ID}/services/svc-1/return`),
        async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({
            id: "svc-1",
            type: "commodity_services",
            attributes: {
              id: "svc-1",
              commodity_id: COMMODITY_ID,
              provider_name: "Apple Service",
              sent_at: "2026-04-01",
              returned_at: "2026-05-05",
              cost_amount: "245.00",
              cost_currency: "EUR",
            },
          })
        }
      )
    )
    const result = await returnService(COMMODITY_ID, "svc-1", {
      returnedAt: "2026-05-05",
      costAmount: "245.00",
      costCurrency: "EUR",
    })
    expect(result.cost_amount).toBe("245.00")
    const body = capturedBody as { data?: { attributes?: Record<string, unknown> } }
    expect(body.data?.attributes?.returned_at).toBe("2026-05-05")
    expect(body.data?.attributes?.cost_amount).toBe("245.00")
    expect(body.data?.attributes?.cost_currency).toBe("EUR")
  })

  it("returnService throws on a malformed response", async () => {
    server.use(
      http.post(apiUrl(`/g/${SLUG}/commodities/${COMMODITY_ID}/services/svc-1/return`), () =>
        HttpResponse.json({ id: "svc-1", type: "commodity_services" })
      )
    )
    await expect(returnService(COMMODITY_ID, "svc-1")).rejects.toThrow(/Malformed POST/)
  })

  it("deleteService issues a DELETE to the service URL", async () => {
    let hit = false
    server.use(
      http.delete(apiUrl(`/g/${SLUG}/commodities/${COMMODITY_ID}/services/svc-1`), () => {
        hit = true
        return new HttpResponse(null, { status: 204 })
      })
    )
    await deleteService(COMMODITY_ID, "svc-1")
    expect(hit).toBe(true)
  })
})

describe("serviceKeys", () => {
  it("scopes the cache by slug + suffix", () => {
    expect(serviceKeys.all).toEqual(["service"])
    expect(serviceKeys.group("household")).toEqual(["service", "household"])
    expect(serviceKeys.byCommodity("household", "c1")).toEqual([
      "service",
      "household",
      "byCommodity",
      "c1",
    ])
  })

  it("encodes group-list options into a stable URLSearchParams suffix", () => {
    const a = serviceKeys.groupList("household", { state: "open", page: 1, perPage: 25 })
    const b = serviceKeys.groupList("household", { perPage: 25, page: 1, state: "open" })
    // Same options in a different order → same key (URLSearchParams is
    // ordered, so this is brittle; the important property is structural
    // equality when the user passes identical inputs).
    expect(a).toEqual(b)
  })

  it("treats counts with the same id-set in any order as one cache entry", () => {
    expect(serviceKeys.counts("household", ["a", "b", "c"])).toEqual(
      serviceKeys.counts("household", ["c", "b", "a"])
    )
  })

  it("returns an empty suffix when groupList is called without options", () => {
    expect(serviceKeys.groupList("household")).toEqual(["service", "household", "groupList", ""])
  })
})

describe("serviceFormSchema", () => {
  it("accepts a minimum-fields valid form", () => {
    const result = serviceFormSchema.safeParse({
      provider_name: "Apple Service",
      sent_at: "2026-05-05",
    })
    expect(result.success).toBe(true)
  })

  it("rejects an empty provider_name", () => {
    const result = serviceFormSchema.safeParse({
      provider_name: "  ",
      sent_at: "2026-05-05",
    })
    expect(result.success).toBe(false)
  })

  it("rejects a malformed sent_at", () => {
    const result = serviceFormSchema.safeParse({
      provider_name: "Apple Service",
      sent_at: "not-a-date",
    })
    expect(result.success).toBe(false)
  })

  it("rejects cost_amount without cost_currency", () => {
    const result = serviceFormSchema.safeParse({
      provider_name: "Apple Service",
      sent_at: "2026-05-05",
      cost_amount: "245.00",
    })
    expect(result.success).toBe(false)
  })

  it("rejects cost_currency without cost_amount", () => {
    const result = serviceFormSchema.safeParse({
      provider_name: "Apple Service",
      sent_at: "2026-05-05",
      cost_currency: "EUR",
    })
    expect(result.success).toBe(false)
  })

  it("accepts both cost fields when set together", () => {
    const result = serviceFormSchema.safeParse({
      provider_name: "Apple Service",
      sent_at: "2026-05-05",
      cost_amount: "245.00",
      cost_currency: "EUR",
    })
    expect(result.success).toBe(true)
  })

  it("rejects bogus currency formats", () => {
    const result = serviceFormSchema.safeParse({
      provider_name: "Apple Service",
      sent_at: "2026-05-05",
      cost_amount: "245.00",
      cost_currency: "eur", // lowercase fails the [A-Z]{3} rule
    })
    expect(result.success).toBe(false)
  })

  it("rejects malformed cost_amount values", () => {
    const result = serviceFormSchema.safeParse({
      provider_name: "Apple Service",
      sent_at: "2026-05-05",
      cost_amount: "abc",
      cost_currency: "EUR",
    })
    expect(result.success).toBe(false)
  })

  it("rejects an over-long reason", () => {
    const result = serviceFormSchema.safeParse({
      provider_name: "Apple Service",
      sent_at: "2026-05-05",
      reason: "x".repeat(1001),
    })
    expect(result.success).toBe(false)
  })
})
