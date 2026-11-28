// Data layer for the background jobs that failed for good (#1307), as a
// group admin sees them: GET /g/{slug}/failed-jobs and POST
// /g/{slug}/failed-jobs/{type}/{id}/retry. The group settings page is not a
// /g/:groupSlug route, so the http client has no slug to rewrite with;
// these calls build the full path and skip the rewrite, as
// features/currency-migration does.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type FailedJobView = Schema<"apiserver.FailedJobView">
export type FailedJobListMeta = Schema<"apiserver.FailedJobListMeta">
export type FailedJobTypeInfo = Schema<"apiserver.FailedJobTypeInfo">
type FailedJobListEnvelope = Schema<"apiserver.FailedJobListEnvelope">

// The job types the BE reports failed jobs for on the system-admin plane, in
// the BE's order (models.FailedJobTypes). A group's list leaves out
// personal-data-export: a personal data export belongs to a user.
export const FAILED_JOB_TYPES = [
  "export",
  "import",
  "restore",
  "thumbnail",
  "intake",
  "currency-migration",
  "personal-data-export",
] as const
export type FailedJobType = (typeof FAILED_JOB_TYPES)[number]

// A failed job with the resource id lifted next to its attributes.
export type FailedJob = FailedJobView & { id: string }

export interface FailedJobsParams {
  page?: number
  perPage?: number
}

export interface FailedJobsResult {
  jobs: FailedJob[]
  meta: FailedJobListMeta
}

// The error codes the failed-job endpoints answer with.
export const FAILED_JOB_ERROR_CODES = {
  notFound: "failed_job.not_found",
  notRetryable: "failed_job.not_retryable",
  unknownType: "failed_job.unknown_type",
} as const

function basePath(slug: string): string {
  return `/g/${encodeURIComponent(slug)}/failed-jobs`
}

// Lifts each row's id next to its attributes. A row without an id would be
// a BE bug; it is dropped instead of rendering a row nothing can act on.
export function resolveFailedJobs<T>(
  rows: Array<{ id?: string; attributes?: T }> | undefined
): Array<T & { id: string }> {
  const jobs: Array<T & { id: string }> = []
  for (const row of rows ?? []) {
    if (row.id && row.attributes) jobs.push({ ...row.attributes, id: row.id })
  }
  return jobs
}

export function failedJobsQuery(params: FailedJobsParams): string {
  const query = new URLSearchParams()
  if (params.page !== undefined) query.set("page", String(params.page))
  if (params.perPage !== undefined) query.set("per_page", String(params.perPage))
  return query.toString()
}

export async function listGroupFailedJobs(
  slug: string,
  params: FailedJobsParams = {},
  signal?: AbortSignal
): Promise<FailedJobsResult> {
  const qs = failedJobsQuery(params)
  const body = await http.get<FailedJobListEnvelope>(
    qs ? `${basePath(slug)}?${qs}` : basePath(slug),
    { signal, skipGroupRewrite: true }
  )
  return { jobs: resolveFailedJobs(body.data), meta: body.meta ?? {} }
}

// Puts a failed job of the group back in its queue. The BE answers 204; a
// 404 (failed_job.not_found) means the job is no longer failed, a 409
// (failed_job.not_retryable) that its type can only be discarded.
export async function retryGroupFailedJob(
  slug: string,
  jobType: string,
  id: string
): Promise<void> {
  await http.post(
    `${basePath(slug)}/${encodeURIComponent(jobType)}/${encodeURIComponent(id)}/retry`,
    undefined,
    { skipGroupRewrite: true }
  )
}
