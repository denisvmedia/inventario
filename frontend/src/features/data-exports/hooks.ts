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

export function usePersonalDataExports() {
  return useQuery({
    queryKey: dataExportsKeys.list(),
    queryFn: ({ signal }) => listPersonalDataExports(signal),
    refetchInterval: (query) =>
      query.state.data?.exports?.some((e) => isInProgress(e.status))
        ? POLL_WHILE_BUILDING_MS
        : false,
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
  return useMutation<string, Error, string>({
    mutationFn: (id) => fetchPersonalDataExportDownloadURL(id),
    onSuccess: (url) => triggerBrowserDownload(url),
  })
}
