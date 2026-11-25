// TanStack Query keys for server drafts. Scoped by group slug because the
// http client rewrites /drafts -> /g/{slug}/drafts.
export const draftKeys = {
  all: ["drafts"] as const,
  group: (slug: string) => [...draftKeys.all, slug] as const,
  detail: (slug: string, id: string) => [...draftKeys.group(slug), "detail", id] as const,
  files: (slug: string, id: string) => [...draftKeys.group(slug), "files", id] as const,
}
