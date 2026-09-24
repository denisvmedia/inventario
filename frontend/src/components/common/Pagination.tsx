import { ChevronLeft, ChevronRight } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"

export interface PaginationProps {
  page: number
  totalPages: number
  onChange: (page: number) => void
  /** Distinguishes the nav and its page buttons when a view has more than one. */
  testId?: string
}

/**
 * Server-side pager shared by every paginated list (#2467). Renders nothing
 * about the data itself: the caller owns the page state and the query, and
 * hides the whole control when there is only one page.
 */
export function Pagination({ page, totalPages, onChange, testId = "pagination" }: PaginationProps) {
  const { t } = useTranslation()
  const pages = pageRange(page, totalPages)

  return (
    <nav
      className="flex items-center justify-center gap-1"
      aria-label={t("common:pagination.label")}
      data-testid={testId}
    >
      <Button
        variant="ghost"
        size="icon"
        className="size-8"
        onClick={() => onChange(page - 1)}
        disabled={page <= 1}
        aria-label={t("common:pagination.previous")}
      >
        <ChevronLeft className="size-4" aria-hidden="true" />
      </Button>
      {pages.map((p, i) =>
        p === "ellipsis" ? (
          <span key={`e-${i}`} className="px-2 text-sm text-muted-foreground" aria-hidden="true">
            …
          </span>
        ) : (
          <Button
            key={p}
            variant={p === page ? "secondary" : "ghost"}
            size="sm"
            className="size-8"
            onClick={() => onChange(p)}
            aria-current={p === page ? "page" : undefined}
            data-testid={`${testId}-page-${p}`}
          >
            {p}
          </Button>
        )
      )}
      <Button
        variant="ghost"
        size="icon"
        className="size-8"
        onClick={() => onChange(page + 1)}
        disabled={page >= totalPages}
        aria-label={t("common:pagination.next")}
      >
        <ChevronRight className="size-4" aria-hidden="true" />
      </Button>
    </nav>
  )
}

/**
 * The page numbers to render, plus "ellipsis" markers. Always includes the
 * first and last page and collapses the middle, so the control keeps a fixed
 * width however many pages there are. The caller treats "ellipsis" as a
 * non-clickable separator.
 */
export function pageRange(current: number, total: number): Array<number | "ellipsis"> {
  if (total <= 7) {
    return Array.from({ length: total }, (_, i) => i + 1)
  }
  const out: Array<number | "ellipsis"> = [1]
  const start = Math.max(2, current - 1)
  const end = Math.min(total - 1, current + 1)
  if (start > 2) out.push("ellipsis")
  for (let p = start; p <= end; p++) out.push(p)
  if (end < total - 1) out.push("ellipsis")
  out.push(total)
  return out
}

/**
 * Folds a requested page back inside the range the server reports, and returns
 * it alongside the page count.
 *
 * The fold runs during render rather than in an effect, so the next render —
 * and the query it drives — already asks for a page that exists. An effect
 * would commit the out-of-range page first, and the user would see an empty
 * list under a pager that has no way back: with `totalPages` down to 1 the
 * control is hidden entirely. Deleting the last item on the last page is the
 * case that hits this.
 *
 * `total` is undefined until the first response, when the requested page is
 * still 1, so nothing is folded on the way in.
 */
export function pageWithin(
  requested: number,
  setPage: (page: number) => void,
  total: number | undefined,
  pageSize: number
): { page: number; totalPages: number } {
  const totalPages = Math.max(1, Math.ceil((total ?? 0) / pageSize))
  if (total !== undefined && requested > totalPages) {
    setPage(totalPages)
  }
  return { page: Math.min(requested, totalPages), totalPages }
}
