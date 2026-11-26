import type { IntakeListOptions } from "./api"

function listKeySuffix(opts: IntakeListOptions | undefined): string {
  if (!opts) return ""
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  return params.toString()
}

// TanStack Query keys for batch intake (#2810). Scoped by group slug because
// the http client rewrites /intake -> /g/{slug}/intake.
export const intakeKeys = {
  all: ["intake"] as const,
  group: (slug: string) => [...intakeKeys.all, slug] as const,
  limits: (slug: string) => [...intakeKeys.group(slug), "limits"] as const,
  list: (slug: string, opts?: IntakeListOptions) =>
    [...intakeKeys.group(slug), "list", listKeySuffix(opts)] as const,
  detail: (slug: string, id: string) => [...intakeKeys.group(slug), "detail", id] as const,
  // The mutation key of a product review, so the review page can wait for
  // the reviews in flight before it creates the drafts.
  review: (slug: string) => [...intakeKeys.group(slug), "review"] as const,
}
