import { useTranslation } from "react-i18next"

import type { IntakeSummary } from "./api"

// useIntakeSummaryText turns the counts of a batch into one line, such as
// "7 drafts created, 1 file could not be read". Parts with a zero count are
// left out.
export function useIntakeSummaryText(): (s: IntakeSummary) => string {
  const { t } = useTranslation()
  return (s) => {
    const parts: string[] = []
    const queued = s.pending + s.scanning
    if (queued > 0) {
      parts.push(
        t("commodities:intake.summary.scanned", {
          done: s.sources - queued,
          total: s.sources,
        })
      )
    }
    if (s.rateLimited > 0) parts.push(t("commodities:intake.summary.rateLimited"))
    if (s.draftsCreated > 0) {
      parts.push(t("commodities:intake.summary.draftsCreated", { count: s.draftsCreated }))
    }
    if (s.undecided > 0) {
      parts.push(t("commodities:intake.summary.toReview", { count: s.undecided }))
    }
    if (s.candidatesFailed > 0) {
      parts.push(t("commodities:intake.summary.draftsFailed", { count: s.candidatesFailed }))
    }
    if (s.failed > 0) {
      parts.push(t("commodities:intake.summary.unreadable", { count: s.failed }))
    }
    if (s.canceled > 0) {
      parts.push(t("commodities:intake.summary.canceled", { count: s.canceled }))
    }
    if (queued === 0 && s.scanned > 0 && s.candidates === 0) {
      parts.push(t("commodities:intake.summary.nothingFound"))
    }
    return parts.join(", ")
  }
}
