import { Link } from "react-router-dom"
import { Package } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
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

// RecentlyAdded is the right-hand list on the dashboard. Each row links
// to /g/:slug/commodities/:id (the items detail page lives in #1410;
// the placeholder mounted there today still renders a recognisable
// "Coming soon" stub, so the click target is not a dead end).
export function RecentlyAdded({ items, isLoading = false }: RecentlyAddedProps) {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
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
                  to={slug ? `/g/${slug}/commodities/${item.id}` : "#"}
                  className="flex w-full items-center justify-between px-6 py-3.5 text-left transition-colors hover:bg-muted/50 focus-visible:bg-muted/50 outline-none"
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
                </Link>
              </li>
            ))}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
