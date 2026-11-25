import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import type { Commodity, CreateCommodityRequest } from "@/features/commodities/api"
import { useCreateCommodity } from "@/features/commodities/hooks"
import { commodityKeys } from "@/features/commodities/keys"
import { fileKeys } from "@/features/files/keys"
import type { ListedFile } from "@/features/files/api"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { tagKeys } from "@/features/tags/keys"
import { useInvalidateTrash } from "@/features/trash/hooks"

import {
  discardDraft,
  getDraft,
  listDraftFiles,
  listDrafts,
  promoteDraft,
  type Draft,
  type DraftListOptions,
  type DraftsPage,
} from "./api"
import { draftKeys } from "./keys"

// The current group's slug, or "" outside a group. The item dialog also
// renders outside a GroupProvider in unit tests, so these hooks use the
// optional context and stay disabled without a slug.
function useSlug(): string {
  return useOptionalCurrentGroup()?.currentGroup?.slug ?? ""
}

export function useDrafts(opts: DraftListOptions = {}) {
  const slug = useSlug()
  return useQuery<DraftsPage>({
    queryKey: draftKeys.list(slug, opts),
    queryFn: ({ signal }) => listDrafts(opts, signal),
    enabled: !!slug,
    placeholderData: (prev) => prev,
  })
}

// useDraft reads one draft. Always fetched fresh: another device may have
// saved it since this one last looked.
export function useDraft(id: string | null | undefined) {
  const slug = useSlug()
  return useQuery<Draft>({
    queryKey: draftKeys.detail(slug, id ?? ""),
    queryFn: ({ signal }) => getDraft(id ?? "", signal),
    enabled: !!slug && !!id,
    staleTime: 0,
    gcTime: 0,
    retry: false,
  })
}

export function useDraftFiles(id: string | null | undefined) {
  const slug = useSlug()
  return useQuery<ListedFile[]>({
    queryKey: draftKeys.files(slug, id ?? ""),
    queryFn: ({ signal }) => listDraftFiles(id ?? "", signal),
    enabled: !!slug && !!id,
    staleTime: 0,
  })
}

// useInvalidateDrafts refreshes the draft list and details, and the file
// lists, which name the draft each file is attached to.
export function useInvalidateDrafts() {
  const qc = useQueryClient()
  const slug = useSlug()
  return () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: draftKeys.group(slug) }),
      qc.invalidateQueries({ queryKey: fileKeys.group(slug) }),
    ])
}

// Discarding moves the files nothing else uses to Trash.
export function useDiscardDraft() {
  const invalidate = useInvalidateDrafts()
  const invalidateTrash = useInvalidateTrash()
  return useMutation<void, Error, string>({
    mutationFn: (id) => discardDraft(id),
    onSuccess: () => Promise.all([invalidate(), invalidateTrash()]).then(() => undefined),
  })
}

export interface PromoteDraftInput {
  id: string
  revision: number
  promotionKey: string
  values: CreateCommodityRequest
}

export function usePromoteDraft() {
  const qc = useQueryClient()
  const slug = useSlug()
  const invalidateDrafts = useInvalidateDrafts()
  return useMutation<Commodity & { id: string }, Error, PromoteDraftInput>({
    mutationFn: ({ id, revision, promotionKey, values }) =>
      promoteDraft(id, revision, promotionKey, values),
    onSuccess: () =>
      Promise.all([
        invalidateDrafts(),
        qc.invalidateQueries({ queryKey: commodityKeys.group(slug) }),
        qc.invalidateQueries({ queryKey: tagKeys.group(slug) }),
      ]).then(() => undefined),
  })
}

// The server draft a form saves to, with the revision its values are based
// on. A finished form also names its promotion key: the same key for every
// attempt until one succeeds, so a retry after a lost response gets the
// item the first attempt made.
export interface DraftRef {
  id: string
  revision: number
  promotionKey?: string
}

// newPromotionKey stands in for a caller that keeps no key: its attempts
// cannot be told apart from another device's.
function newPromotionKey(): string {
  return typeof crypto !== "undefined" && crypto.randomUUID
    ? `promote:${crypto.randomUUID()}`
    : `promote:${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`
}

// useSubmitNewItem creates an item, or promotes the draft the form was
// opened from so the draft's files move to the item.
export function useSubmitNewItem() {
  const create = useCreateCommodity()
  const promote = usePromoteDraft()
  return {
    isPending: create.isPending || promote.isPending,
    submit: (values: CreateCommodityRequest, draft?: DraftRef): Promise<Commodity> =>
      draft
        ? promote.mutateAsync({
            id: draft.id,
            revision: draft.revision,
            promotionKey: draft.promotionKey ?? newPromotionKey(),
            values,
          })
        : create.mutateAsync(values),
  }
}
