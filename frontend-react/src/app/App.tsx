import { AppRoutes } from "@/app/router"

// Top-level app surface. The Shell layout (mounted by router.tsx for the
// authenticated subtree) owns its own page-fill and overflow rules, so
// here we only set the base background + text color tokens. Public pages
// (login, register, etc.) render outside Shell and can opt into a
// full-screen layout per page.
export function App() {
  return (
    <div className="min-h-screen bg-background text-foreground">
      <AppRoutes />
    </div>
  )
}
