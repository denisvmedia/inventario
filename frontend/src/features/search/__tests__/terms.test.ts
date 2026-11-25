import { describe, expect, it } from "vitest"

import { foldCase, matchesEveryTerm, splitSearchTerms } from "@/features/search/terms"

describe("splitSearchTerms", () => {
  it.each([
    ["", []],
    ["   ", []],
    ["garage", ["garage"]],
    ["  lake   road ", ["lake", "road"]],
    ['"lake road" 7', ["lake road", "7"]],
    ['shed "boat', ["shed", "boat"]],
    ["Road road ROAD", ["Road"]],
    ["tab\tseparated", ["tab", "separated"]],
  ])("splits %j into %j", (query, terms) => {
    expect(splitSearchTerms(query)).toEqual(terms)
  })
})

describe("foldCase", () => {
  it("folds the final sigma like any other sigma", () => {
    expect(foldCase("ΟΔΌΣ")).toBe(foldCase("Οδός"))
    expect(foldCase("Οδός")).toBe("οδόσ")
  })
})

describe("matchesEveryTerm", () => {
  it("needs every term, each in any field", () => {
    const fields = ["Weekend Cottage", "Lakeside Road 7"]
    expect(matchesEveryTerm(["cottage", "road"], fields)).toBe(true)
    expect(matchesEveryTerm(["cottage", "boathouse"], fields)).toBe(false)
  })

  it("ignores case, also for Greek", () => {
    expect(matchesEveryTerm(["ΟΔΌΣ"], ["Οδός Αθηνάς"])).toBe(true)
    expect(matchesEveryTerm(["οδόσ"], ["ΟΔΌΣ"])).toBe(true)
  })

  it("matches everything without terms and skips empty fields", () => {
    expect(matchesEveryTerm([], [])).toBe(true)
    expect(matchesEveryTerm(["x"], [null, undefined, ""])).toBe(false)
  })
})
