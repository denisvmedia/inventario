import { useQuery } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"

import { getStorageUsage, type StorageUsage } from "./api"
import { storageKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

// useStorageUsage drives the Storage card in Settings -> Data & storage.
// Disabled until a group is active so the wrapper doesn't fire a request
// that would 404 on the un-rewritten /storage-usage path.
export function useStorageUsage({ enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<StorageUsage>({
    queryKey: storageKeys.usage(slug),
    queryFn: ({ signal }) => getStorageUsage(signal),
    enabled: enabled && Boolean(slug),
    placeholderData: (prev) => prev,
  })
}
