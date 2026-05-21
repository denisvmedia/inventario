import type { ReactNode } from "react"

import { useAuth } from "@/features/auth/AuthContext"
import { useIsSystemAdmin } from "@/features/auth/hooks"
import { AdminForbiddenPage } from "@/pages/admin/AdminForbiddenPage"

interface RequireSystemAdminProps {
  children: ReactNode
  // Rendered while the boot probe is still resolving who the user is.
  // Defaults to nothing so the guard renders empty for a few hundred ms
  // rather than flashing the 403 page at an admin mid-probe.
  fallback?: ReactNode
}

// Gates the /admin/* subtree on the `is_system_admin` flag. Unlike
// ProtectedRoute (which redirects unauthenticated users to /login), a
// signed-in but non-admin user is NOT redirected — they see an in-place
// 403-style page so a hand-typed /admin URL fails loudly and explicably
// rather than silently bouncing somewhere else.
//
// ProtectedRoute already sits above this in the route tree, so by the
// time RequireSystemAdmin renders the user is authenticated; the only
// open question is whether they carry the admin flag.
export function RequireSystemAdmin({ children, fallback = null }: RequireSystemAdminProps) {
  const { isInitialized, user } = useAuth()
  const isSystemAdmin = useIsSystemAdmin()

  // Hold the fallback until the auth probe settles — otherwise an admin
  // would briefly see the 403 page before `user` resolves.
  if (!isInitialized || user === undefined) return <>{fallback}</>
  if (!isSystemAdmin) return <AdminForbiddenPage />
  return <>{children}</>
}
