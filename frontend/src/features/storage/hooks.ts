import { useQuery } from "@tanstack/react-query"

import { getStorageUsageForSlug, type StorageUsage } from "./api"
import { storageKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

// useStorageUsage drives the Storage card in Settings -> Data & storage.
// Disabled until the caller resolves a slug — the Settings card sits on
// `/settings` (no active group in the URL), so it picks one
// explicitly (current group when present, otherwise the user's first
// group) and threads it down here. The hook itself stays slug-agnostic
// so it can be reused from a group-scoped surface in the future.
export function useStorageUsage(slug: string | null, { enabled = true }: QueryOptions = {}) {
  return useQuery<StorageUsage>({
    queryKey: storageKeys.usage(slug ?? ""),
    queryFn: ({ signal }) => getStorageUsageForSlug(slug ?? "", signal),
    enabled: enabled && Boolean(slug),
    placeholderData: (prev) => prev,
  })
}
