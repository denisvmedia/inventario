import type { ReactNode } from "react"

import { useCurrentGroup } from "@/features/group/GroupContext"

// WhenGroupLoaded renders its children once GroupProvider knows the current
// group. In the app a group-scoped tab mounts only on an item page, and that
// page loads after the group does. A test that mounts the tab next to a
// GroupProvider whose /groups request is still running lets the tab's query
// run first under an empty slug and then again under the real one, so what
// the test finds depends on which response arrives first. Wrapping the tab in
// this keeps the order the app has.
export function WhenGroupLoaded({ children }: { children: ReactNode }) {
  const { currentGroup } = useCurrentGroup()
  return currentGroup ? <>{children}</> : null
}
