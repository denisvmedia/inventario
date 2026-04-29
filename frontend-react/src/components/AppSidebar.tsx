import {
  ChevronsUpDown,
  FolderOpen,
  HardDriveDownload,
  LayoutDashboard,
  LogOut,
  MapPin,
  Package,
  Settings,
  ShieldCheck,
  Tag,
  User,
  Users,
  type LucideIcon,
} from "lucide-react"
import { NavLink, useMatch, useNavigate, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"

import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarRail,
  useSidebar,
} from "@/components/ui/sidebar"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { AppLogo } from "@/components/AppLogo"
import { GroupSelector } from "@/components/GroupSelector"
import { cn } from "@/lib/utils"
import { useAuth } from "@/features/auth/AuthContext"

interface NavEntry {
  // Translation key under common:nav for the visible label.
  labelKey: string
  // Path resolver — group-scoped entries take a slug, exempt entries are
  // static. Returning null skips rendering (handled per-group below).
  to: (groupSlug: string | null) => string | null
  icon: LucideIcon
}

// Three sidebar groups mirror the legacy Vue + design-mock layout:
//   Inventory — daily-use group-scoped pages
//   Manage    — group-admin / system pages
//   Personal  — user-scoped pages (no group needed)
//
// Each entry's `to` is keyed off the active group slug so collapsing into a
// non-group route (logout, no-group state) hides the inventory/manage rows
// rather than rendering broken links.
const INVENTORY: NavEntry[] = [
  {
    labelKey: "nav.dashboard",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}` : null),
    icon: LayoutDashboard,
  },
  {
    labelKey: "nav.locations",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/locations` : null),
    icon: MapPin,
  },
  {
    labelKey: "nav.items",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/commodities` : null),
    icon: Package,
  },
  {
    labelKey: "nav.warranties",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/warranties` : null),
    icon: ShieldCheck,
  },
]

