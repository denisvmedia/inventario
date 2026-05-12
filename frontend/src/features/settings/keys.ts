// Query keys for the per-user settings (`/settings`) endpoint. The
// resource is keyed per-(tenant_id, user_id) on the BE but the URL
// rewrites under the active group's slug — so we keep the slug in the
// cache key to avoid leaking the previous group's row when the user
// switches.
export const settingsKeys = {
  all: ["settings"] as const,
  preferences: (slug: string) => [...settingsKeys.all, "preferences", slug] as const,
}
