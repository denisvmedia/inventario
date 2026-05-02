import { Outlet } from "react-router-dom"

import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar"
import { AppSidebar } from "@/components/AppSidebar"
import { CommandPalette } from "@/components/CommandPalette"
import { InviteBanner } from "@/components/InviteBanner"
import { TopBar } from "@/components/TopBar"
import { Toaster } from "@/components/ui/sonner"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { RouteTitleProvider } from "@/components/routing/RouteTitle"

// Shell is the layout component for every authenticated, non-fullscreen
// page. Mount as a layout route in router.tsx — children render inside
// SidebarInset via <Outlet />.
//
//   <SidebarProvider> — shadcn sidebar state (collapsed/expanded, mobile)
//   ├─ <AppSidebar />
//   └─ <SidebarInset>
//        <TopBar />
//        <InviteBanner />  ← driven by data; #1413 wires the count
//        <main>
//          <Outlet />        ← the actual page (Dashboard, Locations, ...)
//        </main>
//      </SidebarInset>
//
// ConfirmProvider, Toaster, and the global Cmd+K palette mount here so
// every authenticated page has them without duplicating providers per
// route.
//
// RouteTitleProvider sits at the shell root so the TopBar can read the
// translated title that RouteTitle (inside each page) broadcasts.
export function Shell() {
  return (
    <RouteTitleProvider>
      <ConfirmProvider>
        <SidebarProvider>
          <AppSidebar />
          <SidebarInset>
            <TopBar />
            {/* count=0 today — once the invites query lands (#1413) it will
                read from the user's pending-invites list. */}
            <InviteBanner count={0} />
            <main className="flex-1 overflow-y-auto">
              <div className="container mx-auto p-6">
                <Outlet />
              </div>
            </main>
          </SidebarInset>
          <CommandPalette />
          <Toaster />
        </SidebarProvider>
      </ConfirmProvider>
    </RouteTitleProvider>
  )
}
