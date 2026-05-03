import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import {
  autocompleteTags,
  createTag,
  deleteTag,
  getTag,
  getTagStats,
  listTags,
  updateTag,
  type CreateTagRequest,
  type ListTagsOptions,
  type ListedTag,
  type TagAutocompleteEntry,
  type TagEntity,
  type TagStats,
  type TagUsage,
  type UpdateTagRequest,
} from "./api"
import { tagKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

export function useTags(opts: ListTagsOptions = {}, query: QueryOptions = {}) {
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

// invalidateAll wipes the entire tags namespace for the active group.
// Used after every mutation because list queries are keyed by sort +
// order + search + include flags, and a focused invalidation would miss
// cached permutations the user can switch back to.
function useInvalidate() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return {
    all: () => qc.invalidateQueries({ queryKey: tagKeys.group(slug) }),
    detail: (id: string) => qc.invalidateQueries({ queryKey: tagKeys.detail(slug, id) }),
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
    onSuccess: () => invalidate.all(),
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
    onSuccess: () => invalidate.all(),
  })
}

export function useTagAutocomplete(q: string, limit = 10, { enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<TagAutocompleteEntry[]>({
    queryKey: tagKeys.autocomplete(slug, q, limit),
    queryFn: ({ signal }) => autocompleteTags(q, limit, signal),
    enabled,
  })
}
