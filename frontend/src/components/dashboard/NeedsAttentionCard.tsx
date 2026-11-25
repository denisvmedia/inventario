import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { ChevronRight, Inbox } from "lucide-react"

import { useCurrentGroup } from "@/features/group/GroupContext"
import { useInboxCount } from "@/features/inbox/hooks"
import { useInboxSummary } from "@/features/inbox/summary"

// NeedsAttentionCard is the dashboard's link to the inbox (#2794): how many
// records still need attention, from the server's count, and what they are.
// It renders nothing while the inbox is empty or its count is loading.
export function NeedsAttentionCard() {
  const { t } = useTranslation(["inbox"])
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const count = useInboxCount()
  const summary = useInboxSummary(count.data)
  const total = count.data?.total ?? 0
  if (!slug || total === 0) return null

  return (
    <Link
      to={`/g/${encodeURIComponent(slug)}/inbox`}
      className="group flex items-center gap-4 rounded-xl border border-border bg-card px-5 py-4 transition-colors hover:border-primary/30 hover:bg-muted/40 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      data-testid="dashboard-needs-attention"
    >
      <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
        <Inbox aria-hidden="true" className="size-5" />
      </div>
      <div className="min-w-0 flex-1">
        <p
          className="text-base font-semibold leading-tight"
          data-testid="dashboard-needs-attention-title"
        >
          {t("inbox:dashboard.title", { total })}
        </p>
        {summary ? (
          <p className="mt-0.5 truncate text-sm text-muted-foreground">{summary}</p>
        ) : null}
      </div>
      <span className="hidden shrink-0 text-sm text-muted-foreground group-hover:text-foreground sm:inline">
        {t("inbox:openInbox")}
      </span>
      <ChevronRight aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
    </Link>
  )
}
