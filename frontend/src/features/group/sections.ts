// Sections a group admin can hide for every member of the group (#1607):
// Warranties, Maintenance and repairs (with In service), Loans and Upcoming.
// The group carries the list as `hidden_sections`; components ask
// `useSectionVisible(section)` instead of reading it themselves.
//
// A hidden section leaves the navigation, the dashboard, the item form and
// the item's actions. An item that already holds data for it still shows
// that data, so callers that render an item's own data combine the answer
// with what the item holds.
import { useOptionalCurrentGroup } from "./GroupContext"
import type { GroupSection, LocationGroup } from "./api"

export type { GroupSection }

// The order the group settings page lists the sections in. It matches the
// order the server stores them in.
export const GROUP_SECTIONS: readonly GroupSection[] = [
  "warranties",
  "maintenance",
  "loans",
  "upcoming",
] as const

export function isSectionHidden(
  group: Pick<LocationGroup, "hidden_sections"> | null | undefined,
  section: GroupSection
): boolean {
  return group?.hidden_sections?.includes(section) ?? false
}

// useSectionVisible reports whether the active group shows the section.
// Without an active group (no provider, groups still loading, a page
// outside any group) every section is visible: the setting belongs to a
// group, and nothing hides until one is known.
export function useSectionVisible(section: GroupSection): boolean {
  const ctx = useOptionalCurrentGroup()
  return !isSectionHidden(ctx?.currentGroup, section)
}
