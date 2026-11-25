import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { Inbox } from "lucide-react"

import { Button } from "@/components/ui/button"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useInboxCount } from "@/features/inbox/hooks"
import { useInboxSummary } from "@/features/inbox/summary"

// InboxNotice is the Items page's way to the drafts (#2794). Server drafts
// are not items, so the item filters cannot list them; the inbox lists them
// together with the items still marked as draft, which the default list
// hides. The notice renders nothing while neither kind exists.
export function InboxNotice() {
  const { t } = useTranslation(["inbox"])
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const count = useInboxCount()
  const summary = useInboxSummary(count.data)
  if (!slug || !summary) return null
  const drafts = count.data?.reasons.draft ?? 0
  const review = count.data?.reasons.item_draft ?? 0
  // Land on the tab that holds them when only one kind is listed.
  const reason =
    drafts > 0 && review === 0 ? "draft" : review > 0 && drafts === 0 ? "item_draft" : ""
  const href = `/g/${encodeURIComponent(slug)}/inbox${reason ? `?reason=${reason}` : ""}`

  return (
    <div
      className="flex flex-wrap items-center gap-x-3 gap-y-2 rounded-lg border border-border bg-card px-4 py-2.5"
      data-testid="items-inbox-notice"
    >
      <Inbox className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      <p className="min-w-0 flex-1 text-sm" data-testid="items-inbox-notice-summary">
        {summary}
      </p>
      <Button asChild variant="outline" size="sm">
        <Link to={href} data-testid="items-inbox-notice-open">
          {t("inbox:openInbox")}
        </Link>
      </Button>
    </div>
  )
}
