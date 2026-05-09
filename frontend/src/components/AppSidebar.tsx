import {
  ChevronsUpDown,
  FolderOpen,
  HandCoins,
  HardDriveDownload,
  LayoutDashboard,
  LogOut,
  MapPin,
  Package,
  Plus,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  Tag,
  User,
  Users,
  Wrench,
  type LucideIcon,
} from "lucide-react"
import { Link, NavLink, useMatch, useNavigate } from "react-router-dom"
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
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import { withGroupQuery } from "@/lib/group-aware-url"
import { useNavLabel } from "@/lib/nav-labels"
import { useAuth } from "@/features/auth/AuthContext"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useGroupMigrationLock } from "@/features/currency-migration/lock"
import type { LocationGroup } from "@/features/group/api"

interface NavEntry {
  // Full translation key (namespace-qualified, e.g. "common:nav.dashboard").
  // Resolved via the useNavLabel hook so each cell-arm is a literal t() call
  // the i18next-cli extractor can verify against the catalog. See
  // src/lib/nav-labels.ts for the matching switch.
  labelKey: string
  // Path resolver — receives the active group (null when the user has none
  // yet). Inventory/Manage entries return null without a group; Personal
  // entries always resolve, but pin the active group via ?g=<slug> so the
  // sidebar keeps its group-aware nav after the user clicks Profile.
  to: (group: LocationGroup | null) => string | null
  icon: LucideIcon
}

// Three sidebar groups mirror the legacy Vue + design-mock layout:
//   Inventory — daily-use group-scoped pages
//   Manage    — group-admin pages
//   Personal  — user-scoped pages (no group needed)
//
// Inventory + Manage entries hide entirely when there's no active group:
// rendering "/g/" links with no slug would 404. Personal entries always
// render — they're reachable for zero-group users too — and just append
// `?g=<slug>` when there IS a group, so navigating into them doesn't
// drop the user out of their group context.
const INVENTORY: NavEntry[] = [
  {
    labelKey: "common:nav.dashboard",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}` : null),
    icon: LayoutDashboard,
  },
  {
    labelKey: "common:nav.locations",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/locations` : null),
    icon: MapPin,
  },
  {
    labelKey: "common:nav.items",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/commodities` : null),
    icon: Package,
  },
  {
    labelKey: "common:nav.warranties",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/warranties` : null),
    icon: ShieldCheck,
  },
  {
    labelKey: "common:nav.lent",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/lent` : null),
    icon: HandCoins,
  },
  {
    labelKey: "common:nav.inService",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/in-service` : null),
    icon: Wrench,
  },
]

const MANAGE: NavEntry[] = [
  {
    labelKey: "common:nav.tags",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/tags` : null),
    icon: Tag,
  },
  {
    labelKey: "common:nav.files",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/files` : null),
    icon: FolderOpen,
  },
  {
    labelKey: "common:nav.members",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/members` : null),
    icon: Users,
  },
  {
    labelKey: "common:nav.backup",
    to: (g) => (g?.slug ? `/g/${encodeURIComponent(g.slug)}/exports` : null),
    icon: HardDriveDownload,
  },
  {
    // Settings is the real GroupSettingsPage at /groups/:id/settings —
    // identity, currency, danger zone. Path uses :id (no ?g=) because
    // GroupContext resolves the active group via the id alone.
    labelKey: "common:nav.system",
    to: (g) => (g?.id ? `/groups/${encodeURIComponent(g.id)}/settings` : null),
    icon: Settings,
  },
]

// Personal section: Profile + Preferences. Path is group-exempt (mounted
// at /profile and /settings, not under /g/:slug/*) so a zero-group user
// can reach them, but for users with ≥1 groups we append ?g=<slug> so
// navigating in keeps the rest of the sidebar populated.
const PERSONAL: NavEntry[] = [
  { labelKey: "common:nav.profile", to: (g) => withGroupQuery("/profile", g?.slug), icon: User },
  {
    labelKey: "common:nav.preferences",
    to: (g) => withGroupQuery("/settings", g?.slug),
    icon: SlidersHorizontal,
  },
]

interface NavRowProps {
  entry: NavEntry
  group: LocationGroup | null
  onNavigate: () => void
}

// A "section root" is a top-level entry whose target has subroutes that
// should keep the row highlighted (e.g. `/g/:slug/locations` should stay
// active on `/g/:slug/locations/new`, `/g/:slug/locations/:id`, etc.).
// The Dashboard target `/g/:slug` is NOT a section root — keeping it
// active on every group-scoped subroute would highlight Dashboard for
// the entire app. We detect a section route as "/g/<slug>/<segment>"
// (3+ segments after the host).
function isGroupSectionRoute(target: string): boolean {
  const segments = target.split("/").filter(Boolean)
  return segments[0] === "g" && segments.length >= 3
}

function NavRow({ entry, group, onNavigate }: NavRowProps) {
  const target = entry.to(group)
  // useMatch matches against pathname only (it ignores query strings), so
  // the ?g=<slug> suffix on Personal entries doesn't disturb the pattern.
  // Strip it anyway for the section-root detection — `isGroupSectionRoute`
  // expects a clean path.
  const targetPath = target ? target.split("?")[0]! : null
  // useMatch must be called unconditionally (hooks rules). For a section
  // root we match the prefix (`/g/:slug/locations/*`) so subroutes count
  // as the same section; for everything else we match the exact path.
  const matchPattern = targetPath
    ? isGroupSectionRoute(targetPath)
      ? `${targetPath}/*`
      : targetPath
    : "__never_match__"
  const match = useMatch(matchPattern)
  const label = useNavLabel(entry.labelKey)
  if (!target || !targetPath) return null
  const Icon = entry.icon
  const isActive = !!match
  // NavLink's `end` makes the link active only on an exact path match.
  // For section roots we want the inverse — subroutes belong to the same
  // section, so drop `end` and let prefix matching highlight the row.
  const navLinkEnd = !isGroupSectionRoute(targetPath)
  return (
    <SidebarMenuItem>
      {/* `isActive` flows into SidebarMenuButton's `data-active` attribute,
          which the shadcn primitive's CSS selectors hang off (`data-[active=true]`).
          asChild forwards the data attribute to the underlying NavLink so the
          highlighted styles actually fire. */}
      <SidebarMenuButton asChild tooltip={label} isActive={isActive}>
        <NavLink to={target} end={navLinkEnd} onClick={onNavigate}>
          <Icon className="size-4" />
          <span>{label}</span>
        </NavLink>
      </SidebarMenuButton>
    </SidebarMenuItem>
  )
}

