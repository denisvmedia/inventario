import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import type { CurrencyMigrationStatus } from "@/features/currency-migration/api"
import { cn } from "@/lib/utils"

const STATUS_VARIANT: Record<
  CurrencyMigrationStatus,
  "secondary" | "default" | "destructive" | "outline"
> = {
  pending: "secondary",
  running: "default",
  completed: "outline",
  failed: "destructive",
}

const STATUS_TONE: Record<CurrencyMigrationStatus, string> = {
  pending: "",
  running: "",
  completed: "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
  failed: "",
}

export interface CurrencyMigrationStatusBadgeProps {
  status: CurrencyMigrationStatus
  className?: string
}

export function CurrencyMigrationStatusBadge({
  status,
  className,
}: CurrencyMigrationStatusBadgeProps) {
  const { t } = useTranslation()
  return (
    <Badge
      variant={STATUS_VARIANT[status]}
      data-testid={`migration-status-${status}`}
      className={cn(STATUS_TONE[status], className)}
    >
      {t(`groups:migration.status.${status}`)}
    </Badge>
  )
}
