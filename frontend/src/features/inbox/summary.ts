import { useTranslation } from "react-i18next"

import type { InboxCounts } from "./api"

// useInboxSummary returns the inbox's counts in words, one part per reason the
// client knows ("2 drafts to finish · 1 item to review"). Reasons a newer
// server added are left out of the words; the total still counts them.
export function useInboxSummary(counts: InboxCounts | undefined): string {
  const { t } = useTranslation(["inbox"])
  if (!counts) return ""
  const parts: string[] = []
  const drafts = counts.reasons.draft ?? 0
  const review = counts.reasons.item_draft ?? 0
  if (drafts > 0) parts.push(t("inbox:summary.drafts", { count: drafts }))
  if (review > 0) parts.push(t("inbox:summary.review", { count: review }))
  return parts.join(" · ")
}
