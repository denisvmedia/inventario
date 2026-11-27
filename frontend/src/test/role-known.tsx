import type { ReactNode } from "react"

import { useMyGroupRole } from "@/features/group/roles"

// WhenRoleKnown renders its children once the caller's role in the current
// group is known (features/group/roles.ts). It needs AuthProvider and
// GroupProvider above it and a members list for the signed-in user
// (memberHandlers.asRole). A page offers a write action only for a role the
// server accepts, and offers none while the role is unknown, so a test that
// asserts an action is absent would pass before the members list arrives.
// Mounting the page after it does makes the absence mean the role was read,
// and lets tests that click a write action find it without waiting.
export function WhenRoleKnown({ children }: { children: ReactNode }) {
  return useMyGroupRole() ? <>{children}</> : null
}
