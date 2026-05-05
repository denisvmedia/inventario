import { describe, expect, it } from "vitest"

import { daysOverdue, isOpen } from "@/features/loans/api"

describe("loan helpers", () => {
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

    it("returns 0 when there is no due date", () => {
      expect(daysOverdue({ due_back_at: undefined, returned_at: undefined }, now)).toBe(0)
    })

    it("returns 0 when the loan is already returned", () => {
      expect(
        daysOverdue(
          {
            due_back_at: "2026-04-15" as unknown as undefined,
            returned_at: "2026-05-01" as unknown as undefined,
          },
          now
        )
      ).toBe(0)
    })

    it("returns 0 when due date is in the future", () => {
      expect(
        daysOverdue(
          { due_back_at: "2026-06-01" as unknown as undefined, returned_at: undefined },
          now
        )
      ).toBe(0)
    })

    it("returns the day count when due date is in the past", () => {
      expect(
        daysOverdue(
          { due_back_at: "2026-04-15" as unknown as undefined, returned_at: undefined },
          now
        )
      ).toBe(25)
    })
  })
})
