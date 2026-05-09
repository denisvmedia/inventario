import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import { warrantyStatus, type CommodityWarrantyStatus } from "@/features/commodities/constants"

import { WARRANTY_STATUS_CONFIG } from "./config"

// WarrantyBadge is the only place a warranty status pill is rendered.
// Callers either hand it a precomputed status (`status`) or the raw
// commodity slice (`source`) and the component derives the bucket via
// the shared `warrantyStatus()` helper. The badge picks colour, icon,
// and i18n label from `WARRANTY_STATUS_CONFIG` so the four design
// tokens (`--status-active|expiring|expired|none`) stay the canonical
// colour binding.
//
// Intentional non-features (deferred to follow-ups):
// - No size variants. Every consumer renders the badge inline next to
//   text at the default badge height; if a smaller pill is needed
//   later (e.g. dense tables), add a `size` prop.
// - No "live update" of the status as the day rolls over. The status
//   is computed from the commodity payload at render time.
export interface WarrantyBadgeProps {
  // Either pass an already-classified status, or…
  status?: CommodityWarrantyStatus
  // …a raw commodity slice carrying the warranty fields. Convenient
  // when you have the row in hand; the badge does the bucketing.
  source?: {
    warranty_expires_at?: string
  }
  // Hide the leading shield icon (e.g. inside a row that already
  // shows one). Defaults to `true` — the icon is part of the badge's
  // identity and dropping it should be a deliberate per-call choice.
  showIcon?: boolean
  className?: string
  // Optional test selector. Pass when the badge needs to be located
  // independently of its parent row.
  "data-testid"?: string
}

export function WarrantyBadge({
  status,
  source,
  showIcon = true,
  className,
  "data-testid": testId,
}: WarrantyBadgeProps) {
  const { t } = useTranslation()
  const resolved =
    status ?? warrantyStatus({ warranty_expires_at: source?.warranty_expires_at })
  const visual = WARRANTY_STATUS_CONFIG[resolved]
  const Icon = visual.icon
  return (
    <Badge
      variant="outline"
      className={cn("gap-1 font-medium", visual.text, visual.bg, visual.border, className)}
      data-testid={testId}
      data-status={resolved}
    >
      {showIcon ? <Icon className="size-3" aria-hidden="true" /> : null}
      {t(visual.i18nKey)}
    </Badge>
  )
}
