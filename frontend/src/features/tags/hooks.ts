import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useCallback, useMemo } from "react"

import { commodityKeys } from "@/features/commodities/keys"
import { fileKeys } from "@/features/files/keys"
import { useCurrentGroup, useOptionalCurrentGroup } from "@/features/group/GroupContext"

import {
  autocompleteTags,
  createTag,
  deleteTag,
  getTag,
  getTagStats,
  listAllTags,
  listTags,
  updateTag,
  type CreateTagRequest,
  type ListTagsOptions,
  type ListedTag,
  type TagAutocompleteEntry,
  type TagColor,
  type TagEntity,
  type TagKind,
  type TagStats,
  type TagUsage,
  type UpdateTagRequest,
} from "./api"
import { tagKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

export function useTags(opts: ListTagsOptions, query: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const enabled = query.enabled ?? true
  return useQuery<{ tags: ListedTag[]; total: number }>({
    queryKey: tagKeys.list(slug, opts),
    queryFn: ({ signal }) => listTags({ ...opts, signal }),
    enabled,
    placeholderData: (prev) => prev,
  })
}

export function useTagStats({ enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<TagStats>({
    queryKey: tagKeys.stats(slug),
    queryFn: ({ signal }) => getTagStats(signal),
    enabled,
    placeholderData: (prev) => prev,
  })
}

export function useTag(id: string | undefined, { enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<{ tag: TagEntity & { id: string }; usage?: TagUsage }>({
    queryKey: tagKeys.detail(slug, id ?? ""),
    queryFn: ({ signal }) => {
      if (!id) throw new Error("useTag called without an id")
      return getTag(id, signal)
    },
    enabled: enabled && !!id,
  })
}

// all wipes the entire tags namespace for the active group. Used after
// every mutation because list queries are keyed by sort + order + search +
// include flags, and a focused invalidation would miss cached permutations
// the user can switch back to.
//
// tagged also wipes the item and file namespaces (lists, details, tag
// counts): renaming a tag's slug or deleting it changes the tags those
// records carry.
function useInvalidate() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const all = () => qc.invalidateQueries({ queryKey: tagKeys.group(slug) })
  return {
    all,
    tagged: () =>
      Promise.all([
        all(),
        qc.invalidateQueries({ queryKey: commodityKeys.group(slug) }),
        qc.invalidateQueries({ queryKey: fileKeys.group(slug) }),
      ]),
  }
}

export function useCreateTag() {
  const invalidate = useInvalidate()
  return useMutation<TagEntity & { id: string }, Error, CreateTagRequest>({
    mutationFn: (req) => createTag(req),
    onSuccess: () => invalidate.all(),
  })
}

interface UpdateTagVars {
  id: string
  req: UpdateTagRequest
}

export function useUpdateTag() {
  const invalidate = useInvalidate()
  return useMutation<TagEntity & { id: string }, Error, UpdateTagVars>({
    mutationFn: ({ id, req }) => updateTag(id, req),
    onSuccess: () => invalidate.tagged(),
  })
}

interface DeleteTagVars {
  id: string
  force?: boolean
}

export function useDeleteTag() {
  const invalidate = useInvalidate()
  return useMutation<void, Error, DeleteTagVars>({
    mutationFn: ({ id, force }) => deleteTag(id, force),
    onSuccess: () => invalidate.tagged(),
  })
}

// useTagAutocomplete uses the *optional* group hook so it can be
// rendered from surfaces that aren't wrapped in <GroupProvider>
// (e.g. the CommodityFormDialog's per-file TagsInput inside the
// dialog test harness, which intentionally skips the provider).
// When the slug is unknown we just don't enable the query — there's
// no group to scope autocomplete results to anyway.
//
// kind selects which entity's tags to suggest (item-tags vs file-tags) —
// required, since the two are separate entities and the BE 422s without it.
// When kind is absent the query stays disabled (no request fired).
interface AutocompleteOptions extends QueryOptions {
  kind?: TagKind
}

export function useTagAutocomplete(q: string, limit = 10, opts: AutocompleteOptions = {}) {
  const ctx = useOptionalCurrentGroup()
  const slug = ctx?.currentGroup?.slug ?? ""
  const enabled = opts.enabled ?? true
  const kind = opts.kind
  return useQuery<TagAutocompleteEntry[]>({
    queryKey: tagKeys.autocomplete(slug, q, limit, kind),
    queryFn: ({ signal }) => {
      // Defensive: `enabled` already gates on kind, but a manual refetch()
      // bypasses it — never issue a malformed `kind=undefined` request.
      if (!kind) return Promise.resolve([])
      return autocompleteTags(q, limit, { kind, signal })
    },
    enabled: enabled && slug.length > 0 && kind !== undefined,
    // Keep the previous result visible while the next query (next
    // keystroke / different prefix) is in flight. Without this, `data`
    // briefly returns `undefined` between queries, the consumer's
    // dropdown empties, and the popover flicker-closes-then-reopens.
    placeholderData: (prev) => prev,
  })
}

// useTagCatalog lists every tag of a kind in the active group: the tag
// palette offers them, useTagIndex and useTagLabel resolve slugs against
// them. All share one cached query. Outside a group the query stays off and
// `data` is undefined.
//
// The key sits under the group's tag namespace, so tag mutations and the
// commodity / file writes that auto-create tags refresh it.
export function useTagCatalog(kind: TagKind) {
  const ctx = useOptionalCurrentGroup()
  const groupSlug = ctx?.currentGroup?.slug ?? ""
  return useQuery({
    queryKey: tagKeys.labels(groupSlug, kind),
    queryFn: ({ signal }) => listAllTags(kind, signal),
    enabled: groupSlug.length > 0,
    placeholderData: (prev) => prev,
  })
}

// A tag as a slug stored on an item or file resolves to: the label the user
// typed or gave it on the Tags page, and its color.
export interface ResolvedTag {
  slug: string
  label: string
  color: TagColor
}

export interface TagIndex {
  // Every tag of the kind in the current group, as listed by the API.
  tags: ResolvedTag[]
  // Resolves a slug; a slug without a tag row comes back as itself in
  // the muted color.
  resolve: (slug: string) => ResolvedTag
  isLoading: boolean
}

// useTagIndex resolves slugs against the group's tags of one kind. It falls
// back to the slug while the catalogue loads, outside a group, and for a slug
// with no tag row. Uses the optional group hook so a component rendered
// outside <GroupProvider> (a dialog test harness) just gets the slug back.
export function useTagIndex(kind: TagKind): TagIndex {
  const ctx = useOptionalCurrentGroup()
  const groupSlug = ctx?.currentGroup?.slug ?? ""
  const { data, isLoading } = useTagCatalog(kind)
  // A Map, not an object literal: a slug such as "constructor" must not
  // resolve to an Object.prototype member.
  const { tags, bySlug } = useMemo(() => {
    const list: ResolvedTag[] = []
    const map = new Map<string, ResolvedTag>()
    for (const tag of data ?? []) {
      if (!tag.slug) continue
      const resolved: ResolvedTag = {
        slug: tag.slug,
        label: tag.label || tag.slug,
        color: (tag.color ?? "muted") as TagColor,
      }
      list.push(resolved)
      map.set(tag.slug, resolved)
    }
    return { tags: list, bySlug: map }
  }, [data])
  const resolve = useCallback(
    (slug: string): ResolvedTag => bySlug.get(slug) ?? { slug, label: slug, color: "muted" },
    [bySlug]
  )
  return { tags, resolve, isLoading: isLoading && groupSlug.length > 0 }
}

// useTagLabel is useTagIndex for callers that only show the label.
export function useTagLabel(kind: TagKind): (slug: string) => string {
  const { resolve } = useTagIndex(kind)
  return useCallback((slug: string) => resolve(slug).label, [resolve])
}
