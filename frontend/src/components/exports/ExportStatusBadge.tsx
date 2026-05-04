import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import type { ExportStatus, RestoreStatus } from "@/features/export/api"
import { cn } from "@/lib/utils"

type Status = ExportStatus | RestoreStatus

// Hue mapping mirrors the design mock's "Backup" view: pending stays
// neutral, in-flight uses the brand accent, completed is success-green,
// failed is destructive. The same map serves both export and restore
// statuses because the design system intentionally renders them with
// the same vocabulary.
const STATUS_VARIANT: Record<Status, "secondary" | "default" | "destructive" | "outline"> = {
  pending: "secondary",
  in_progress: "default",
  running: "default",
  completed: "outline",
  failed: "destructive",
}

const STATUS_TONE: Record<Status, string> = {
  pending: "",
  in_progress: "",
  running: "",
  completed: "border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300",
  failed: "",
}

export interface ExportStatusBadgeProps {
  status: Status
  className?: string
}

export function ExportStatusBadge({ status, className }: ExportStatusBadgeProps) {
  const { t } = useTranslation(["exports"])
  return (
    <Badge
      variant={STATUS_VARIANT[status]}
      data-testid={`status-${status}`}
      className={cn(STATUS_TONE[status], className)}
    >
      {t(`exports:status.${status}`)}
    </Badge>
  )
}
