// TanStack Query keys for the custom fields slice. Scoped by group slug,
// because the http client rewrites /custom-fields to /g/{slug}/custom-fields.
export const customFieldKeys = {
  all: ["customField"] as const,
  group: (slug: string) => [...customFieldKeys.all, slug] as const,
  list: (slug: string) => [...customFieldKeys.group(slug), "list"] as const,
}
