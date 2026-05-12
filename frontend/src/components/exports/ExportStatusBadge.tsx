import { CheckCircle2, Clock, Loader2, XCircle } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import type { ExportStatus, RestoreStatus } from "@/features/export/api"
import { cn } from "@/lib/utils"

type Status = ExportStatus | RestoreStatus

// Tinted-tone treatment lifted from design-mocks/src/views/BackupView.tsx
// (StatusBadge, lines 131-155). One badge serves both export and restore
// surfaces because the design system intentionally uses the same vocabulary.
const STATUS_TONE: Record<Status, string> = {
  pending: "",
  in_progress: "bg-status-expiring/10 text-status-expiring border-0",
  running: "bg-status-expiring/10 text-status-expiring border-0",
  completed: "bg-status-active/10 text-status-active border-0",
  failed: "bg-destructive/10 text-destructive border-0",
}

const STATUS_ICON: Record<Status, React.ComponentType<{ className?: string }>> = {
  pending: Clock,
  in_progress: Loader2,
  running: Loader2,
  completed: CheckCircle2,
  failed: XCircle,
}

const IS_SPINNING: Record<Status, boolean> = {
  pending: false,
  in_progress: true,
  running: true,
  completed: false,
  failed: false,
}

export interface ExportStatusBadgeProps {
  status: Status
  className?: string
}

export function ExportStatusBadge({ status, className }: ExportStatusBadgeProps) {
  const { t } = useTranslation(["exports"])
  const Icon = STATUS_ICON[status]
  return (
    <Badge
      variant="secondary"
      data-testid={`status-${status}`}
      className={cn("gap-1", STATUS_TONE[status], className)}
    >
      <Icon className={cn("size-3", IS_SPINNING[status] && "animate-spin")} aria-hidden="true" />
      {t(`exports:status.${status}`)}
    </Badge>
  )
}
