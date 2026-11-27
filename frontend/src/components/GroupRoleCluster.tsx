import { useState } from "react"
import { Check, ChevronsUpDown } from "lucide-react"
import { useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { RoleBadge } from "@/components/RoleBadge"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useOptionalAuth } from "@/features/auth/AuthContext"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { useMembers } from "@/features/group/hooks"
import { useSwitchGroup } from "@/features/group/useSwitchGroup"
import { cn } from "@/lib/utils"

// GroupRoleCluster lives in the TopBar header. It pairs the active-group
// switcher trigger (`.group-selector__trigger`) with the caller's role badge
// (`[data-testid="current-role"]`) inside a single `.group-role-cluster` flex
// row. The mock has no group control in its header; the cluster is a logged
// deviation (#1258). Switching goes through useSwitchGroup, the handler the
// sidebar GroupSelector uses too, and the badge is the shared RoleBadge.
export function GroupRoleCluster() {
  const { t } = useTranslation()
  const groupCtx = useOptionalCurrentGroup()
  const groups = groupCtx?.groups
  const currentGroup = groupCtx?.currentGroup
  const params = useParams<{ groupSlug?: string }>()
  const auth = useOptionalAuth()
  const user = auth?.user
  const switchGroup = useSwitchGroup()
  const [open, setOpen] = useState(false)

  const activeSlug = currentGroup?.slug ?? params.groupSlug ?? null

  // The React Compiler memoizes this derived value; members lists are small.
  const membersQuery = useMembers(currentGroup?.id)
  const role =
    user?.id && membersQuery.data
      ? (membersQuery.data.find((m) => m.member_user_id === user.id)?.role ?? null)
      : null

  if (!groups || groups.length === 0) return null

  return (
    <div className="group-role-cluster flex items-center gap-1.5">
      <DropdownMenu open={open} onOpenChange={setOpen}>
        <DropdownMenuTrigger asChild>
          <button
            type="button"
            aria-label={t("common:shell.switchGroup")}
            className={cn(
              "group-selector__trigger inline-flex h-7 max-w-[14rem] items-center gap-1.5 rounded-md border",
              "border-border bg-background px-2 text-xs font-medium leading-none",
              "transition-colors hover:bg-accent hover:text-accent-foreground",
              "focus-visible:outline-hidden focus-visible:ring-[3px] focus-visible:ring-ring/50",
              "data-[state=open]:bg-accent"
            )}
          >
            <span className="group-selector__name truncate">
              {currentGroup?.name ?? t("common:shell.noActiveGroup")}
            </span>
            <ChevronsUpDown className="size-3 shrink-0 text-muted-foreground" aria-hidden="true" />
          </button>
        </DropdownMenuTrigger>
        <DropdownMenuContent className="min-w-[14rem]" align="start" side="bottom" sideOffset={4}>
          {groups.map((group) => (
            <DropdownMenuItem
              key={group.id}
              onSelect={() => {
                setOpen(false)
                switchGroup(group)
              }}
              className="group-selector__item gap-2"
            >
              <span className="flex-1 truncate">{group.name}</span>
              {group.slug === activeSlug ? (
                <Check className="size-3.5 text-primary shrink-0" aria-hidden="true" />
              ) : null}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
      {/* The badge takes the trigger's height, radius and font size so the
          pair reads as one unit (#1258). */}
      {role ? (
        <RoleBadge
          role={role}
          testId="current-role"
          className={`role-indicator role-indicator--${role} h-7 rounded-md text-xs`}
        />
      ) : null}
    </div>
  )
}
