import {
  LayoutDashboard,
  Package,
  ShieldCheck,
  Tag,
  Settings,
  Plus,
  FolderOpen,
  MapPin,
  User,
  Users,
  HardDriveDownload,
  SearchX,
  Layers,
  LayoutGrid,
  Wrench,
  Palette,
  LogOut,
  SlidersHorizontal,
  ChevronsUpDown,
  Building2,
} from "lucide-react"
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
import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { cn } from "@/lib/utils"
import { LocationGroupSwitcher } from "@/components/LocationGroupSwitcher"
import { AppLogo } from "@/components/AppLogo"

interface AppSidebarProps {
  activeView: string
  onNavigate: (view: string) => void
  onAddItem: () => void
  activeGroupId: string
  onGroupChange: (groupId: string) => void
}

const INVENTORY_ITEMS = [
  { id: "dashboard", label: "Dashboard", icon: LayoutDashboard, tour: "nav-dashboard" },
  { id: "locations", label: "Locations", icon: MapPin, tour: "nav-locations" },
  { id: "items", label: "All Items", icon: Package, tour: "nav-items" },
  { id: "warranties", label: "Warranties", icon: ShieldCheck, tour: "nav-warranties" },
]

const MANAGE_ITEMS = [
  { id: "tags", label: "Tags", icon: Tag, tour: undefined },
  { id: "files", label: "Files", icon: FolderOpen, tour: "nav-files" },
  { id: "members", label: "Members", icon: Users, tour: undefined },
  { id: "backup", label: "Backup", icon: HardDriveDownload, tour: undefined },
  { id: "group-settings", label: "Settings", icon: Settings, tour: undefined },
]

const PERSONAL_ITEMS = [
  { id: "profile", label: "Profile", icon: User },
  { id: "settings", label: "Preferences", icon: Settings },
]

const ADMIN_ITEMS = [
  { id: "admin-tenants", label: "Tenants", icon: Building2 },
  { id: "admin-groups", label: "Groups", icon: Layers },
]

const STATE_ITEMS = [
  { id: "state-404", label: "404 Not Found", icon: SearchX },
  { id: "state-no-group", label: "No Location Group", icon: Layers },
  { id: "state-no-location", label: "No Location", icon: MapPin },
  { id: "state-no-area", label: "No Area", icon: LayoutGrid },
  { id: "state-maintenance", label: "Maintenance", icon: Wrench },
  { id: "ui-showcase", label: "UI Showcase", icon: Palette },
]

