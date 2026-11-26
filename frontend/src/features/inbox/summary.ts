import { useTranslation } from "react-i18next"

import { INBOX_REASONS, type InboxCounts, type InboxReason } from "./api"

// useInboxSummary returns the inbox's counts in words, one part per reason the
// client knows ("2 drafts to finish · 1 item to review"), or per reason of
// `only` when given. Reasons a newer server added are left out of the words;
// the total still counts them.
export function useInboxSummary(
  counts: InboxCounts | undefined,
  only: readonly InboxReason[] = INBOX_REASONS
): string {
  const { t } = useTranslation(["inbox"])
  if (!counts) return ""
  const parts: string[] = []
  for (const reason of only) {
    const count = counts.reasons[reason] ?? 0
    if (count === 0) continue
    switch (reason) {
      case "draft":
        parts.push(t("inbox:summary.drafts", { count }))
        break
      case "item_draft":
        parts.push(t("inbox:summary.review", { count }))
        break
      case "intake":
        parts.push(t("inbox:summary.intake", { count }))
        break
    }
  }
  return parts.join(" · ")
}
