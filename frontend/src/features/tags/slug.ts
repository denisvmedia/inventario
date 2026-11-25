// normaliseSlug has no dependencies, so modules on the app's first load
// (the items query) can use it without pulling in zod, whose eval probe the
// Content-Security-Policy reports as a violation.

const LETTER_OR_NUMBER = /[\p{L}\p{N}]/u
const MARK = /\p{M}/u

// normaliseSlug mirrors models.NormalizeTagSlug on the BE, step for step:
// NFC, full Unicode lowercase, NFC again; keep letters and numbers of any
// script plus the combining marks that follow them; turn every other run
// into one `-`; drop leading and trailing `-`. The two are tested against
// the same table (go/models/tag_test.go tagSlugCases and
// __tests__/schemas.test.ts) and must change together. Kept client-side so
// the create form can show a live preview as the user types the label.
export function normaliseSlug(input: string): string {
  const lowered = input.normalize("NFC").toLowerCase().normalize("NFC")
  let out = ""
  let inRun = false
  let pendingDash = false
  for (const ch of lowered) {
    if (LETTER_OR_NUMBER.test(ch)) {
      if (pendingDash) {
        out += "-"
        pendingDash = false
      }
      out += ch
      inRun = true
    } else if (inRun && MARK.test(ch)) {
      out += ch
    } else {
      // A mark with nothing to attach to counts as a separator too.
      inRun = false
      pendingDash = out.length > 0
    }
  }
  return out
}
