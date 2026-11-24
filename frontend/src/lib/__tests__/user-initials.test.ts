import { describe, expect, it } from "vitest"

import { userInitials } from "@/lib/user-initials"

describe("userInitials", () => {
  it.each([
    ["two words", "Alex Johnson", "alex@example.com", "AJ"],
    ["only the first two words", "Mary Ann Smith", undefined, "MA"],
    ["one word", "alex", undefined, "A"],
    ["extra whitespace", "  jordan   lee ", undefined, "JL"],
    ["a non-Latin name", "Иван Петров", undefined, "ИП"],
    ["a letter outside the BMP", "𝓐lex", undefined, "𝓐"],
    ["the email when there is no name", "", "sam@example.com", "S"],
    ["the email when the name is blank", "   ", "sam@example.com", "S"],
    ["a question mark when there is neither", null, null, "?"],
  ])("uses %s", (_, name, email, want) => {
    expect(userInitials(name, email)).toBe(want)
  })
})
