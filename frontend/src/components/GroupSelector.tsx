import { useState } from "react"
import { Building2, Check, ChevronsUpDown, Plus, Settings } from "lucide-react"
import { useNavigate, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"

import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { SidebarMenu, SidebarMenuButton, SidebarMenuItem } from "@/components/ui/sidebar"
import { useCurrentGroup } from "@/features/group/GroupContext"

// GroupSelector shows the currently-active LocationGroup in the sidebar
// header and lets the user switch groups. Switching navigates to
// /g/{newSlug}/, which is the URL-as-source-of-truth contract from #1404 —
// the GroupProvider's slug-mirror effect feeds the http rewrite from
// there, and two browser tabs at different groups stay isolated.
//
// "Create new group" routes to /groups/new, the existing onboarding/empty
// state. Persisting the choice as the user's default_group_id (#1263) is
// out of scope for the shell PR; that lands when the Settings page (#1414)
// or the legacy debounced PUT /auth/me is ported.
export function GroupSelector() {
  const { groups, currentGroup } = useCurrentGroup()
  const navigate = useNavigate()
  const params = useParams<{ groupSlug?: string }>()
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  const activeSlug = currentGroup?.slug ?? params.groupSlug ?? null

  function handleSwitch(slug: string | undefined) {
    setOpen(false)
    if (!slug) return
    if (slug === activeSlug) return
    navigate(`/g/${encodeURIComponent(slug)}`)
  }

  // Absent groups data → render nothing rather than a flicker. The shell's
  // ProtectedRoute keeps the user on the boot fallback while groups load
  // for the first time, so this branch is rarely hit in practice.
  if (!groups) return null

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        <DropdownMenu open={open} onOpenChange={setOpen}>
          <DropdownMenuTrigger asChild>
            <SidebarMenuButton
              size="lg"
              aria-label={t("common:shell.switchGroup")}
              className="group-selector data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
            >
              <div className="flex size-8 items-center justify-center rounded-lg bg-primary text-primary-foreground shrink-0">
                <Building2 className="size-4" />
              </div>
              <div className="flex flex-col gap-0.5 leading-none min-w-0">
                <span className="font-semibold text-sm truncate">
                  {currentGroup?.name ?? t("common:shell.noActiveGroup")}
                </span>
                <span className="text-xs text-muted-foreground truncate">
                  {t("common:shell.groupCount", { count: groups.length })}
                </span>
              </div>
              <ChevronsUpDown className="ml-auto size-4 shrink-0 text-muted-foreground" />
            </SidebarMenuButton>
          </DropdownMenuTrigger>
          <DropdownMenuContent
            className="w-[--radix-dropdown-menu-trigger-width] min-w-56"
            align="start"
            side="bottom"
            sideOffset={4}
          >
            {groups.map((group) => (
              <DropdownMenuItem
                key={group.id}
                onSelect={() => handleSwitch(group.slug)}
                className="gap-2 p-2"
              >
                <div className="flex size-6 items-center justify-center rounded-md bg-primary/10 shrink-0">
                  <Building2 className="size-3.5 text-primary" />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium truncate">{group.name}</p>
                </div>
                {group.slug === activeSlug ? (
                  <Check className="size-4 text-primary shrink-0" />
                ) : null}
              </DropdownMenuItem>
            ))}
            <DropdownMenuSeparator />
            {currentGroup?.id ? (
              <DropdownMenuItem
                className="gap-2 p-2 text-muted-foreground"
                data-testid="group-selector-settings"
                onSelect={() => {
                  setOpen(false)
                  navigate(`/groups/${encodeURIComponent(currentGroup.id!)}/settings`)
                }}
              >
                <div className="flex size-6 items-center justify-center rounded-md border border-dashed border-border">
                  <Settings className="size-3.5" />
                </div>
                <span className="text-sm">{t("common:shell.groupSettings")}</span>
              </DropdownMenuItem>
            ) : null}
            <DropdownMenuItem
              className="gap-2 p-2 text-muted-foreground"
              onSelect={() => {
                setOpen(false)
                navigate("/groups/new")
              }}
            >
              <div className="flex size-6 items-center justify-center rounded-md border border-dashed border-border">
                <Plus className="size-3.5" />
              </div>
              <span className="text-sm">{t("common:shell.createGroup")}</span>
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
