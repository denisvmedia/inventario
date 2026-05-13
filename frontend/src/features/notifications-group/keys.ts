// Query keys for the per-group notification prefs slice (#1648).
// Keyed by group slug because the URL itself is `/g/{slug}/...`; a
// slug change means a different (user × group) and a clean cache miss.
export const groupNotificationsKeys = {
  all: ["group-notifications"] as const,
  group: (groupSlug: string) => [...groupNotificationsKeys.all, "group", groupSlug] as const,
}