export function AppSidebar() {
  const { isMobile, setOpenMobile, state } = useSidebar()
  const { user, logout } = useAuth()
  const { currentGroup } = useCurrentGroup()
  const migrationLock = useGroupMigrationLock()
  const navigate = useNavigate()
  const { t } = useTranslation()

  function closeMobileSidebar() {
    if (isMobile) setOpenMobile(false)
  }

  // Add-item entry-point. Mirrors the design-mock AppSidebar: a primary
  // button under the group switcher that drills into the commodity-create
  // dialog via the /commodities/new side-effect route. Hidden when there
  // is no active group (the destination needs a slug) and disabled while
  // a currency migration locks writes for the active group.
  const addItemHref = currentGroup?.slug
    ? `/g/${encodeURIComponent(currentGroup.slug)}/commodities/new`
    : null
  const addItemLabel = t("commodities:list.addItem")

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
        {addItemHref ? (
          <div className="px-2 pb-2 group-data-[collapsible=icon]:px-0">
            {/* Render as a real <button> (not Button asChild + Link) so
                the cursor matches the design-mock AppSidebar exactly:
                Tailwind v4 preflight drops `cursor: pointer` from
                buttons, so anchors (which keep the browser default)
                would visually diverge here. We navigate imperatively
                in onClick. aria-disabled (not disabled) keeps the
                title tooltip reachable during a migration lock —
                ui/Button's `disabled:pointer-events-none` would
                otherwise swallow it; aria-label keeps the control
                accessible in icon-only collapsed mode where the
                text span is hidden. */}
            <Button
              size="sm"
              data-testid="sidebar-add-item"
              aria-label={addItemLabel}
              aria-disabled={migrationLock.locked || undefined}
              title={migrationLock.locked ? t("errors:lockedDuringMigration") : undefined}
              onClick={() => {
                if (migrationLock.locked) return
                closeMobileSidebar()
                navigate(addItemHref)
              }}
              className={cn(
                "w-full justify-start gap-2 group-data-[collapsible=icon]:size-8 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:p-0",
                "aria-disabled:cursor-not-allowed aria-disabled:opacity-50"
              )}
            >
              <Plus aria-hidden="true" className="size-4 shrink-0" />
              <span className="group-data-[collapsible=icon]:hidden">{addItemLabel}</span>
            </Button>
          </div>
        ) : null}
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
                  group={currentGroup}
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
                  group={currentGroup}
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
                  group={currentGroup}
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
                  data-testid="user-menu"
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
              <DropdownMenuContent
                side="top"
                align="start"
                sideOffset={8}
                className="user-dropdown w-60"
              >
                {user?.email ? (
                  <DropdownMenuLabel className="font-normal text-muted-foreground text-xs px-2 py-1.5">
                    {user.email}
                  </DropdownMenuLabel>
                ) : null}
                <DropdownMenuSeparator />
                <DropdownMenuItem asChild className="gap-2 dropdown-item--profile">
                  {/* Render the Profile entry as an anchor so test selectors
                      like `.user-dropdown a:has-text("Profile")` resolve to a
                      real link; SPA navigation goes through react-router's
                      <Link>. */}
                  <Link
                    to={withGroupQuery("/profile", currentGroup?.slug)}
                    onClick={() => {
                      closeMobileSidebar()
                    }}
                  >
                    <User className="size-4 text-muted-foreground" />
                    {t("common:nav.profile")}
                  </Link>
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="gap-2 text-destructive focus:text-destructive dropdown-item--logout"
                  onSelect={async () => {
                    closeMobileSidebar()
                    await logout()
                  }}
                  data-testid="sign-out"
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
