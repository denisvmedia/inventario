import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { ChevronLeft, ChevronRight } from "lucide-react"

import { Button } from "@/components/ui/button"
import type { ListNavigation, ListNeighbor } from "@/hooks/useListNavigation"
import { cn } from "@/lib/utils"

export interface ListPagerProps {
  nav: ListNavigation
  /** Where Previous / Next lead. */
  hrefFor: (target: ListNeighbor) => string
  /** Router state the links carry, such as the background of a modal route. */
  linkState?: unknown
  /**
   * Makes Previous / Next buttons that call this instead of links, for a
   * surface that has to ask about unsaved edits before it leaves.
   */
  onNavigate?: (target: ListNeighbor) => void
  labels: { pager: string; previous: string; next: string }
  /** Test ids are `<prefix>-pager`, `-previous`, `-position` and `-next`. */
  testIdPrefix: string
  className?: string
}

// Previous / position / Next for a record opened from a list; see
// hooks/useListNavigation.ts for where the neighbors come from.
export function ListPager({
  nav,
  hrefFor,
  linkState,
  onNavigate,
  labels,
  testIdPrefix,
  className,
}: ListPagerProps) {
  const { t } = useTranslation()
  return (
    <nav
      className={cn("flex items-center gap-2", className)}
      aria-label={labels.pager}
      data-testid={`${testIdPrefix}-pager`}
    >
      <PagerButton
        target={nav.previous}
        hrefFor={hrefFor}
        linkState={linkState}
        onNavigate={onNavigate}
        label={labels.previous}
        testId={`${testIdPrefix}-previous`}
      >
        <ChevronLeft className="size-4" aria-hidden="true" />
      </PagerButton>
      <span
        className="min-w-16 text-center text-sm tabular-nums text-muted-foreground"
        data-testid={`${testIdPrefix}-position`}
      >
        {nav.position !== null && nav.total !== null
          ? t("common:listNav.position", { position: nav.position, total: nav.total })
          : nav.loading
            ? "…"
            : "—"}
      </span>
      <PagerButton
        target={nav.next}
        hrefFor={hrefFor}
        linkState={linkState}
        onNavigate={onNavigate}
        label={labels.next}
        testId={`${testIdPrefix}-next`}
      >
        <ChevronRight className="size-4" aria-hidden="true" />
      </PagerButton>
    </nav>
  )
}

function PagerButton({
  target,
  hrefFor,
  linkState,
  onNavigate,
  label,
  testId,
  children,
}: {
  target: ListNeighbor | null
  hrefFor: (target: ListNeighbor) => string
  linkState?: unknown
  onNavigate?: (target: ListNeighbor) => void
  label: string
  testId: string
  children: ReactNode
}) {
  if (!target) {
    return (
      <Button variant="outline" size="icon-sm" disabled aria-label={label} data-testid={testId}>
        {children}
      </Button>
    )
  }
  if (onNavigate) {
    return (
      <Button
        type="button"
        variant="outline"
        size="icon-sm"
        onClick={() => onNavigate(target)}
        aria-label={label}
        data-testid={testId}
      >
        {children}
      </Button>
    )
  }
  return (
    <Button asChild variant="outline" size="icon-sm">
      <Link to={hrefFor(target)} state={linkState} aria-label={label} data-testid={testId}>
        {children}
      </Link>
    </Button>
  )
}
