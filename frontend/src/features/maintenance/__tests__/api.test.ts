import { beforeEach, describe, expect, it } from "vitest"
import { http, HttpResponse } from "msw"

import {
  createMaintenanceSchedule,
  daysUntilDue,
  deleteMaintenanceSchedule,
  listGroupMaintenance,
  listSchedulesForCommodity,
  markMaintenanceDone,
  updateMaintenanceSchedule,
} from "@/features/maintenance/api"
import { setAccessToken, clearAuth } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { server } from "@/test/server"
import { apiUrl } from "@/test/handlers"

const commodityID = "c1"
const perCommodityPath = apiUrl(`/g/g1/commodities/${commodityID}/maintenance`)
const groupPath = apiUrl("/g/g1/maintenance")

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("token")
  setCurrentGroupSlug("g1")
})

describe("features/maintenance/api", () => {
  it("listSchedulesForCommodity reads the flat shape and prefers meta.total", async () => {
    server.use(
      http.get(perCommodityPath, () =>
        HttpResponse.json({
          data: [{ id: "m1", title: "Oil change", interval_days: 90 }],
          meta: { schedules: 1, total: 4 },
        })
      )
    )

    const result = await listSchedulesForCommodity(commodityID)
    expect(result.schedules.map((s) => s.id)).toEqual(["m1"])
    expect(result.total).toBe(4)
  })

  describe("listGroupMaintenance", () => {
    it("splits the commodity reference out of each row", async () => {
      // The group listing inlines the owning commodity on the row. Leaving
      // it on the schedule would make the two shapes silently different
      // from the per-commodity listing's.
      server.use(
        http.get(groupPath, () =>
          HttpResponse.json({
            data: [
              {
                id: "m1",
                title: "Oil change",
                interval_days: 90,
                commodity: { id: "c1", name: "Mower" },
              },
            ],
            meta: { total: 1 },
          })
        )
      )

      const result = await listGroupMaintenance()
      expect(result.schedules).toHaveLength(1)
      expect(result.schedules[0].commodity).toEqual({ id: "c1", name: "Mower" })
      expect(result.schedules[0].schedule).not.toHaveProperty("commodity")
      expect(result.schedules[0].schedule.id).toBe("m1")
    })

    it("leaves commodity undefined when the row carries none", async () => {
      server.use(
        http.get(groupPath, () =>
          HttpResponse.json({ data: [{ id: "m1", title: "Oil change" }], meta: { total: 1 } })
        )
      )

      const result = await listGroupMaintenance()
      expect(result.schedules[0].commodity).toBeUndefined()
    })

    it("sends only the filters the caller set", async () => {
      let search = ""
      server.use(
        http.get(groupPath, ({ request }) => {
          search = new URL(request.url).search
          return HttpResponse.json({ data: [], meta: { total: 0 } })
        })
      )

      await listGroupMaintenance({
        page: 2,
        perPage: 25,
        dueBefore: "2027-01-31",
        enabledOnly: true,
      })
      const params = new URLSearchParams(search)
      expect(params.get("page")).toBe("2")
      expect(params.get("per_page")).toBe("25")
      expect(params.get("due_before")).toBe("2027-01-31")
      expect(params.get("enabled_only")).toBe("true")

      // enabled_only=false is the default, so sending it would be noise.
      await listGroupMaintenance({ enabledOnly: false })
      expect(search).toBe("")
    })
  })

  describe("createMaintenanceSchedule", () => {
    it("strips commodity_id out of the attributes and lifts data.id", async () => {
      let sent: unknown
      server.use(
        http.post(perCommodityPath, async ({ request }) => {
          sent = await request.json()
          return HttpResponse.json(
            {
              data: {
                id: "m9",
                type: "maintenance_schedules",
                attributes: { title: "Oil change", interval_days: 90 },
              },
            },
            { status: 201 }
          )
        })
      )

      const created = await createMaintenanceSchedule({
        commodity_id: commodityID,
        title: "Oil change",
        interval_days: 90,
      })

      expect(sent).toEqual({
        data: {
          type: "maintenance_schedules",
          attributes: { title: "Oil change", interval_days: 90 },
        },
      })
      expect(created.id).toBe("m9")
    })

    it("refuses a response with no attributes", async () => {
      server.use(
        http.post(perCommodityPath, () =>
          HttpResponse.json({ data: { id: "m9", type: "maintenance_schedules" } }, { status: 201 })
        )
      )

      await expect(
        createMaintenanceSchedule({ commodity_id: commodityID, title: "x", interval_days: 1 })
      ).rejects.toThrow(/missing data.attributes/)
    })

    it("refuses a response with no id", async () => {
      server.use(
        http.post(perCommodityPath, () =>
          HttpResponse.json(
            { data: { type: "maintenance_schedules", attributes: { title: "x" } } },
            { status: 201 }
          )
        )
      )

      await expect(
        createMaintenanceSchedule({ commodity_id: commodityID, title: "x", interval_days: 1 })
      ).rejects.toThrow(/missing data.id/)
    })
  })

  it("updateMaintenanceSchedule patches by schedule id, not through the commodity", async () => {
    let sent: unknown
    server.use(
      http.patch(apiUrl("/g/g1/maintenance/m1"), async ({ request }) => {
        sent = await request.json()
        return HttpResponse.json({
          data: { id: "m1", type: "maintenance_schedules", attributes: { title: "Renamed" } },
        })
      })
    )

    const updated = await updateMaintenanceSchedule("m1", { title: "Renamed" })
    expect(sent).toEqual({
      data: { id: "m1", type: "maintenance_schedules", attributes: { title: "Renamed" } },
    })
    expect(updated.title).toBe("Renamed")
  })

  describe("markMaintenanceDone", () => {
    it("sends no body for the plain case so the server dates it", async () => {
      let body = ""
      server.use(
        http.post(apiUrl("/g/g1/maintenance/m1/done"), async ({ request }) => {
          body = await request.text()
          return HttpResponse.json({
            data: { id: "m1", type: "maintenance_schedules", attributes: { title: "Oil change" } },
          })
        })
      )

      await markMaintenanceDone("m1")
      expect(body).toBe("")
    })

    it("sends done_at when the caller backdates it", async () => {
      let sent: unknown
      server.use(
        http.post(apiUrl("/g/g1/maintenance/m1/done"), async ({ request }) => {
          sent = await request.json()
          return HttpResponse.json({
            data: { id: "m1", type: "maintenance_schedules", attributes: { title: "Oil change" } },
          })
        })
      )

      await markMaintenanceDone("m1", "2026-09-01")
      expect(sent).toEqual({
        data: { type: "maintenance_schedules", attributes: { done_at: "2026-09-01" } },
      })
    })
  })

  it("deleteMaintenanceSchedule targets the schedule path", async () => {
    let hit = false
    server.use(
      http.delete(apiUrl("/g/g1/maintenance/m1"), () => {
        hit = true
        return new HttpResponse(null, { status: 204 })
      })
    )

    await deleteMaintenanceSchedule("m1")
    expect(hit).toBe(true)
  })

  // The overdue badge has to agree with the reminder worker, which compares
  // next_due_at against midnight UTC of the current day
  // (models.MaintenanceSchedule.IsOverdue). A local-time comparison here
  // would show a badge the emails contradict for part of every day.
  describe("daysUntilDue", () => {
    it("returns 0 on the due date itself", () => {
      expect(daysUntilDue({ next_due_at: "2026-09-21" }, new Date("2026-09-21T00:00:00Z"))).toBe(0)
      expect(daysUntilDue({ next_due_at: "2026-09-21" }, new Date("2026-09-21T23:59:59Z"))).toBe(0)
    })

    it("counts forward and backward in whole days", () => {
      const now = new Date("2026-09-21T12:00:00Z")
      expect(daysUntilDue({ next_due_at: "2026-09-28" }, now)).toBe(7)
      expect(daysUntilDue({ next_due_at: "2026-09-14" }, now)).toBe(-7)
    })

    it("uses the UTC day, not the runner's local one", () => {
      // 23:30 in UTC+14 is already the next day locally. Both must read
      // the same UTC day as the worker does.
      const lateUTC = new Date("2026-09-21T23:30:00Z")
      const earlyUTC = new Date("2026-09-21T00:30:00Z")
      expect(daysUntilDue({ next_due_at: "2026-09-22" }, lateUTC)).toBe(1)
      expect(daysUntilDue({ next_due_at: "2026-09-22" }, earlyUTC)).toBe(1)
    })

    it("crosses a DST boundary without gaining or losing a day", () => {
      // Europe/Berlin ends DST on 2026-10-25. A local-midnight
      // implementation lands on 30.96 days here and rounds wrong.
      const now = new Date("2026-10-20T12:00:00Z")
      expect(daysUntilDue({ next_due_at: "2026-11-20" }, now)).toBe(31)
    })

    it("returns null when there is nothing to compare", () => {
      expect(daysUntilDue({ next_due_at: undefined })).toBeNull()
      expect(daysUntilDue({ next_due_at: "" as unknown as undefined })).toBeNull()
      expect(daysUntilDue({ next_due_at: "not-a-date" as unknown as undefined })).toBeNull()
    })
  })
})
