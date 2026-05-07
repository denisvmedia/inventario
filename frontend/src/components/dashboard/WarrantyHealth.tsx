import { useTranslation } from "react-i18next"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { WARRANTY_STATUS_CONFIG } from "@/components/warranty/config"
import {
  COMMODITY_WARRANTY_STATUSES,
  type CommodityWarrantyStatus,
} from "@/features/commodities/constants"
import { cn } from "@/lib/utils"

interface WarrantyHealthProps {
  // Per-status counts across the loaded commodities slice. The widths
  // are computed against `total` rather than the page's visible total
  // so a future per-tab variant of the dashboard reads the same way.
  counts: Record<CommodityWarrantyStatus, number>
  isLoading?: boolean
}

// WarrantyHealth is the dashboard's "status distribution across all
// items" card. Four horizontal rows (one per status), each with a
// label, a proportional bar (`bgSolid` from `WARRANTY_STATUS_CONFIG`),
// and the absolute count right-aligned. Mirrors the design mock's
// `<Card>` at the bottom of `DashboardView.tsx`.
export function WarrantyHealth({ counts, isLoading = false }: WarrantyHealthProps) {
  const { t } = useTranslation()
  const total = counts.active + counts.expiring + counts.expired + counts.none
  return (
    <Card data-testid="dashboard-warranty-health">
      <CardHeader>
        <CardTitle className="text-base">{t("dashboard:warrantyHealth.title")}</CardTitle>
        <CardDescription>{t("dashboard:warrantyHealth.description")}</CardDescription>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <ul className="space-y-3" aria-busy="true">
            {Array.from({ length: 4 }).map((_, i) => (
              <li key={i} className="flex items-center gap-3">
                <Skeleton className="h-3 w-24" />
                <Skeleton className="h-2 flex-1 rounded-full" />
                <Skeleton className="h-3 w-6" />
              </li>
            ))}
          </ul>
        ) : total === 0 ? (
          <p
            className="text-sm text-muted-foreground"
            data-testid="dashboard-warranty-health-empty"
          >
            {t("dashboard:warrantyHealth.empty")}
          </p>
        ) : (
          <ul className="space-y-3">
            {COMMODITY_WARRANTY_STATUSES.map((status) => {
              const visual = WARRANTY_STATUS_CONFIG[status]
              const count = counts[status]
              const pct = total > 0 ? Math.round((count / total) * 100) : 0
              return (
                <li
                  key={status}
                  className="flex items-center gap-3"
                  data-testid={`dashboard-warranty-health-${status}`}
                >
                  <span className={cn("w-24 text-xs font-medium", visual.text)}>
                    {t(visual.i18nKey)}
                  </span>
                  <div
                    className="flex-1 h-2 rounded-full bg-muted overflow-hidden"
                    role="progressbar"
                    aria-valuemin={0}
                    aria-valuemax={total}
                    aria-valuenow={count}
                    aria-label={t(visual.i18nKey)}
                  >
                    <div
                      className={cn("h-full rounded-full transition-all", visual.bgSolid)}
                      style={{ width: `${pct}%` }}
                    />
                  </div>
                  <span className="w-8 text-right text-xs text-muted-foreground">{count}</span>
                </li>
              )
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}
