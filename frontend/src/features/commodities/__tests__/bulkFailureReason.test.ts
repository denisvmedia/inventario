import { describe, expect, it } from "vitest"

import type { CommodityBulkFailure } from "@/features/commodities/api"
import { bulkFailureReason, itemFieldProblem } from "@/features/commodities/bulkFailureReason"
import { i18next } from "@/i18n"

const t = i18next.getFixedT("en")

function failure(attrs: Partial<CommodityBulkFailure>): CommodityBulkFailure {
  return { id: "c1", name: "Lamp", code: "invalid", error: "", ...attrs }
}

describe("bulkFailureReason", () => {
  it.each([
    [{ field: "short_name", reason: "validation_is_empty" }, "Short name is required."],
    [{ field: "name", reason: "validation_length_out_of_range" }, "Name is too long."],
    [{ field: "brand", reason: "validation_length_too_long" }, "Brand is too long."],
    [{ field: "type", reason: "invalid_type" }, "Type has a value that is not allowed."],
    [
      { field: "purchase_date", reason: "validation_date_invalid" },
      "Purchase date is not in a valid format.",
    ],
    [
      { field: "current_price", reason: "validation_min_greater_equal_than_required" },
      "A price can't be negative.",
    ],
    [
      { field: "count", reason: "validation_min_greater_equal_than_required" },
      "Quantity must be at least 1.",
    ],
    [{ field: "status_date", reason: "status_date_required_on_transition" }, "Date is required."],
    [
      { field: "warranty_expires_at", reason: "quantity_forbids_warranty" },
      "Items with quantity greater than 1 don't support warranty tracking. Split into separate items for per-unit tracking.",
    ],
  ])("says %o in the user's language", (attrs, want) => {
    expect(bulkFailureReason(t, failure({ error: "raw English", ...attrs }))).toBe(want)
  })

  it("keeps the server's message for a rule it does not know", () => {
    expect(
      bulkFailureReason(
        t,
        failure({ field: "name", reason: "validation_new_rule", error: "name: new rule." })
      )
    ).toBe("name: new rule.")
    expect(
      bulkFailureReason(
        t,
        failure({
          field: "extra.serial",
          reason: "validation_required",
          error: "extra.serial: cannot be blank.",
        })
      )
    ).toBe("extra.serial: cannot be blank.")
  })

  it("says why an item was not found, not sent or failed otherwise", () => {
    expect(bulkFailureReason(t, failure({ code: "not_found" }))).toBe(
      "Not found. It may have been deleted."
    )
    expect(bulkFailureReason(t, failure({ code: "not_sent" }))).toBe(
      "Not changed: the action stopped before it reached this item. Try again."
    )
    expect(bulkFailureReason(t, failure({ code: "unknown" }))).toBe(
      "May have changed: no answer came back for this item. Check it in the list before you try again."
    )
    expect(bulkFailureReason(t, failure({ code: "failed", error: "pq: deadlock" }))).toBe(
      "Could not be changed. Try again later."
    )
    expect(bulkFailureReason(t, failure({ code: "invalid" }))).toBe(
      "Could not be changed. Try again later."
    )
  })
})

describe("itemFieldProblem", () => {
  it("uses the rule's own sentence when it has one", () => {
    expect(
      itemFieldProblem(t, "original_price", "validation_no_price_in_group_currency", "if ...")
    ).toBe(
      "Enter at least one of converted price or current value so the item is reportable in group currency."
    )
    expect(itemFieldProblem(t, "original_price_currency", "validation_required", "")).toBe(
      "Currency is required."
    )
  })

  it("names the field and translates the code otherwise", () => {
    expect(itemFieldProblem(t, "count", "validation_nil", "must be blank")).toBe(
      "Quantity: must be blank"
    )
  })

  it("keeps the server's message for a code or field it does not know", () => {
    expect(itemFieldProblem(t, "count", "brand_new_rule", "is odd")).toBe("Quantity: is odd")
    expect(itemFieldProblem(t, "colour", "", "is odd")).toBe("is odd")
  })
})
