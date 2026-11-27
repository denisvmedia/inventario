import type { ElementType } from "react"
import { Crown, Eye, Shield, User as UserIcon } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import type { GroupRole } from "@/features/group/api"
import { cn } from "@/lib/utils"

// Role styling mirrors design-mocks/src/views/MembersView.tsx (ROLE_CONFIG,
// RoleBadge): an icon and a tint per role on a borderless secondary badge.
// The label is `members:roles.<role>`.
export const ROLE_ICON: Record<GroupRole, ElementType> = {
  viewer: Eye,
  user: UserIcon,
  admin: Shield,
  owner: Crown,
}

// viewer stays neutral, user takes the cool chart tint, admin the primary
// color and owner the amber accent.
export const ROLE_BADGE_CLASS: Record<GroupRole, string> = {
  viewer: "bg-muted text-muted-foreground border-0",
  user: "bg-chart-3/10 text-chart-3 border-0",
  admin: "bg-primary/10 text-primary border-0",
  owner: "bg-accent text-accent-foreground border-0",
}

function isGroupRole(role: string | null | undefined): role is GroupRole {
  return !!role && Object.hasOwn(ROLE_ICON, role)
}

interface RoleBadgeProps {
  // A role the app does not know, or none, renders a neutral em dash.
  role: string | null | undefined
  // Merged over the base classes, e.g. `text-xs` for the admin pages.
  className?: string
  // Defaults to `role-badge-<role>`.
  testId?: string
}

// RoleBadge draws a group role the same way on every surface: the Members
// page, the admin pages and the top bar.
export function RoleBadge({ role, className, testId }: RoleBadgeProps) {
  const { t } = useTranslation()
  if (!isGroupRole(role)) {
    return (
      <Badge
        variant="secondary"
        className={cn("h-5 gap-1 text-[10px] bg-muted text-muted-foreground border-0", className)}
        data-testid={testId}
      >
        —
      </Badge>
    )
  }
  const Icon = ROLE_ICON[role]
  return (
    <Badge
      variant="secondary"
      className={cn("h-5 gap-1 text-[10px]", ROLE_BADGE_CLASS[role], className)}
      data-testid={testId ?? `role-badge-${role}`}
    >
      <Icon className="size-3" aria-hidden="true" />
      {t(`members:roles.${role}`)}
    </Badge>
  )
}
