import { describe, expect, it } from "vitest"

import {
  inboxReviewContext,
  inboxReviewOptions,
  isInboxReviewContext,
  reviewPageForAllOffset,
} from "@/features/inbox/listContext"

describe("inbox review list context", () => {
  it("names the Items to review list and its page", () => {
    expect(inboxReviewContext("home")).toEqual({
      pathname: "/g/home/inbox",
      search: "reason=item_draft",
    })
    expect(inboxReviewContext("home", 3)).toEqual({
      pathname: "/g/home/inbox",
      search: "reason=item_draft&page=3",
    })
    expect(inboxReviewOptions(2)).toEqual({ reason: "item_draft", page: 2, perPage: 50 })
  })

  it("recognizes only the group's Items to review list", () => {
    expect(isInboxReviewContext(inboxReviewContext("home", 2), "home")).toBe(true)
    expect(isInboxReviewContext(inboxReviewContext("home"), "work")).toBe(false)
    expect(isInboxReviewContext({ pathname: "/g/home/inbox", search: "" }, "home")).toBe(false)
    expect(
      isInboxReviewContext({ pathname: "/g/home/inbox", search: "reason=draft" }, "home")
    ).toBe(false)
    expect(
      isInboxReviewContext({ pathname: "/g/home/commodities", search: "reason=item_draft" }, "home")
    ).toBe(false)
  })

  it("finds the review page of an entry of the All list, past the drafts", () => {
    // Two drafts come first: the entry at offset 2 is the first item to review.
    expect(reviewPageForAllOffset(2, 2)).toBe(1)
    expect(reviewPageForAllOffset(51, 2)).toBe(1)
    expect(reviewPageForAllOffset(52, 2)).toBe(2)
    expect(reviewPageForAllOffset(0, 0)).toBe(1)
  })
})
