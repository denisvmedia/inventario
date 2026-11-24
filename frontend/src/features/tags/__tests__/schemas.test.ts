import { describe, expect, it } from "vitest"

import { isValidSlug, normaliseSlug, tagFormSchema } from "../schemas"

// Same inputs and outputs as tagSlugCases in go/models/tag_test.go: the FE
// preview has to produce the slug the BE will store. Keep the two identical.
const slugCases: Array<[name: string, input: string, slug: string]> = [
  ["plain", "kitchen", "kitchen"],
  ["capitalized", "Kitchen", "kitchen"],
  ["surrounding spaces", "  kitchen  ", "kitchen"],
  ["space", "front office", "front-office"],
  ["underscore", "FRONT_OFFICE", "front-office"],
  ["dash run", "front---office", "front-office"],
  ["punctuation run", "front!@#office", "front-office"],
  ["leading and trailing dashes", "-leading-and-trailing-", "leading-and-trailing"],
  ["empty", "", ""],
  ["spaces only", "   ", ""],
  ["punctuation only", "###", ""],
  ["latin with umlaut", "unicode-ücase", "unicode-ücase"],
  ["cyrillic", "кухня", "кухня"],
  ["cyrillic capitalized", "Кухня", "кухня"],
  ["cyrillic two words", "на продажу", "на-продажу"],
  ["cjk", "書類", "書類"],
  ["czech", "Kuchyně", "kuchyně"],
  ["czech leading caron", "Čaj", "čaj"],
  ["french leading accent", "électronique", "électronique"],
  ["mixed scripts", "USB кабель", "usb-кабель"],
  ["mixed scripts, second word differs", "USB хаб", "usb-хаб"],
  ["decomposed input", "Kuchyně", "kuchyně"],
  ["greek final sigma", "ΟΔΟΣ", "οδος"],
  ["devanagari vowel signs", "हिन्दी", "हिन्दी"],
  ["cjk with digits", "東京 2024", "東京-2024"],
  ["superscript digit", "m²", "m²"],
  ["emoji dropped", "🍕 pizza", "pizza"],
  ["stray leading mark", "́abc", "abc"],
  ["dotted capital I", "İstanbul", "i̇stanbul"],
]

describe("normaliseSlug", () => {
  it.each(slugCases)("%s", (_name, input, slug) => {
    expect(normaliseSlug(input)).toBe(slug)
  })

  it.each(slugCases.filter(([, , slug]) => slug !== ""))(
    "%s: the slug is canonical",
    (_name, _input, slug) => {
      expect(normaliseSlug(slug)).toBe(slug)
      expect(isValidSlug(slug)).toBe(true)
    }
  )

  it("gives decomposed and precomposed spellings the same slug", () => {
    expect(normaliseSlug("Kuchyně čaj")).toBe(normaliseSlug("Kuchyně čaj"))
  })
})

describe("isValidSlug", () => {
  it.each(["kitchen", "front-office", "a-1-2", "кухня", "на-продажу", "書類", "kuchyně", "हिन्दी"])(
    "accepts %s",
    (slug) => {
      expect(isValidSlug(slug)).toBe(true)
    }
  )

  it.each([
    ["empty", ""],
    ["uppercase", "Kitchen"],
    ["cyrillic uppercase", "Кухня"],
    ["decomposed", "kuchyně"],
    ["double dash", "front--office"],
    ["leading dash", "-front"],
    ["space", "front office"],
    ["leading mark", "́abc"],
    ["emoji", "pizza-🍕"],
  ])("rejects %s", (_name, slug) => {
    expect(isValidSlug(slug)).toBe(false)
  })
})

describe("tagFormSchema", () => {
  it("accepts a Cyrillic label with its derived slug", () => {
    const parsed = tagFormSchema.safeParse({
      label: "Кухня",
      slug: normaliseSlug("Кухня"),
      color: "amber",
    })
    expect(parsed.success).toBe(true)
  })

  it("counts characters, not UTF-16 units, against the 64 limit", () => {
    // 64 emoji are 128 UTF-16 units but 64 characters, which the BE accepts.
    const label = "🍕".repeat(64)
    expect(tagFormSchema.safeParse({ label, slug: "pizza", color: "amber" }).success).toBe(true)

    const tooLong = tagFormSchema.safeParse({
      label: "ж".repeat(65),
      slug: "pizza",
      color: "amber",
    })
    expect(tooLong.success).toBe(false)
    expect(tooLong.error?.issues[0]?.message).toBe("tags:validation.labelTooLong")
  })

  it("rejects a slug that is not its own normal form", () => {
    const parsed = tagFormSchema.safeParse({ label: "Кухня", slug: "Кухня", color: "amber" })
    expect(parsed.success).toBe(false)
    expect(parsed.error?.issues[0]?.message).toBe("tags:validation.slugInvalid")
  })
})
