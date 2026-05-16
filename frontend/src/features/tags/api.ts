// Pure data-layer functions for the tags feature slice. Hooks live in
// `./hooks.ts`. Backed by the `/tags` surface introduced under #1400 +
// extended for #1412 with `?include=usage` (per-row meta) and
// `/tags/stats` (group-wide adoption summary).
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type TagEntity = Schema<"models.Tag">
export type TagColor = Schema<"models.TagColor">
export type TagUsage = Schema<"registry.TagUsage">
export type TagStats = Schema<"registry.TagStats">

// `as const` keeps the literal-union type so downstream callers (e.g.
// `z.enum(TAG_COLORS)`) infer `"amber"|"green"|…` rather than the
// widened `string`. The `satisfies readonly TagColor[]` check ensures
// we cannot accidentally drift away from the BE's models.TagColor enum
// without a typecheck error.
export const TAG_COLORS = [
  "amber",
  "green",
  "blue",
  "orange",
  "red",
  "muted",
] as const satisfies readonly TagColor[]

export type TagSortField = "label" | "created_at" | "usage"
export type TagSortOrder = "asc" | "desc"

// TagScope narrows tag listing / autocomplete to tags actually used on a
// specific entity type. Wire contract: bare "commodity" / "file" tokens
// (singular) match the BE; "all" is the in-FE label for "no filter" and
// is omitted from the request URL by the data layer.
export type TagScope = "commodity" | "file"

export interface ListTagsOptions {
  page?: number
  perPage?: number
  search?: string
  sort?: TagSortField
  order?: TagSortOrder
  includeUsage?: boolean
  scope?: TagScope
  signal?: AbortSignal
}

// Per-row usage as it lands in the list response when ?include=usage is set.
// Optional because the BE only attaches it when the include token is present.
export interface ListedTag {
  tag: TagEntity & { id: string }
  usage?: TagUsage
}

// List envelope: tags are FLAT inside `data` (project convention), with
// an optional inline `meta.usage` block per row when ?include=usage was
// requested. Mirrors the FilesListEnvelope shape.
interface TagsListEnvelope {
  data: Array<TagEntity & { id: string; meta?: { usage?: TagUsage } }>
  meta?: {
    tags?: number
    total?: number
  }
}

interface TagDetailEnvelope {
  id?: string
  type?: string
  attributes?: TagEntity
  meta?: { usage?: TagUsage }
}

interface TagStatsEnvelope {
  data: TagStats
}

export async function listTags(
  options: ListTagsOptions = {}
): Promise<{ tags: ListedTag[]; total: number }> {
  const params = new URLSearchParams()
  if (options.page !== undefined) params.set("page", String(options.page))
  if (options.perPage !== undefined) params.set("per_page", String(options.perPage))
  if (options.search?.trim()) params.set("q", options.search.trim())
  if (options.sort) params.set("sort", options.sort)
  if (options.order) params.set("order", options.order)
  if (options.includeUsage) params.set("include", "usage")
  if (options.scope) params.set("scope", options.scope)
  const qs = params.toString()
  const path = qs ? `/tags?${qs}` : "/tags"
  const body = await http.get<TagsListEnvelope>(path, { signal: options.signal })
  return {
    tags: (body.data ?? []).map((row) => {
      const { meta, ...rest } = row
      return { tag: rest, usage: meta?.usage }
    }),
    total: body.meta?.total ?? body.data?.length ?? 0,
  }
}

export async function getTagStats(signal?: AbortSignal): Promise<TagStats> {
  const body = await http.get<TagStatsEnvelope>("/tags/stats", { signal })
  return body.data
}

export async function getTag(
  id: string,
  signal?: AbortSignal
): Promise<{ tag: TagEntity & { id: string }; usage?: TagUsage }> {
  const body = await http.get<TagDetailEnvelope>(`/tags/${encodeURIComponent(id)}`, { signal })
  if (!body.attributes) {
    throw new Error(`Malformed /tags/${id} response: missing attributes`)
  }
  return { tag: { ...body.attributes, id: body.id ?? id }, usage: body.meta?.usage }
}

export interface CreateTagRequest {
  slug: string
  label: string
  color: TagColor
}

export async function createTag(req: CreateTagRequest): Promise<TagEntity & { id: string }> {
  const body = await http.post<TagDetailEnvelope>("/tags", {
    data: { type: "tags", attributes: req },
  })
  if (!body.attributes) {
    throw new Error("Malformed POST /tags response: missing attributes")
  }
  return { ...body.attributes, id: body.id ?? "" }
}

export interface UpdateTagRequest {
  slug?: string
  label?: string
  color?: TagColor
}

export async function updateTag(
  id: string,
  req: UpdateTagRequest
): Promise<TagEntity & { id: string }> {
  const body = await http.patch<TagDetailEnvelope>(`/tags/${encodeURIComponent(id)}`, {
    data: { id, type: "tags", attributes: req },
  })
  if (!body.attributes) {
    throw new Error(`Malformed PATCH /tags/${id} response: missing attributes`)
  }
  return { ...body.attributes, id: body.id ?? id }
}

// deleteTag returns 409 with usage breakdown when force=false and the
// tag is in use. The Tags page surfaces that error, then re-issues with
// force=true after user confirmation. We don't unwrap the 409 body here
// because the http client throws Error(message) on 409 and the page
// reads the message directly.
export async function deleteTag(id: string, force = false): Promise<void> {
  const path = force
    ? `/tags/${encodeURIComponent(id)}?force=true`
    : `/tags/${encodeURIComponent(id)}`
  await http.del<void>(path)
}

export interface TagAutocompleteEntry {
  id: string
  slug: string
  label: string
  color: TagColor
}

interface TagAutocompleteEnvelope {
  data?: TagAutocompleteEntry[]
}

export async function autocompleteTags(
  q: string,
  limit = 10,
  options: { scope?: TagScope; signal?: AbortSignal } = {}
): Promise<TagAutocompleteEntry[]> {
  const params = new URLSearchParams()
  if (q.trim()) params.set("q", q.trim())
  params.set("limit", String(limit))
  if (options.scope) params.set("scope", options.scope)
  const body = await http.get<TagAutocompleteEnvelope>(`/tags/autocomplete?${params.toString()}`, {
    signal: options.signal,
  })
  return body.data ?? []
}
