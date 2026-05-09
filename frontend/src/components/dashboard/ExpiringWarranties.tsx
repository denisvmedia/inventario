import { Link } from "react-router-dom"
import { ShieldAlert } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { WARRANTY_STATUS_CONFIG } from "@/components/warranty/config"
import type { ExpiringWarrantyRow } from "@/features/dashboard/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { cn } from "@/lib/utils"

interface ExpiringWarrantiesProps {
  // Up to 5 commodities whose warranty falls in the "expiring" bucket
  // (≤60 days from expiry), pre-sorted by expiry ascending.
  items: ExpiringWarrantyRow[]
  // True while the parent dashboard query is on its first fetch.
  // Renders skeleton rows so the panel doesn't pop in.
  isLoading?: boolean
}

function daysUntil(dateStr: string | undefined): number | null {
  if (!dateStr) return null
  const t = Date.parse(`${dateStr}T00:00:00Z`)
  if (Number.isNaN(t)) return null
  const now = new Date()
  const todayUTC = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate())
  return Math.round((t - todayUTC) / (1000 * 60 * 60 * 24))
}

// ExpiringWarranties is the dashboard's left-hand panel above the
// Warranty Health card. Mirrors the design mock's "Expiring Warranties"
// list — each row shows item name, optional short-name secondary line,
// and a status-coloured "N days left" pill on the right. Rows are
// links so click + keyboard activation drill into the item's detail
// page (Warranty tab preselected).
export function ExpiringWarranties({ items, isLoading = false }: ExpiringWarrantiesProps) {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const visual = WARRANTY_STATUS_CONFIG.expiring
  return (
    <Card data-testid="dashboard-expiring-warranties">
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <ShieldAlert aria-hidden="true" className={cn("size-4", visual.text)} />
          {t("dashboard:expiringWarranties.title")}
        </CardTitle>
        <CardDescription>{t("dashboard:expiringWarranties.description")}</CardDescription>
      </CardHeader>
      <CardContent className="p-0">
        {isLoading ? (
          <ul aria-busy="true">
            {Array.from({ length: 3 }).map((_, i) => (
              <li key={i}>
                {i > 0 ? <Separator /> : null}
                <div className="flex items-center justify-between gap-4 px-6 py-3.5">
                  <div className="flex-1 space-y-1.5">
                    <Skeleton className="h-3 w-32" />
                    <Skeleton className="h-3 w-20" />
                  </div>
                  <Skeleton className="h-5 w-20" />
                </div>
              </li>
            ))}
          </ul>
        ) : items.length === 0 ? (
          <p
            className="px-6 pb-6 text-sm text-muted-foreground"
            data-testid="dashboard-expiring-warranties-empty"
          >
            {t("dashboard:expiringWarranties.empty")}
          </p>
        ) : (
          <ul>
            {items.map(({ commodity, expiresAt }, i) => {
              const days = daysUntil(expiresAt)
              const id = commodity.id ?? ""
              const subtitle =
                commodity.short_name && commodity.short_name !== commodity.name
                  ? commodity.short_name
                  : null
              return (
                <li key={id || i}>
                  {i > 0 ? <Separator /> : null}
                  <Link
                    to={
                      id && slug
                        ? `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(id)}?tab=warranty`
                        : "#"
                    }
                    className="flex w-full items-center justify-between px-6 py-3.5 text-left transition-colors hover:bg-muted/50 focus-visible:bg-muted/50 outline-none"
                    data-testid="dashboard-expiring-warranty-row"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">
                        {commodity.name ?? t("dashboard:recentlyAdded.untitled")}
                      </p>
                      {subtitle ? (
                        <p className="truncate text-xs text-muted-foreground">{subtitle}</p>
                      ) : null}
                    </div>
                    {days !== null ? (
                      <Badge
                        variant="outline"
                        className={cn("shrink-0 ml-4", visual.text, visual.bg, visual.border)}
                      >
                        {days >= 0
                          ? t("dashboard:expiringWarranties.daysLeft", { count: days })
                          : t("dashboard:expiringWarranties.daysAgo", { count: -days })}
                      </Badge>
                    ) : null}
                  </Link>
                </li>
              )
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
