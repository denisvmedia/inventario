import { Link } from "react-router-dom"
import type { LucideIcon } from "lucide-react"

import { cn } from "@/lib/utils"

interface StatTeaserCardProps {
  // Localised caption rendered above the value ("Items tracked").
  label: string
  // Pre-formatted headline value ("12", "$3,140"). Caller owns formatting
  // so currency / locale stays under one roof.
  value: string
  // Optional Lucide icon for the left-rail mark. Omit for the auth-panel
  // variant where the value stands on its own.
  icon?: LucideIcon
  // When set, the whole card becomes a router link. NoGroupPage uses fixed
  // illustrative numbers and leaves this off; HomePage variants pass real
  // drill-down targets.
  href?: string
  // Test handle so smoke tests can target a specific card without
  // depending on its localised label.
  testId?: string
  className?: string
}

// StatTeaserCard renders one cell of an illustrative or summary stats
// strip — three cards in NoGroupPage above the create-group CTA, and a
// reusable building block for any future "value-prop at a glance" surface.
// Layout follows the "Stats Row" recipe in design-mocks/CLAUDE.md §5
// (icon-rail on the left, label-over-value stack on the right) so the
// teaser inherits the rest of the app's visual rhythm without a new
// design token.
export function StatTeaserCard({
  label,
  value,
  icon: Icon,
  href,
  testId,
  className,
}: StatTeaserCardProps) {
  const body = (
    <>
      {Icon ? (
        <div className="flex size-8 items-center justify-center rounded-lg bg-muted shrink-0">
          <Icon aria-hidden="true" className="size-4 text-muted-foreground" />
        </div>
      ) : null}
      <div className="min-w-0">
        <p className="text-xs text-muted-foreground truncate">{label}</p>
        <p className="text-lg font-semibold leading-tight">{value}</p>
      </div>
    </>
  )

  const base = cn(
    "rounded-xl border border-border bg-card px-4 py-3 flex items-center gap-3",
    className
  )

  if (!href) {
    return (
      <div className={base} data-testid={testId}>
        {body}
      </div>
    )
  }

  return (
    <Link
      to={href}
      data-testid={testId}
      className={cn(
        base,
        "transition-colors hover:border-primary/30 hover:bg-muted/40",
        "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      )}
    >
      {body}
    </Link>
  )
}
