import { describe, expect, it } from "vitest"
import type { z } from "zod"

import { profileEditSchema, registerSchema } from "@/features/auth/schemas"
import { fileMetadataSchema } from "@/features/files/schemas"
import { createGroupSchema } from "@/features/group/schemas"
import { lendFormSchema } from "@/features/loans/schemas"
import { maintenanceFormSchema } from "@/features/maintenance/schemas"
import { supplyLinkFormSchema } from "@/features/supplies/schemas"
import { capCodePoints } from "@/lib/text-length"

// "😀" is one code point and two UTF-16 units. The server counts code points
// (Go runes), so a limit of n takes n emoji.
const EMOJI = "😀"

describe("capCodePoints", () => {
  it("keeps a value that fits", () => {
    expect(capCodePoints(`a${EMOJI}`, 2)).toBe(`a${EMOJI}`)
  })

  it("cuts after the last whole emoji", () => {
    expect(capCodePoints(`a${EMOJI}${EMOJI}`, 2)).toBe(`a${EMOJI}`)
  })
})

// Each form field checks the limit the server checks, counted the same way.
// A value of `limit` emoji is `2 * limit` UTF-16 units, so a check on
// String.length refuses it at half the limit.
type LimitCase = {
  name: string
  schema: z.ZodType
  valid: Record<string, unknown>
  field: string
  limit: number
}

const cases: LimitCase[] = [
  {
    name: "account name on sign-up",
    schema: registerSchema,
    valid: { name: "Alex", email: "a@b.c", password: "secret", acceptTerms: true },
    field: "name",
    limit: 100,
  },
  {
    name: "account name on the profile",
    schema: profileEditSchema,
    valid: { name: "Alex", defaultGroupId: "" },
    field: "name",
    limit: 100,
  },
  {
    name: "group name",
    schema: createGroupSchema,
    valid: { name: "Home", icon: "", group_currency: "USD", kind: "household" },
    field: "name",
    limit: 100,
  },
  {
    name: "file description",
    schema: fileMetadataSchema,
    valid: { title: "", description: "", path: "manual", category: "documents", tags: [] },
    field: "description",
    limit: 1000,
  },
  {
    name: "loan borrower name",
    schema: lendFormSchema,
    valid: { borrower_name: "Alice", lent_at: "2026-01-01" },
    field: "borrower_name",
    limit: 200,
  },
  {
    name: "maintenance title",
    schema: maintenanceFormSchema,
    valid: { title: "Descale", interval_days: 90 },
    field: "title",
    limit: 200,
  },
  {
    name: "supply link label",
    schema: supplyLinkFormSchema,
    valid: { label: "Filter", url: "https://example.com/filter" },
    field: "label",
    limit: 200,
  },
]

describe("form length limits count code points", () => {
  it.each(cases)("$name accepts the limit in emoji", ({ schema, valid, field, limit }) => {
    const result = schema.safeParse({ ...valid, [field]: EMOJI.repeat(limit) })
    expect(result.error?.issues).toBeUndefined()
  })

  it.each(cases)("$name refuses one emoji over the limit", ({ schema, valid, field, limit }) => {
    const result = schema.safeParse({ ...valid, [field]: EMOJI.repeat(limit + 1) })
    expect(result.error?.issues.map((issue) => issue.path.join("."))).toEqual([field])
  })
})
