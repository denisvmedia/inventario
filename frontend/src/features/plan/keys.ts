// Query keys for the subscription-plan slice. Plan + usage are keyed by
// group slug because the URL rewriter resolves `/plan` to
// `/g/{slug}/plan`; switching the active group must invalidate the cache
// to avoid showing one group's chips inside another's settings page.
export const planKeys = {
  all: ["plan"] as const,
  group: (groupSlug: string) => [...planKeys.all, "group", groupSlug] as const,
}
