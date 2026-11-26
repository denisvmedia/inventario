// Personal data export client (#2177). Mirrors
// apiserver/personal_data_exports.go. The endpoints live under
// /api/v1/users/me/data-exports and belong to the user, not a group, so the
// group rewrite in lib/http is skipped.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type PersonalDataExport = Schema<"apiserver.PersonalDataExportView">
export type PersonalDataExportsList = Schema<"apiserver.PersonalDataExportsListResponse">
export type PersonalDataExportDownloadURL =
  Schema<"apiserver.PersonalDataExportDownloadURLResponse">
export type PersonalDataExportStatus = NonNullable<PersonalDataExport["status"]>

export const DATA_EXPORT_RATE_LIMITED = "data_export.rate_limited"
export const DATA_EXPORT_IMPERSONATION_FORBIDDEN = "data_export.impersonation_forbidden"

export async function listPersonalDataExports(
  signal?: AbortSignal
): Promise<PersonalDataExportsList> {
  return http.get<PersonalDataExportsList>("/users/me/data-exports", {
    signal,
    skipGroupRewrite: true,
  })
}

export async function requestPersonalDataExport(): Promise<PersonalDataExport> {
  return http.post<PersonalDataExport>("/users/me/data-exports", undefined, {
    skipGroupRewrite: true,
  })
}

// fetchPersonalDataExportDownloadURL mints a short-lived signed URL for a
// finished export. The URL is app-absolute (`/api/v1/data-exports/...`),
// bound to this browser session, and serves the archive as an attachment.
export async function fetchPersonalDataExportDownloadURL(id: string): Promise<string> {
  const body = await http.post<PersonalDataExportDownloadURL>(
    `/users/me/data-exports/${encodeURIComponent(id)}/download-url`,
    undefined,
    { skipGroupRewrite: true }
  )
  if (!body.url) {
    throw new Error("Malformed download-url response: missing url")
  }
  return body.url
}

// isInProgress reports whether the worker has yet to finish an export.
export function isInProgress(status: PersonalDataExportStatus | undefined): boolean {
  return status === "pending" || status === "in_progress"
}
