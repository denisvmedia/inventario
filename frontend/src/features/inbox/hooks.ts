import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import type { Commodity } from "@/features/commodities/api"
import { commodityKeys } from "@/features/commodities/keys"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"

import {
  getInboxCount,
  listInbox,
  setCommodityDraft,
  type InboxCounts,
  type InboxListOptions,
  type InboxPage,
} from "./api"
import { inboxKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

// The current group's slug, or "" outside a group. The sidebar renders
// before a group resolves, so the hooks stay disabled without one.
function useSlug(): string {
  return useOptionalCurrentGroup()?.currentGroup?.slug ?? ""
}

export function useInbox(opts: InboxListOptions = {}, { enabled = true }: QueryOptions = {}) {
  const slug = useSlug()
  return useQuery<InboxPage>({
    queryKey: inboxKeys.list(slug, opts),
    queryFn: ({ signal }) => listInbox(opts, signal),
    enabled: enabled && !!slug,
    placeholderData: (prev) => prev,
  })
}

// The server's count of the inbox, for the sidebar and the dashboard. It
// counts what the group holds, each record once.
export function useInboxCount({ enabled = true }: QueryOptions = {}) {
  const slug = useSlug()
  return useQuery<InboxCounts>({
    queryKey: inboxKeys.count(slug),
    queryFn: ({ signal }) => getInboxCount(signal),
    enabled: enabled && !!slug,
    placeholderData: (prev) => prev,
  })
}

export interface SetItemDraftInput {
  id: string
  draft: boolean
}

// useSetItemDraft marks an item as draft or clears the mark. Clearing the
// mark completes the item's review, so the item leaves the inbox; an
// unknown price or date does not bring it back.
export function useSetItemDraft() {
  const qc = useQueryClient()
  const slug = useSlug()
  return useMutation<Commodity, Error, SetItemDraftInput>({
    mutationFn: ({ id, draft }) => setCommodityDraft(id, draft),
    onSuccess: () => qc.invalidateQueries({ queryKey: commodityKeys.group(slug) }),
  })
}
