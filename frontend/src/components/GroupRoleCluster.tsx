import { useEffect, useMemo, useRef, useState } from "react"
import { Check, ChevronsUpDown } from "lucide-react"
import { useNavigate, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useOptionalAuth } from "@/features/auth/AuthContext"
import { useUpdateProfile } from "@/features/auth/hooks"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { useMembers } from "@/features/group/hooks"
import { cn } from "@/lib/utils"

// GroupRoleCluster lives in the TopBar header. It pairs the active-group
// switcher trigger (`.group-selector__trigger`) with the caller's role badge
// (`[data-testid="current-role"]`) inside a single `.group-role-cluster` flex
// row. Switching groups (a) navigates to /g/<new-slug>/ and (b) fires the
// debounced PUT /auth/me with `default_group_id` so the choice survives a
// fresh login on a new device — both halves of the #1262 / #1300 contract.
export function GroupRoleCluster() {
  const { t } = useTranslation()
  const groupCtx = useOptionalCurrentGroup()
  const groups = groupCtx?.groups
  const currentGroup = groupCtx?.currentGroup
  const params = useParams<{ groupSlug?: string }>()
  const navigate = useNavigate()
  const auth = useOptionalAuth()
  const user = auth?.user
  const updateProfile = useUpdateProfile()
  const [open, setOpen] = useState(false)

  const activeSlug = currentGroup?.slug ?? params.groupSlug ?? null

  const membersQuery = useMembers(currentGroup?.id)
  const role = useMemo(() => {
    if (!user?.id || !membersQuery.data) return null
    const me = membersQuery.data.find((m) => m.member_user_id === user.id)
    return (me?.role ?? null) as "admin" | "user" | null
  }, [membersQuery.data, user?.id])

  // Debounced PUT /auth/me — see #1262 / #1300. We hold the latest target
  // group id in a ref so cascading clicks within the debounce window
  // collapse into a single network call carrying the final selection.
  const pendingDefaultGroupId = useRef<string | null>(null)
  const debounceHandle = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(
    () => () => {
      if (debounceHandle.current) clearTimeout(debounceHandle.current)
    },
    []
  )

  function persistDefaultGroup(groupId: string) {
    pendingDefaultGroupId.current = groupId
    if (debounceHandle.current) clearTimeout(debounceHandle.current)
    debounceHandle.current = setTimeout(() => {
      const next = pendingDefaultGroupId.current
      pendingDefaultGroupId.current = null
      debounceHandle.current = null
      if (!next) return
      // The BE accepts both `name` and `default_group_id` on PUT /auth/me;
      // pass the current name verbatim so we mutate only the group preference.
      updateProfile.mutate({ name: user?.name ?? "", default_group_id: next })
    }, 400)
  }

  function handleSwitch(group: { id?: string; slug?: string } | undefined) {
    if (!group?.slug) return
    setOpen(false)
    if (group.slug === activeSlug) return
    navigate(`/g/${encodeURIComponent(group.slug)}`)
    if (group.id) persistDefaultGroup(group.id)
  }

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
              onSelect={() => handleSwitch(group)}
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
      {role ? (
        <span
          data-testid="current-role"
          className={cn(
            "role-indicator inline-flex h-7 items-center rounded-md border px-2 text-xs font-medium leading-none",
            "border-border bg-muted/40 text-muted-foreground",
            role === "admin" ? "role-indicator--admin" : "role-indicator--user"
          )}
        >
          {role}
        </span>
      ) : null}
    </div>
  )
}
