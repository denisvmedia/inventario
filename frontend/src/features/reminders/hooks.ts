import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import {
  createReminder,
  deleteReminder,
  listRemindersForCommodity,
  listUpcoming,
  updateReminder,
  type Reminder,
  type ReminderInput,
} from "./api"
import { reminderKeys } from "./keys"

export function useRemindersForCommodity(commodityID: string | undefined) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<Reminder[]>({
    queryKey: reminderKeys.byCommodity(slug, commodityID ?? ""),
    queryFn: ({ signal }) => {
      if (!commodityID) {
        throw new Error("useRemindersForCommodity called without a commodity id")
      }
      return listRemindersForCommodity(commodityID, signal)
    },
    enabled: !!commodityID && !!currentGroup,
  })
}

export function useUpcoming(days: number) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery({
    queryKey: reminderKeys.upcoming(slug, days),
    queryFn: ({ signal }) => listUpcoming(days, signal),
    enabled: !!currentGroup,
    placeholderData: (prev) => prev,
  })
}

// Every reminder write invalidates the whole group subtree: the item's
// list and every cached Upcoming window can change.
function useInvalidateGroup() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return () => qc.invalidateQueries({ queryKey: reminderKeys.group(slug) })
}

export function useCreateReminder(commodityID: string) {
  const invalidate = useInvalidateGroup()
  return useMutation<Reminder, Error, ReminderInput>({
    mutationFn: (input) => createReminder(commodityID, input),
    onSuccess: () => invalidate(),
  })
}

export function useUpdateReminder() {
  const invalidate = useInvalidateGroup()
  return useMutation<Reminder, Error, { reminderID: string; input: Partial<ReminderInput> }>({
    mutationFn: ({ reminderID, input }) => updateReminder(reminderID, input),
    onSuccess: () => invalidate(),
  })
}

export function useDeleteReminder() {
  const invalidate = useInvalidateGroup()
  return useMutation<void, Error, string>({
    mutationFn: (reminderID) => deleteReminder(reminderID),
    onSuccess: () => invalidate(),
  })
}
