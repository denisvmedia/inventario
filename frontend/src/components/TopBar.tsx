import { Separator } from "@/components/ui/separator"
import { SidebarTrigger } from "@/components/ui/sidebar"
import { GroupRoleCluster } from "@/components/GroupRoleCluster"
import { ModeToggle } from "@/components/ModeToggle"
import { DensityToggle } from "@/components/DensityToggle"
import { useCurrentRouteTitle } from "@/components/routing/RouteTitle"

// TopBar is the sticky chrome at the top of every authenticated page. It
// shows the sidebar toggle, the active page title (sourced from
// RouteTitleContext — RouteTitle inside each page broadcasts the
// translated title), and the mode + density toggles on the right. The
// onboarding-tour and group selector live in the sidebar; we keep this
// bar minimal so per-feature pages have room for their own breadcrumbs
// later if needed.
export function TopBar() {
  const title = useCurrentRouteTitle()
  return (
    <header className="flex h-12 items-center gap-2 border-b border-border px-4 sticky top-0 bg-background z-10">
      <SidebarTrigger className="-ml-1" />
      <Separator orientation="vertical" className="h-4" />
      <div className="flex items-center gap-2 text-sm min-w-0">
        <span className="font-medium truncate" data-testid="topbar-title">
          {title}
        </span>
      </div>
      <div className="ml-auto flex items-center gap-2">
        <GroupRoleCluster />
        <DensityToggle />
        <ModeToggle />
      </div>
    </header>
  )
}
