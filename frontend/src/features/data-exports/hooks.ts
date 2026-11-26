// TanStack Query hooks for personal data exports (#2177).
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  fetchPersonalDataExportDownloadURL,
  isInProgress,
  listPersonalDataExports,
  requestPersonalDataExport,
  type PersonalDataExport,
} from "./api"
import { dataExportsKeys } from "./keys"

// While an export is being built the list polls, so the row turns into a
// download link without a reload. Building one takes seconds to minutes.
const POLL_WHILE_BUILDING_MS = 5_000

// A copy still unfinished this long after it was requested is stuck, for
// instance with the worker down; the server queues a stalled build again
// after an hour. The list stops polling for it and refetches when the window
// regains focus.
export const POLL_GIVE_UP_AFTER_MS = 2 * 60 * 60 * 1000

// isWorthPolling reports whether an export may still change soon.
export function isWorthPolling(e: PersonalDataExport, now: number): boolean {
  if (!isInProgress(e.status)) return false
  const requested = e.created_at ? Date.parse(e.created_at) : NaN
  return Number.isNaN(requested) || now - requested < POLL_GIVE_UP_AFTER_MS
}

export function usePersonalDataExports() {
  return useQuery({
    queryKey: dataExportsKeys.list(),
    queryFn: ({ signal }) => listPersonalDataExports(signal),
    refetchInterval: (query) => {
      const now = Date.now()
      return query.state.data?.exports?.some((e) => isWorthPolling(e, now))
        ? POLL_WHILE_BUILDING_MS
        : false
    },
  })
}

export function useRequestPersonalDataExport() {
  const qc = useQueryClient()
  return useMutation<PersonalDataExport, Error, void>({
    mutationFn: () => requestPersonalDataExport(),
    // A refused request (429) also changes what the list says about the
    // next allowed request when another tab made one, so refetch either way.
    onSettled: () => qc.invalidateQueries({ queryKey: dataExportsKeys.all }),
  })
}

// Triggers a native browser download of a signed URL. The response carries
// `Content-Disposition: attachment`, so a transient anchor streams the file
// without unloading the SPA.
function triggerBrowserDownload(url: string): void {
  const a = document.createElement("a")
  a.href = url
  a.rel = "noopener"
  a.download = ""
  document.body.appendChild(a)
  a.click()
  a.remove()
}

export function useDownloadPersonalDataExport() {
  const qc = useQueryClient()
  return useMutation<string, Error, string>({
    mutationFn: (id) => fetchPersonalDataExportDownloadURL(id),
    onSuccess: (url) => triggerBrowserDownload(url),
    // A copy that expired or went back to the queue since the list was
    // loaded answers 404 or 409; the list shows its current state.
    onError: () => qc.invalidateQueries({ queryKey: dataExportsKeys.all }),
  })
}
