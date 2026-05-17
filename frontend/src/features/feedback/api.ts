// Feedback feature slice (issue #1387). Single endpoint —
// POST /api/v1/feedback — auth-required, per-user rate-limited (5/hour
// at the BE; the FE surfaces a 429 via a friendlier "try again later"
// toast).
//
// The route is tenant-scoped (not group-scoped) so we skip the /g/{slug}/
// rewrite — the BE mounts it at /api/v1/feedback alongside /users/me.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

// FeedbackType keys match the BE allow-list in apiserver/feedback.go.
// Keep these in sync — adding a new variant on either side without
// updating the other produces a 400 the user can't action.
export type FeedbackType = "feedback" | "bug" | "feature" | "question"

export const FEEDBACK_TYPES: readonly FeedbackType[] = [
  "feedback",
  "bug",
  "feature",
  "question",
] as const

export type FeedbackRequest = Schema<"apiserver.FeedbackRequest">
export type FeedbackResponse = Schema<"apiserver.FeedbackResponse">

export interface SubmitFeedbackInput {
  type: FeedbackType
  message: string
  replyToEmail?: string
  // Diagnostics is a free-form `{ label: value }` map. The BE caps the
  // count (32) and per-line size (1 KB) and sorts keys alphabetically
  // before rendering, so the FE can submit whatever it wants in any
  // order.
  diagnostics?: Record<string, string>
}

export async function submitFeedback(input: SubmitFeedbackInput): Promise<FeedbackResponse> {
  const body: FeedbackRequest = {
    type: input.type,
    message: input.message,
    reply_to_email: input.replyToEmail?.trim() || undefined,
    diagnostics:
      input.diagnostics && Object.keys(input.diagnostics).length > 0
        ? input.diagnostics
        : undefined,
  }
  return http.post<FeedbackResponse>("/feedback", body, { skipGroupRewrite: true })
}
