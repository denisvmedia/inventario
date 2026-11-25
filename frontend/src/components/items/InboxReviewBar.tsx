import { useTranslation } from "react-i18next"
import { ListChecks } from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

interface InboxReviewBarProps {
  // 1-based place of the item in the Items to review list, and the list's
  // length; null while unknown.
  position: number | null
  total: number | null
  hasNext: boolean
  pending: boolean
  disabled: boolean
  disabledReason?: string
  onSkip: () => void
  onReview: () => void
  className?: string
}

// InboxReviewBar sits on the page of an item opened from the inbox while the
// item is marked as draft (#2794). Mark as reviewed clears the mark and moves
// to the next item to review; Skip moves on without changing anything. With
// the item page's own editing in between, the items to review can be checked
// one after another without going back to the inbox.
export function InboxReviewBar({
  position,
  total,
  hasNext,
  pending,
  disabled,
  disabledReason,
  onSkip,
  onReview,
  className,
}: InboxReviewBarProps) {
  const { t } = useTranslation(["inbox"])
  return (
    <section
      aria-label={t("inbox:walk.label")}
      className={cn(
        "flex flex-wrap items-center gap-x-3 gap-y-2 rounded-lg border border-border bg-card px-4 py-2.5",
        className
      )}
      data-testid="commodity-review-bar"
    >
      <ListChecks className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      <p className="min-w-0 flex-1 basis-48 text-sm">
        {position !== null && total !== null ? (
          <span className="font-medium" data-testid="commodity-review-position">
            {t("inbox:walk.position", { position, total })}
          </span>
        ) : null}{" "}
        <span className="text-muted-foreground">{t("inbox:itemDraftHint")}</span>
      </p>
      <div className="flex shrink-0 items-center gap-2">
        {hasNext ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={onSkip}
            data-testid="commodity-review-skip"
          >
            {t("inbox:walk.skip")}
          </Button>
        ) : null}
        <Button
          type="button"
          size="sm"
          onClick={onReview}
          disabled={pending || disabled}
          title={disabled ? disabledReason : undefined}
          data-testid="commodity-review-mark"
        >
          {t("inbox:markReviewed")}
        </Button>
      </div>
    </section>
  )
}
