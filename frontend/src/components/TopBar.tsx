import { Separator } from "@/components/ui/separator"
import { SidebarTrigger } from "@/components/ui/sidebar"
import { CommandPaletteProvider } from "@/components/CommandPaletteProvider"
import { GroupRoleCluster } from "@/components/GroupRoleCluster"
import { HeaderSearch } from "@/components/HeaderSearch"
import { ModeToggle } from "@/components/ModeToggle"
import { DensityToggle } from "@/components/DensityToggle"
import { useCurrentRouteTitle } from "@/components/routing/RouteTitle"

// TopBar is the sticky chrome at the top of every authenticated page. It
// shows the sidebar toggle, the active page title (sourced from
// RouteTitleContext — RouteTitle inside each page broadcasts the
// translated title), and on the right the search entry that opens the
// command palette, the group switcher with the caller's role
// (GroupRoleCluster), then the density and mode toggles.
//
// The TopBar is on every authenticated page, so it also mounts the command
// palette: CommandPaletteProvider renders the palette dialog and binds
// Cmd/Ctrl+K, and the search entry opens it through the same state.
export function TopBar() {
  const title = useCurrentRouteTitle()
  return (
    <CommandPaletteProvider>
      <header className="flex h-12 items-center gap-2 border-b border-border px-4 sticky top-0 bg-background z-10">
        <SidebarTrigger className="-ml-1" />
        <Separator orientation="vertical" className="h-4" />
        <div className="flex items-center gap-2 text-sm min-w-0">
          <span className="font-medium truncate" data-testid="topbar-title">
            {title}
          </span>
        </div>
        <div className="ml-auto flex items-center gap-2">
          <HeaderSearch />
          <GroupRoleCluster />
          <DensityToggle />
          <ModeToggle />
        </div>
      </header>
    </CommandPaletteProvider>
  )
}
