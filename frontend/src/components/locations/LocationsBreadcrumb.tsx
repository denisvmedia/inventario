import type { ReactNode } from "react"
import { Link } from "react-router-dom"
import { ArrowLeft, ChevronRight } from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export interface BreadcrumbSegment {
  label: ReactNode
  to?: string
  testId?: string
}

interface LocationsBreadcrumbProps {
  // Ordered root → current. The last segment renders as the current
  // page (bold, non-interactive) regardless of whether it carries a
  // `to`. Earlier segments with a `to` are rendered as Links; without
  // a `to` they fall back to muted plain text (e.g. while a parent
  // detail still loads). "#" / empty strings are treated as absent so
  // a placeholder href doesn't leave the user one click away from a
  // bare-hash URL change.
  segments: BreadcrumbSegment[]
  // Optional leading back-chevron target. Mirrors
  // `design-mocks/src/views/LocationPickerView.tsx` L460-L465: an
  // ArrowLeft button that always walks one level up. Hidden when
  // omitted or when the caller passes a placeholder ("#" / "").
  backHref?: string
  // aria-label for the back-chevron button. Hidden from sighted users.
  backLabel?: string
  // aria-label for the `<nav>` itself. Defaults to "Breadcrumb" so screen
  // readers always announce the landmark even when the caller wires
  // backLabel only.
  navLabel?: string
  className?: string
  testId?: string
}

// Treat "#" and empty strings as "no destination" — callers wire these
// up before `currentGroup.slug` resolves and we don't want the back
// button (or a clickable parent segment) to leave the user one click
// away from a bare-hash URL change.
function realHref(href: string | undefined): string | undefined {
  if (!href) return undefined
  if (href === "#") return undefined
  return href
}

// Multi-segment breadcrumb used on location and area detail pages.
// Ports `design-mocks/src/views/LocationPickerView.tsx` L459-L498:
// optional left ArrowLeft + chevron-separated clickable segments with
// the current one bold. Non-sticky here — the existing app shell already
// owns the sticky TopBar slot, so adding a second sticky strip below it
// would compete for the top edge of the viewport.
export function LocationsBreadcrumb({
  segments,
  backHref,
  backLabel,
  navLabel,
  className,
  testId,
}: LocationsBreadcrumbProps) {
  if (segments.length === 0) return null
  const lastIndex = segments.length - 1
  const effectiveBackHref = realHref(backHref)
  return (
    <nav
      aria-label={navLabel ?? "Breadcrumb"}
      className={cn("flex items-center gap-2", className)}
      data-testid={testId ?? "locations-breadcrumb"}
    >
      {effectiveBackHref ? (
        <Button
          asChild
          variant="ghost"
          size="icon"
          className="-ml-1 size-8 shrink-0"
          aria-label={backLabel ?? "Back"}
          data-testid="locations-breadcrumb-back"
        >
          <Link to={effectiveBackHref}>
            <ArrowLeft className="size-4" aria-hidden="true" />
          </Link>
        </Button>
      ) : null}
      <ol className="flex min-w-0 flex-1 items-center gap-1.5">
        {segments.map((segment, i) => {
          const isCurrent = i === lastIndex
          const segmentHref = realHref(segment.to)
          return (
            <li key={i} className="flex min-w-0 items-center gap-1.5">
              {i > 0 ? (
                <ChevronRight
                  className="size-3.5 shrink-0 text-muted-foreground"
                  aria-hidden="true"
                />
              ) : null}
              {isCurrent || !segmentHref ? (
                <span
                  className={cn(
                    "truncate text-sm",
                    isCurrent ? "font-semibold text-foreground" : "text-muted-foreground"
                  )}
                  aria-current={isCurrent ? "page" : undefined}
                  data-testid={segment.testId}
                >
                  {segment.label}
                </span>
              ) : (
                <Link
                  to={segmentHref}
                  className="truncate text-sm text-muted-foreground transition-colors hover:text-foreground"
                  data-testid={segment.testId}
                >
                  {segment.label}
                </Link>
              )}
            </li>
          )
        })}
      </ol>
    </nav>
  )
}
