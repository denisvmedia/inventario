import type { ReactNode } from "react"
import { Link } from "react-router-dom"
import type { LucideIcon } from "lucide-react"

import { Card, CardContent, CardDescription, CardHeader } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { cn } from "@/lib/utils"

interface StatCardProps {
  // Localised label rendered above the value (e.g. "Total items"). Acts
  // as the card's accessible heading via aria-labelledby.
  label: string
  // Optional sub-line below the value (e.g. "across all locations" or
  // "first-class warranties coming soon"). Falls back to nothing if
  // omitted.
  sub?: ReactNode
  // The headline value. `string` for already-formatted output (currency,
  // "—" for missing). `number` for raw counts the card formats.
  value: string | number
  // Lucide icon that anchors the top-right corner. Optional — stat cards
  // for unsupported metrics omit it.
  icon?: LucideIcon
  // Tailwind colour class applied to the icon + value (e.g.
  // `text-status-active`). Defaults to `text-foreground`.
  tone?: string
  // When provided, the entire card becomes a router link so click +
  // keyboard activation drill into the matching filtered list.
  to?: string
  // True while upstream data is loading; renders skeletons in place of
  // value + sub line so the layout doesn't shift on resolve.
  isLoading?: boolean
  // data-testid hook so smoke tests can target a specific card without
  // depending on its label text.
  testId?: string
}

// StatCard renders one of the dashboard's four headline metrics. The
// surrounding grid is owned by Dashboard.tsx; this component is the
// individual cell. When `to` is set the whole card is wrapped in a
// `<Link>` — the focus ring and hover affordance live on the inner
// `<Card>` so the keyboard story matches a button.
export function StatCard({
  label,
  sub,
  value,
  icon: Icon,
  tone = "text-foreground",
  to,
  isLoading = false,
  testId,
}: StatCardProps) {
  const labelId = `${testId ?? `stat-${slugify(label)}`}-label`
  const card = (
    <Card
      className={cn(
        "gap-3 transition-colors",
        to && "hover:border-primary/30 hover:bg-muted/40 focus-within:ring-2 focus-within:ring-ring"
      )}
      aria-labelledby={labelId}
      data-testid={testId}
    >
      <CardHeader className="pb-2">
        <div className="flex items-center justify-between">
          <CardDescription id={labelId} className="text-xs font-medium uppercase tracking-wide">
            {label}
          </CardDescription>
          {Icon ? <Icon aria-hidden="true" className={cn("size-4", tone)} /> : null}
        </div>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <>
            <Skeleton className="h-7 w-20" />
            {sub ? <Skeleton className="mt-2 h-3 w-28" /> : null}
          </>
        ) : (
          <>
            <p className={cn("text-2xl font-bold tracking-tight", tone)}>{value}</p>
            {sub ? <p className="mt-0.5 text-xs text-muted-foreground">{sub}</p> : null}
          </>
        )}
      </CardContent>
    </Card>
  )
  if (!to) return card
  return (
    <Link to={to} className="rounded-xl outline-none focus-visible:ring-2 focus-visible:ring-ring">
      {card}
    </Link>
  )
}

// slugify produces a stable testid suffix from the localised label. We
// intentionally keep it ASCII-only so non-Latin labels still emit a
// usable id (cyrillic / czech accented chars are dropped, leaving the
// fallback `stat-` prefix to disambiguate via order).
function slugify(label: string): string {
  return label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
}
