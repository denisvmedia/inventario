// TanStack Query keys for the currency-migration slice. Scoped by group
// slug because the http client rewrites /currency-migrations →
// /g/{slug}/currency-migrations; without the slug in the key, switching
// between groups would reuse the wrong cache.
export const currencyMigrationKeys = {
  all: ["currency-migration"] as const,
  group: (slug: string) => [...currencyMigrationKeys.all, slug] as const,
  list: (slug: string) => [...currencyMigrationKeys.group(slug), "list"] as const,
  detail: (slug: string, id: string) =>
    [...currencyMigrationKeys.group(slug), "detail", id] as const,
}