export function AppSidebar({ activeView, onNavigate, onAddItem, activeGroupId, onGroupChange }: AppSidebarProps) {
  const { isMobile, setOpenMobile, state } = useSidebar()

  function handleNavigate(view: string) {
    if (isMobile) setOpenMobile(false)
    onNavigate(view)
  }

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader className="border-b border-sidebar-border">
        {/* Brand row */}
        <div className={cn(
          "flex h-10 items-center",
          isMobile || state === "expanded" ? "px-3" : "justify-center px-0"
        )}>
          <button
            type="button"
            onClick={() => handleNavigate("dashboard")}
            className="rounded-md focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <AppLogo className={cn(state === "collapsed" && !isMobile && "[&_span]:hidden")} />
          </button>
        </div>
        {/* Group switcher — always rendered, text hidden by sidebar in collapsed mode */}
        <div className="px-2 group-data-[collapsible=icon]:px-0">
          <LocationGroupSwitcher activeGroupId={activeGroupId} onGroupChange={onGroupChange} />
        </div>
        {/* Add item — fixed in header */}
        <div className="px-2 pb-2 group-data-[collapsible=icon]:px-0">
          <Button
            data-tour="add-item"
            size="sm"
            className="w-full justify-start gap-2 group-data-[collapsible=icon]:size-8 group-data-[collapsible=icon]:justify-center group-data-[collapsible=icon]:p-0"
            onClick={() => { if (isMobile) setOpenMobile(false); onAddItem() }}
          >
            <Plus className="size-4 shrink-0" />
            <span className="group-data-[collapsible=icon]:hidden">Add item</span>
          </Button>
        </div>
      </SidebarHeader>

      <SidebarContent className="pt-2 group-data-[collapsible=icon]:!overflow-y-auto">
        <SidebarGroup>
          <SidebarGroupLabel>Inventory</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {INVENTORY_ITEMS.map((item) => (
                <SidebarMenuItem key={item.id}>
                  <SidebarMenuButton
                    isActive={activeView === item.id}
                    tooltip={item.label}
                    onClick={() => handleNavigate(item.id)}
                    data-tour={item.tour}
                  >
                    <item.icon className="size-4" />
                    <span>{item.label}</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        <SidebarGroup>
          <SidebarGroupLabel>Manage</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {MANAGE_ITEMS.map((item) => (
                <SidebarMenuItem key={item.id}>
                  <SidebarMenuButton
                    isActive={activeView === item.id}
                    tooltip={item.label}
                    onClick={() => handleNavigate(item.id)}
                    data-tour={item.tour}
                  >
                    <item.icon className="size-4" />
                    <span>{item.label}</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        <SidebarGroup>
          <SidebarGroupLabel>Personal</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {PERSONAL_ITEMS.map((item) => (
                <SidebarMenuItem key={item.id}>
                  <SidebarMenuButton
                    isActive={activeView === item.id}
                    tooltip={item.label}
                    onClick={() => handleNavigate(item.id)}
                  >
                    <item.icon className="size-4" />
                    <span>{item.label}</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        <SidebarGroup>
          <SidebarGroupLabel>Admin</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {ADMIN_ITEMS.map((item) => (
                <SidebarMenuItem key={item.id}>
                  <SidebarMenuButton
                    isActive={
                      activeView === item.id ||
                      (item.id === "admin-tenants" &&
                        (activeView === "admin-tenant-detail" || activeView === "admin-user-detail")) ||
                      (item.id === "admin-groups" && activeView === "admin-group-detail")
                    }
                    tooltip={item.label}
                    onClick={() => handleNavigate(item.id)}
                  >
                    <item.icon className="size-4" />
                    <span>{item.label}</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
              ))}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>

        <SidebarGroup>
          <SidebarGroupLabel>States</SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              {STATE_ITEMS.map((item) => (
                <SidebarMenuItem key={item.id}>
                  <SidebarMenuButton
                    isActive={activeView === item.id}
                    tooltip={item.label}
                    onClick={() => handleNavigate(item.id)}
                  >
                    <item.icon className="size-4" />
                    <span>{item.label}</span>
                  </SidebarMenuButton>
                </SidebarMenuItem>
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
                  tooltip="Alex Johnson"
                  className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
                >
                  <div className="flex size-8 shrink-0 items-center justify-center rounded-full bg-sidebar-primary text-sidebar-primary-foreground text-xs font-semibold">
                    AJ
                  </div>
                  <div className="flex flex-col gap-0.5 leading-none min-w-0">
                    <span className="text-sm font-semibold truncate">Alex Johnson</span>
                    <span className="text-xs text-muted-foreground truncate">alex@example.com</span>
                  </div>
                  <ChevronsUpDown className="ml-auto size-4 shrink-0 text-muted-foreground" />
                </SidebarMenuButton>
              </DropdownMenuTrigger>
              <DropdownMenuContent side="top" align="start" sideOffset={8} className="w-60">
                <DropdownMenuLabel className="font-normal text-muted-foreground text-xs px-2 py-1.5">
                  alex@example.com
                </DropdownMenuLabel>
                <DropdownMenuSeparator />
                <DropdownMenuItem className="gap-2" onSelect={() => handleNavigate("profile")}>
                  <User className="size-4 text-muted-foreground" />
                  Profile
                </DropdownMenuItem>
                <DropdownMenuItem className="gap-2" onSelect={() => handleNavigate("settings")}>
                  <SlidersHorizontal className="size-4 text-muted-foreground" />
                  Preferences
                </DropdownMenuItem>
                <DropdownMenuSeparator />
                <DropdownMenuItem
                  className="gap-2 text-destructive focus:text-destructive"
                  onSelect={() => handleNavigate("auth")}
                >
                  <LogOut className="size-4" />
                  Sign out
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