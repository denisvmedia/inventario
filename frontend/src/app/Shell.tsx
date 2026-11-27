import { useEffect, useMemo } from "react"
import { Outlet } from "react-router-dom"

import { SidebarInset, SidebarProvider } from "@/components/ui/sidebar"
import { AppSidebar } from "@/components/AppSidebar"
import { CommitBadge } from "@/components/CommitBadge"
import { CurrencyMigrationBanner } from "@/components/CurrencyMigrationBanner"
import { ImpersonationBanner } from "@/components/ImpersonationBanner"
import { InviteBanner } from "@/components/InviteBanner"
import { OnboardingTour } from "@/components/OnboardingTour"
import { TopBar } from "@/components/TopBar"
import { Toaster } from "@/components/ui/sonner"
import { useAuth } from "@/features/auth/AuthContext"
import { ImpersonationProvider } from "@/features/admin/impersonation/ImpersonationContext"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useMyGroupRole } from "@/features/group/roles"
import { LanguageSync } from "@/features/settings/LanguageSync"
import { NumberFormatLocaleSync } from "@/features/settings/NumberFormatLocaleSync"
import { KeyboardShortcutsProvider } from "@/features/shortcuts"
import { useIsMobile } from "@/hooks/use-mobile"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { useOnboardingTour } from "@/hooks/useOnboardingTour"
import {
  OnboardingTourControlsProvider,
  TOUR_BLOCKER_GRACE_MS,
  type OnboardingTourControls,
  type TourBlocker,
} from "@/hooks/useOnboardingTourControls"
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
// ConfirmProvider and Toaster mount here so every authenticated page has
// them without duplicating providers per route. The global command palette
// mounts with the TopBar, whose search entry opens it as well as
// Cmd/Ctrl+K.
//
// RouteTitleProvider sits at the shell root so the TopBar can read the
// translated title that RouteTitle (inside each page) broadcasts.
export function Shell() {
  const { user } = useAuth()
  const { currentGroup } = useCurrentGroup()
  const isMobile = useIsMobile()
  // OnboardingTour state lives at the shell level so AppSidebar can
  // re-launch it from the user menu, Settings → Help can do the same
  // through OnboardingTourControlsProvider, and the overlay mounts once
  // (#1543 / design-audit #1527). useOnboardingTour persists the "seen"
  // flag in localStorage keyed by user.id.
  //
  // Every step but the welcome card points at the sidebar, so the tour
  // needs an active group (the group's entries render only with one) and
  // a screen wide enough for the sidebar to be on it (below the mobile
  // breakpoint it is a closed sheet). Without either it neither
  // auto-launches nor offers a restart (#2787).
  //
  // The tour picks its steps from the sidebar when it opens, and the Add
  // item entry is there only for a role that adds items, so the first
  // launch waits until the role is known.
  const hasGroup = Boolean(currentGroup?.slug)
  const roleKnown = useMyGroupRole() !== null
  const tour = useOnboardingTour(user?.id ?? null, {
    hasGroup,
    autoLaunch: !isMobile && roleKnown,
  })
  const tourBlocker: TourBlocker | null = !hasGroup ? "noGroup" : isMobile ? "narrowScreen" : null
  const tourControls = useMemo<OnboardingTourControls>(
    () => ({ restart: tour.restart, blocker: tourBlocker }),
    [tour.restart, tourBlocker]
  )
  // A blocker can appear while the tour is open: the window narrows below
  // the mobile breakpoint and the sidebar turns into a closed sheet, so the
  // highlights would point at elements that are no longer on screen. The
  // tour hides at once, and closes without being marked seen only when the
  // blocker lasts TOUR_BLOCKER_GRACE_MS; it can run again once it fits.
  //
  // The grace period is there because the window can cross the breakpoint
  // for a moment only: Chromium shrinks the page to 1x1 while it takes a
  // full-page screenshot, and a window dragged across the breakpoint and
  // back passes it twice. When the blocker goes away in time, the tour
  // comes back on the step it was on.
  const { isOpen: tourIsOpen, close: closeTour } = tour
  useEffect(() => {
    if (!tourIsOpen || !tourBlocker) return
    const timer = window.setTimeout(closeTour, TOUR_BLOCKER_GRACE_MS)
    return () => window.clearTimeout(timer)
  }, [tourIsOpen, tourBlocker, closeTour])

  return (
    <RouteTitleProvider>
      <ConfirmProvider>
        <NumberFormatLocaleSync />
        <LanguageSync />
        <KeyboardShortcutsProvider>
          {/* ImpersonationProvider tracks the active admin-impersonation
              session (#1752) so the ImpersonationBanner below can render
              whenever a session is in progress, regardless of route. */}
          <ImpersonationProvider>
            <OnboardingTourControlsProvider value={tourControls}>
              <SidebarProvider>
                {/* The user-menu entry is left out while the tour cannot
                    start; Settings → Help shows its row disabled with the
                    reason. */}
                <AppSidebar onRestartTour={tourBlocker ? undefined : tour.restart} />
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
                <Toaster />
                {/* Faint build-commit watermark, bottom-right, hidden on
                    mobile; renders nothing in dev/tests (#1972). */}
                <CommitBadge />
                {/* Stays mounted while hidden, so the steps it resolved
                    from the sidebar survive a moment without the sidebar. */}
                {tour.isOpen ? (
                  <OnboardingTour
                    hidden={tourBlocker !== null}
                    step={tour.step}
                    onNext={tour.next}
                    onPrev={tour.prev}
                    onFinish={tour.finish}
                    onSkip={tour.skip}
                    onClose={tour.close}
                  />
                ) : null}
              </SidebarProvider>
            </OnboardingTourControlsProvider>
          </ImpersonationProvider>
        </KeyboardShortcutsProvider>
      </ConfirmProvider>
    </RouteTitleProvider>
  )
}
