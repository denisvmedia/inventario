import { afterEach, describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"

import {
  CURRENCY_MIGRATION_STATUSES,
  getMigration,
  isCurrencyMigrationActive,
  isCurrencyMigrationTerminal,
  listMigrations,
  previewMigration,
  startMigration,
} from "@/features/currency-migration/api"
import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"

const SLUG = "household"

afterEach(() => {
  server.resetHandlers()
})

describe("currency-migration api", () => {
  describe("status helpers", () => {
    it("isCurrencyMigrationTerminal flags completed/failed only", () => {
      expect(isCurrencyMigrationTerminal("completed")).toBe(true)
      expect(isCurrencyMigrationTerminal("failed")).toBe(true)
      expect(isCurrencyMigrationTerminal("pending")).toBe(false)
      expect(isCurrencyMigrationTerminal("running")).toBe(false)
      expect(isCurrencyMigrationTerminal(undefined)).toBe(false)
    })

    it("isCurrencyMigrationActive flags pending/running only", () => {
      expect(isCurrencyMigrationActive("pending")).toBe(true)
      expect(isCurrencyMigrationActive("running")).toBe(true)
      expect(isCurrencyMigrationActive("completed")).toBe(false)
      expect(isCurrencyMigrationActive("failed")).toBe(false)
      expect(isCurrencyMigrationActive(undefined)).toBe(false)
    })

    it("CURRENCY_MIGRATION_STATUSES enumerates the closed union", () => {
      expect([...CURRENCY_MIGRATION_STATUSES].sort()).toEqual([
        "completed",
        "failed",
        "pending",
        "running",
      ])
    })
  })

  describe("listMigrations", () => {
    it("unwraps the JSON:API envelope to a Migration[]", async () => {
      server.use(
        msw.get(apiUrl(`/g/${SLUG}/currency-migrations`), () =>
          HttpResponse.json({
            data: [
              {
                id: "m1",
                type: "currency-migrations",
                attributes: {
                  from_currency: "USD",
                  to_currency: "EUR",
                  status: "completed",
                },
              },
            ],
          })
        )
      )
      const out = await listMigrations(SLUG)
      expect(out.migrations).toHaveLength(1)
      expect(out.migrations[0]).toMatchObject({ id: "m1", from_currency: "USD" })
    })

    it("returns an empty array when data is missing", async () => {
      server.use(msw.get(apiUrl(`/g/${SLUG}/currency-migrations`), () => HttpResponse.json({})))
      const out = await listMigrations(SLUG)
      expect(out.migrations).toEqual([])
    })

    it("hits the /g/{slug}/ scoped path even without an active group context", async () => {
      // The api builds the full path; the http rewrite is bypassed via
      // `skipGroupRewrite: true`. This guards Copilot's #1604 finding —
      // a regression that re-introduces the rewrite-slot dependency
      // would route to /api/v1/currency-migrations and fail this test.
      let captured = ""
      server.use(
        msw.get(apiUrl(`/g/${SLUG}/currency-migrations`), ({ request }) => {
          captured = new URL(request.url).pathname
          return HttpResponse.json({ data: [] })
        })
      )
      await listMigrations(SLUG)
      expect(captured).toBe("/api/v1/g/household/currency-migrations")
    })
  })

  describe("getMigration", () => {
    it("unwraps a single migration", async () => {
      server.use(
        msw.get(apiUrl(`/g/${SLUG}/currency-migrations/m1`), () =>
          HttpResponse.json({
            data: {
              id: "m1",
              type: "currency-migrations",
              attributes: { status: "running" },
            },
          })
        )
      )
      const m = await getMigration(SLUG, "m1")
      expect(m).toMatchObject({ id: "m1", status: "running" })
    })

    it("falls back to the requested id when the envelope omits it", async () => {
      server.use(
        msw.get(apiUrl(`/g/${SLUG}/currency-migrations/m2`), () =>
          HttpResponse.json({
            data: {
              type: "currency-migrations",
              attributes: { status: "pending" },
            },
          })
        )
      )
      const m = await getMigration(SLUG, "m2")
      expect(m.id).toBe("m2")
    })

    it("throws when the envelope is empty", async () => {
      server.use(msw.get(apiUrl(`/g/${SLUG}/currency-migrations/m3`), () => HttpResponse.json({})))
      await expect(getMigration(SLUG, "m3")).rejects.toThrow(/missing data/)
    })

    it("throws when attributes are missing", async () => {
      server.use(
        msw.get(apiUrl(`/g/${SLUG}/currency-migrations/m4`), () =>
          HttpResponse.json({ data: { id: "m4", type: "currency-migrations" } })
        )
      )
      await expect(getMigration(SLUG, "m4")).rejects.toThrow(/missing attributes/)
    })
  })

  describe("previewMigration", () => {
    it("posts the request and returns the preview body", async () => {
      let capturedBody: unknown = null
      server.use(
        msw.post(apiUrl(`/g/${SLUG}/currency-migrations/preview`), async ({ request }) => {
          capturedBody = await request.json()
          return HttpResponse.json({
            data: {
              type: "currency-migration-previews",
              attributes: {
                preview_token: "tok",
                preview_expires_in_seconds: 600,
                commodity_count: 0,
              },
            },
          })
        })
      )
      const body = await previewMigration(SLUG, {
        from_currency: "USD",
        to_currency: "EUR",
        exchange_rate: 0.9,
      })
      expect(body.preview_token).toBe("tok")
      expect(capturedBody).toMatchObject({
        data: {
          type: "currency-migrations",
          attributes: { from_currency: "USD", to_currency: "EUR", exchange_rate: 0.9 },
        },
      })
    })

    it("throws when the response has no attributes", async () => {
      server.use(
        msw.post(apiUrl(`/g/${SLUG}/currency-migrations/preview`), () => HttpResponse.json({}))
      )
      await expect(
        previewMigration(SLUG, { from_currency: "USD", to_currency: "EUR", exchange_rate: 1 })
      ).rejects.toThrow(/missing attributes/)
    })
  })

  describe("startMigration", () => {
    it("posts the start request and unwraps the migration", async () => {
      server.use(
        msw.post(apiUrl(`/g/${SLUG}/currency-migrations`), () =>
          HttpResponse.json(
            {
              data: {
                id: "m9",
                type: "currency-migrations",
                attributes: { status: "pending", to_currency: "EUR" },
              },
            },
            { status: 201 }
          )
        )
      )
      const m = await startMigration(SLUG, {
        from_currency: "USD",
        to_currency: "EUR",
        exchange_rate: 0.9,
        preview_token: "tok",
      })
      expect(m).toMatchObject({ id: "m9", status: "pending" })
    })

    it("throws on an empty envelope", async () => {
      server.use(msw.post(apiUrl(`/g/${SLUG}/currency-migrations`), () => HttpResponse.json({})))
      await expect(
        startMigration(SLUG, {
          from_currency: "USD",
          to_currency: "EUR",
          exchange_rate: 1,
          preview_token: "t",
        })
      ).rejects.toThrow(/missing data/)
    })
  })
})
