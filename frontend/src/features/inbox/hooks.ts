import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import type { Commodity } from "@/features/commodities/api"
import { commodityKeys } from "@/features/commodities/keys"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"

import {
  getInboxChecks,
  getInboxCount,
  listInbox,
  setCommodityDraft,
  type InboxCheck,
  type InboxCounts,
  type InboxListOptions,
  type InboxPage,
} from "./api"
import { inboxKeys, sameInboxList } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

// The current group's slug, or "" outside a group. The sidebar renders
// before a group resolves, so the hooks stay disabled without one.
function useSlug(): string {
  return useOptionalCurrentGroup()?.currentGroup?.slug ?? ""
}

// A page of the inbox. While the next page of the same reason loads, the
// page before stays on screen; a page of another reason does not, since its
// rows and their buttons do not belong under the newly selected tab.
export function useInbox(opts: InboxListOptions = {}, { enabled = true }: QueryOptions = {}) {
  const slug = useSlug()
  const queryKey = inboxKeys.list(slug, opts)
  return useQuery<InboxPage>({
    queryKey,
    queryFn: ({ signal }) => listInbox(opts, signal),
    enabled: enabled && !!slug,
    placeholderData: (prev, prevQuery) =>
      prevQuery && sameInboxList(prevQuery.queryKey, queryKey) ? prev : undefined,
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

// The quality checks next to the inbox, with the server's counts. Missing
// details are not unfinished work, so these never add to useInboxCount.
export function useInboxChecks({ enabled = true }: QueryOptions = {}) {
  const slug = useSlug()
  return useQuery<InboxCheck[]>({
    queryKey: inboxKeys.checks(slug),
    queryFn: ({ signal }) => getInboxChecks(signal),
    enabled: enabled && !!slug,
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
