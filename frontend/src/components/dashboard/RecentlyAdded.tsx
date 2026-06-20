import { Link, useLocation, useNavigate } from "react-router-dom"
import { Package } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { WarrantyBadge } from "@/components/warranty/WarrantyBadge"
import type { Commodity } from "@/features/commodities/api"
import { useCurrentGroup } from "@/features/group/GroupContext"

interface RecentlyAddedProps {
  // The five-or-fewer most recent commodities, already sorted. Empty
  // array renders the card's empty-state copy.
  items: Commodity[]
  // True while the upstream query is loading on first render. Renders
  // skeleton rows instead of the list.
  isLoading?: boolean
}

// RecentlyAdded is the right-hand list on the dashboard. A plain click on
// a row opens the commodity detail in the slide-out sheet overlay (mock
// parity, #1581 item 6) by stamping the current dashboard URL onto
// `state.background` — the router (app/router.tsx) reads that state and
// renders CommodityDetailSheet on top of the dashboard. This is the same
// row→sheet pattern the items list uses (CommoditiesListPage's
// `openCommodityInSheet`). Modifier / middle clicks fall through to the
// underlying <Link> so "open in new tab" lands on the full detail page (a
// new document carries no `state.background`, so the overlay tree stays
// unmounted there).
export function RecentlyAdded({ items, isLoading = false }: RecentlyAddedProps) {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const navigate = useNavigate()
  const location = useLocation()
  const slug = currentGroup?.slug
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex items-center gap-2 text-base">
          <Package aria-hidden="true" className="size-4" />
          {t("dashboard:recentlyAdded.title")}
        </CardTitle>
        <CardDescription>{t("dashboard:recentlyAdded.description")}</CardDescription>
      </CardHeader>
      <CardContent className="p-0">
        {isLoading ? (
          <ul aria-busy="true">
            {Array.from({ length: 3 }).map((_, i) => (
              <li key={i}>
                {i > 0 && <Separator />}
                <div className="flex items-center gap-3 px-6 py-3.5">
                  <Skeleton className="size-8 rounded-md" />
                  <div className="flex-1 space-y-1.5">
                    <Skeleton className="h-3 w-32" />
                    <Skeleton className="h-3 w-20" />
                  </div>
                </div>
              </li>
            ))}
          </ul>
        ) : items.length === 0 ? (
          <p className="px-6 pb-6 text-sm text-muted-foreground">
            {t("dashboard:recentlyAdded.empty")}
          </p>
        ) : (
          <ul>
            {items.map((item, i) => (
              <li key={item.id ?? i}>
                {i > 0 && <Separator />}
                <Link
                  to={
                    slug && item.id
                      ? `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(item.id)}`
                      : "#"
                  }
                  onClick={(e) => {
                    // Plain left-click → open the slide-out sheet over the
                    // dashboard. Let modifier / middle clicks through so the
                    // browser opens the full page in a new tab.
                    if (!slug || !item.id) return
                    if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return
                    e.preventDefault()
                    navigate(
                      `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(item.id)}`,
                      { state: { background: location } }
                    )
                  }}
                  className="flex w-full items-center justify-between gap-3 px-6 py-3.5 text-left transition-colors hover:bg-muted/50 focus-visible:bg-muted/50 outline-none"
                  data-testid="recently-added-row"
                >
                  <div className="flex items-center gap-3 min-w-0">
                    <div
                      aria-hidden="true"
                      className="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted"
                    >
                      <Package className="size-4 text-muted-foreground" />
                    </div>
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">
                        {item.short_name || item.name || t("dashboard:recentlyAdded.untitled")}
                      </p>
                      {item.serial_number ? (
                        <p className="truncate text-xs text-muted-foreground">
                          {item.serial_number}
                        </p>
                      ) : null}
                    </div>
                  </div>
                  {/* Mock parity (#1544 item 3): right-aligned warranty
                      pill so the dashboard's first-glance surface carries
                      the same warranty-status signal as the list pages.
                      `WarrantyBadge` derives status from
                      `warranty_expires_at` via the shared bucketing
                      helper — `none` is rendered explicitly, the row is
                      never blank. */}
                  <WarrantyBadge
                    source={{ warranty_expires_at: item.warranty_expires_at }}
                    className="shrink-0"
                    data-testid="recently-added-warranty"
                  />
                </Link>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
