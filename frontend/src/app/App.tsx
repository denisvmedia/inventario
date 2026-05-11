import { AppRoutes } from "@/app/router"
import { RootErrorBoundary } from "@/components/RootErrorBoundary"

// Top-level app surface. The Shell layout (mounted by router.tsx for the
// authenticated subtree) owns its own page-fill and overflow rules, so
// here we only set the base background + text color tokens. Public pages
// (login, register, etc.) render outside Shell and can opt into a
// full-screen layout per page.
//
// `RootErrorBoundary` wraps the route tree so a render-time crash
// surfaces our `UnexpectedErrorPage` instead of a white screen. It sits
// inside `<Providers>` (so the page can use i18n / theme) but outside
// the routes (so any route's failure is caught).
export function App() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <RootErrorBoundary>
        <AppRoutes />
      </RootErrorBoundary>
    </div>
  )
}
