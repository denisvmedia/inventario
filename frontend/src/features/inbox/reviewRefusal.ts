import type { TFunction } from "i18next"

import { itemFieldProblem } from "@/features/commodities/bulkFailureReason"
import { extractFieldErrorCodes, extractFieldErrors } from "@/lib/server-error"

// reviewRefusal says what to fix when the server refuses to clear an item's
// draft mark. The refusal is a 422 whose only content is the tree of field
// errors and their codes, so the sentence is built from those; undefined
// when err carries no field errors.
export function reviewRefusal(t: TFunction, err: unknown): string | undefined {
  const messages = extractFieldErrors(err)
  if (!messages) return undefined
  const codes = extractFieldErrorCodes(err)
  return Object.entries(messages)
    .map(([path, message]) => {
      const detail = codes?.[path]
      return itemFieldProblem(t, path.split(".")[0], detail?.code ?? "", message, detail?.params)
    })
    .join(" ")
}
