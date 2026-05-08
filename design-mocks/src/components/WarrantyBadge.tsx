import { Badge } from "@/components/ui/badge"
import { ShieldCheck, ShieldAlert, ShieldOff, Shield } from "lucide-react"
import { warrantyStatus, type InventoryItem, WARRANTY_STATUS_CONFIG } from "@/data/mock"

interface WarrantyBadgeProps {
  item: InventoryItem
  showIcon?: boolean
}

const STATUS_ICONS = {
  active: ShieldCheck,
  expiring: ShieldAlert,
  expired: ShieldOff,
  none: Shield,
}

export function WarrantyBadge({ item, showIcon = true }: WarrantyBadgeProps) {
  const status = warrantyStatus(item)
  const config = WARRANTY_STATUS_CONFIG[status]
  const Icon = STATUS_ICONS[status]

  return (
    <Badge
      variant="outline"
      className={`${config.color} ${config.bg} border-current/20 font-medium`}
    >
      {showIcon && <Icon data-icon="inline-start" className="size-3" />}
      {config.label}
    </Badge>
  )
}
