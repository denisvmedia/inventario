import { Link, Outlet, useLocation } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { LogOut, ShieldCheck, User } from "lucide-react"

import { Button } from "@/components/ui/button"
import { ConfirmProvider } from "@/hooks/useConfirm"
import { ImpersonationBanner } from "@/components/ImpersonationBanner"
import { ImpersonationProvider } from "@/features/admin/impersonation/ImpersonationContext"
import { RouteTitleProvider } from "@/components/routing/RouteTitle"
import { Toaster } from "@/components/ui/sonner"
import { useBackofficeAuth } from "@/features/backoffice/auth/context"
import { useBackofficeLogout } from "@/features/backoffice/auth/hooks"
import { hardRedirect } from "@/lib/navigation"

// AdminShell is the chrome for every back-office (/admin/* and
// /backoffice/me) page (#1785 Phase 6). Replaces the tenant Shell for
// this subtree — a back-office operator is NOT required to also have a
// tenant session, so the tenant ProtectedRoute / GroupProvider / AppSidebar
// chain doesn't apply here. The chrome surfaces:
//   - the operator identity (top-right) + sign-out
//   - the impersonation banner (an active session is itself a back-office
//     concern: the operator initiated it; the banner ends it)
//   - a quiet "PLATFORM" chip so a glance tells the operator they are
//     on the back-office side of the app
//
// Page-level chrome (the Admin → <section> breadcrumb + the secondary nav
// pills) is owned by AdminLayout under <Outlet />; this shell handles only
// the top-level frame.
export function AdminShell() {
  const { user } = useBackofficeAuth()
  const logoutMutation = useBackofficeLogout()
  const { t } = useTranslation("backoffice")
  const location = useLocation()

  async function handleLogout() {
    try {
      await logoutMutation.mutateAsync()
    } finally {
      // Hard redirect tears down every cached query, including admin
      // queries that would otherwise re-fire with the now-empty
      // token and surface a 401 toast on the way out.
      hardRedirect("/backoffice/login")
    }
  }

  const displayName = user?.name?.trim() || user?.email?.split("@")[0] || ""

  return (
    <RouteTitleProvider>
      <ConfirmProvider>
        <ImpersonationProvider>
          <div className="flex min-h-svh flex-col bg-background">
            <ImpersonationBanner />
            <header
              className="flex h-14 items-center justify-between border-b border-border bg-card px-4"
              data-testid="admin-shell-top-bar"
            >
              <div className="flex items-center gap-3">
                <Link
                  to="/admin/tenants"
                  className="flex items-center gap-2 font-semibold text-foreground"
                >
                  <div className="flex size-7 items-center justify-center rounded-md bg-slate-950">
                    <ShieldCheck className="size-4 text-white" />
                  </div>
                  <span>{t("shell.brand")}</span>
                </Link>
                <span className="inline-flex items-center gap-1.5 rounded-full border border-amber-400/40 bg-amber-50 px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wider text-amber-700 dark:bg-amber-950 dark:text-amber-200">
                  <span className="block size-1.5 rounded-full bg-amber-500" aria-hidden="true" />
                  {t("shell.planeChip")}
                </span>
              </div>
              <div className="flex items-center gap-2">
                <Link
                  to="/backoffice/me"
                  className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
                  data-testid="admin-shell-operator"
                  aria-current={location.pathname === "/backoffice/me" ? "page" : undefined}
                >
                  <User className="size-4" aria-hidden="true" />
                  <span className="hidden sm:inline">{displayName || t("shell.operator")}</span>
                </Link>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleLogout}
                  disabled={logoutMutation.isPending}
                  data-testid="admin-shell-logout"
                >
                  <LogOut className="size-4" aria-hidden="true" />
                  <span className="sr-only">{t("shell.signOut")}</span>
                </Button>
              </div>
            </header>
            <main className="flex-1 overflow-y-auto">
              <div className="container mx-auto p-6">
                <Outlet />
              </div>
            </main>
            <Toaster />
          </div>
        </ImpersonationProvider>
      </ConfirmProvider>
    </RouteTitleProvider>
  )
}
