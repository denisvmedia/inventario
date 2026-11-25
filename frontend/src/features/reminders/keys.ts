// TanStack Query keys for item reminders and the Upcoming list (#2805).
// Scoped by group slug because the http client rewrites /reminders and
// /upcoming to /g/{slug}/...; without the slug in the key, switching groups
// would reuse the wrong cache.
export const reminderKeys = {
  all: ["reminders"] as const,
  group: (slug: string) => [...reminderKeys.all, slug] as const,
  byCommodity: (slug: string, commodityID: string) =>
    [...reminderKeys.group(slug), "byCommodity", commodityID] as const,
  upcoming: (slug: string, days: number) =>
    [...reminderKeys.group(slug), "upcoming", days] as const,
}