const MANAGE: NavEntry[] = [
  {
    labelKey: "nav.tags",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/tags` : null),
    icon: Tag,
  },
  {
    labelKey: "nav.files",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/files` : null),
    icon: FolderOpen,
  },
  {
    labelKey: "nav.members",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/members` : null),
    icon: Users,
  },
  {
    labelKey: "nav.backup",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/backup` : null),
    icon: HardDriveDownload,
  },
  {
    labelKey: "nav.system",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/system` : null),
    icon: Settings,
  },
]

// Personal section currently has just Profile; the legacy "Preferences"
// row pointed at /profile too (same destination as Profile), which made it
// impossible to reach a distinct preferences screen from the sidebar. The
// real preferences UI lands with the Settings page (#1414); the entry is
// re-added there with its own route.
const PERSONAL: NavEntry[] = [{ labelKey: "nav.profile", to: () => "/profile", icon: User }]

interface NavRowProps {
  entry: NavEntry
  groupSlug: string | null
  onNavigate: () => void
}

function NavRow({ entry, groupSlug, onNavigate }: NavRowProps) {
  const { t } = useTranslation()
  const target = entry.to(groupSlug)
  // useMatch must be called unconditionally (hooks rules) — pass an empty
  // pattern when the entry doesn't render so the call exists for every
  // render order without affecting routing.
  const match = useMatch(target ?? "__never_match__")
  if (!target) return null
  const Icon = entry.icon
  const label = t(`common:${entry.labelKey}`)
  const isActive = !!match
  return (
    <SidebarMenuItem>
      {/* `isActive` flows into SidebarMenuButton's `data-active` attribute,
          which the shadcn primitive's CSS selectors hang off (`data-[active=true]`).
          asChild forwards the data attribute to the underlying NavLink so the
          highlighted styles actually fire. */}
      <SidebarMenuButton asChild tooltip={label} isActive={isActive}>
        <NavLink to={target} end onClick={onNavigate}>
          <Icon className="size-4" />
          <span>{label}</span>
        </NavLink>
      </SidebarMenuButton>
    </SidebarMenuItem>
  )
}

export function AppSidebar() {
  const { isMobile, setOpenMobile, state } = useSidebar()
  const params = useParams<{ groupSlug?: string }>()
  const navigate = useNavigate()
  const { user, logout } = useAuth()
  const { t } = useTranslation()
  const groupSlug = params.groupSlug ?? null

  function closeMobileSidebar() {
    if (isMobile) setOpenMobile(false)
  }

  // The bottom-of-sidebar user button. Initials fall back to the email's
  // first two characters when the user's display name isn't set yet.
  const displayName = user?.name?.trim() || user?.email?.split("@")[0] || ""
  const initials = displayName
    .split(/\s+/)
    .map((s) => s.charAt(0))
    .join("")
    .slice(0, 2)
    .toUpperCase()

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader className="border-b border-sidebar-border">
        <div
          className={cn(
            "flex h-10 items-center",
            isMobile || state === "expanded" ? "px-3" : "justify-center px-0"
          )}
        >
          <NavLink
            to="/"
            onClick={closeMobileSidebar}
            className="rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <AppLogo className={cn(state === "collapsed" && !isMobile && "[&_span]:hidden")} />
          </NavLink>
        </div>
        <div className="px-2 group-data-[collapsible=icon]:px-0">
          <GroupSelector />
        </div>
      </SidebarHeader>

      <SidebarContent className="pt-2 group-data-[collapsible=icon]:!overflow-y-auto">
        <SidebarGroup>
          <SidebarGroupLabel>{t("common:nav.groupInventory")}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {INVENTORY.map((e) => (
                <NavRow
                  key={e.labelKey}
                  entry={e}
                  groupSlug={groupSlug}
                  onNavigate={closeMobileSidebar}
                />
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        <SidebarGroup>
          <SidebarGroupLabel>{t("common:nav.groupManage")}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {MANAGE.map((e) => (
                <NavRow
                  key={e.labelKey}
                  entry={e}
                  groupSlug={groupSlug}
                  onNavigate={closeMobileSidebar}
                />
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        <SidebarGroup>
          <SidebarGroupLabel>{t("common:nav.groupPersonal")}</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {PERSONAL.map((e) => (
                <NavRow
                  key={e.labelKey}
                  entry={e}
                  groupSlug={groupSlug}
                  onNavigate={closeMobileSidebar}
                />
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>

      <SidebarFooter className="border-t border-sidebar-border">
        <SidebarMenu>
          <SidebarMenuItem>
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <SidebarMenuButton
                  size="lg"
                  tooltip={displayName || t("common:shell.account")}
                  className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
                >
                  <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-sidebar-primary text-sidebar-primary-foreground text-xs font-semibold">
                    {initials || "?"}
                  </div>
                  <div className="flex flex-col gap-0.5 leading-none min-w-0">
                    <span className="text-sm font-semibold truncate">
                      {displayName || t("common:shell.account")}
                    </span>
                    {user?.email ? (
                      <span className="text-xs text-muted-foreground truncate">{user.email}</span>
                    ) : null}
                  </div>
                  <ChevronsUpDown className="ml-auto size-4 shrink-0 text-muted-foreground" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="top" align="start" sideOffset={8} className="w-60">
                {user?.email ? (
                  <DropdownMenuLabel className="font-normal text-muted-foreground text-xs px-2 py-1.5">
                    {user.email}
                  </DropdownMenuLabel>
                ) : null}
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="gap-2"
                  onSelect={() => {
                    closeMobileSidebar()
                    navigate("/profile")
                  }}
                >
                  <User className="size-4 text-muted-foreground" />
                  {t("common:nav.profile")}
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="gap-2 text-destructive focus:text-destructive"
                  onSelect={async () => {
                    closeMobileSidebar()
                    await logout()
                  }}
                >
                  <LogOut className="size-4" />
                  {t("common:shell.signOut")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </SidebarMenuItem>
        </SidebarMenu>
      </SidebarFooter>
      <SidebarRail />
    </Sidebar>
  )
}
