import { Outlet } from "react-router-dom"

import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar"
import { AppSidebar } from "@/components/AppSidebar"
import { CommandPalette } from "@/components/CommandPalette"
import { CommitBadge } from "@/components/CommitBadge"
import { CurrencyMigrationBanner } from "@/components/CurrencyMigrationBanner"
import { ImpersonationBanner } from "@/components/ImpersonationBanner"
import { InviteBanner } from "@/components/InviteBanner"
import { OnboardingTour, TOUR_STEPS } from "@/components/OnboardingTour"
import { TopBar } from "@/components/TopBar"
import { Toaster } from "@/components/ui/sonner"
import { useAuth } from "@/features/auth/AuthContext"
import { ImpersonationProvider } from "@/features/admin/impersonation/ImpersonationContext"
import { NumberFormatLocaleSync } from "@/features/settings/NumberFormatLocaleSync"
import { KeyboardShortcutsProvider } from "@/features/shortcuts"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { useOnboardingTour } from "@/hooks/useOnboardingTour"
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
  const { user } = useAuth()
  // OnboardingTour state lives at the shell level so AppSidebar can
  // re-launch it from the user menu and SidebarInset can host the
  // overlay (#1543 / design-audit #1527). Auto-launches once per user
  // on first authenticated render — useOnboardingTour persists the
  // "seen" flag in localStorage keyed by user.id.
  const tour = useOnboardingTour(user?.id ?? null)

  return (
    <RouteTitleProvider>
      <ConfirmProvider>
        <NumberFormatLocaleSync />
        <KeyboardShortcutsProvider>
          {/* ImpersonationProvider tracks the active admin-impersonation
              session (#1752) so the ImpersonationBanner below can render
              whenever a session is in progress, regardless of route. */}
          <ImpersonationProvider>
            <SidebarProvider>
              <AppSidebar onRestartTour={tour.restart} />
              <SidebarInset>
                {/* Persistent impersonation banner — sits above the
                    TopBar; renders only when a session is active. */}
                <ImpersonationBanner />
                <TopBar />
                <CurrencyMigrationBanner />
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
              {/* Faint build-commit watermark, bottom-right, hidden on
                  mobile; renders nothing in dev/tests (#1972). */}
              <CommitBadge />
              {tour.isOpen ? (
                <OnboardingTour
                  step={tour.step}
                  totalSteps={TOUR_STEPS.length}
                  onNext={tour.next}
                  onPrev={tour.prev}
                  onFinish={tour.finish}
                  onSkip={tour.skip}
                />
              ) : null}
            </SidebarProvider>
          </ImpersonationProvider>
        </KeyboardShortcutsProvider>
      </ConfirmProvider>
    </RouteTitleProvider>
  )
}
