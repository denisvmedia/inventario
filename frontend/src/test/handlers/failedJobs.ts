import { http, HttpResponse } from "msw"

import type { AdminFailedJob } from "@/features/admin/api"
import type { FailedJob } from "@/features/failed-jobs/api"

import { apiUrl } from "."

// A failed-jobs backend for one group or for the back office: GET lists
// what is still failed, retry and discard take a job off the list. A job
// whose `retryable` is false answers retry with 409, an unknown id with
// 404, as the BE does. `requests` records every call for assertions.
interface FailedJobsState<T> {
  jobs: T[]
  requests: Array<{ method: string; url: URL }>
}

function envelope<T extends { id: string }>(jobs: T[], url: URL) {
  const page = Number(url.searchParams.get("page") ?? "1")
  const perPage = Number(url.searchParams.get("per_page") ?? "50")
  const type = url.searchParams.get("type")
  const tenantID = url.searchParams.get("tenantID")
  const matching = jobs.filter((job) => {
    const row = job as T & { job_type?: string; tenant_id?: string }
    return (!type || row.job_type === type) && (!tenantID || row.tenant_id === tenantID)
  })
  const start = (page - 1) * perPage
  return {
    data: matching.slice(start, start + perPage).map(({ id, ...attributes }) => ({
      id,
      type: "failed_jobs",
      attributes,
    })),
    meta: {
      page,
      per_page: perPage,
      total: matching.length,
      total_pages: Math.max(1, Math.ceil(matching.length / perPage)),
    },
  }
}

function errorBody(status: number, code: string) {
  return HttpResponse.json({ errors: [{ status: String(status), code }] }, { status })
}

function backend<T extends { id: string; job_type?: string; retryable?: boolean }>(
  base: string,
  initial: T[],
  actions: Array<"retry" | "discard">
) {
  const state: FailedJobsState<T> = { jobs: [...initial], requests: [] }
  const handlers = [
    http.get(apiUrl(base), ({ request }) => {
      const url = new URL(request.url)
      state.requests.push({ method: "GET", url })
      return HttpResponse.json(envelope(state.jobs, url))
    }),
    ...actions.map((action) =>
      http.post(apiUrl(`${base}/:jobType/:jobID/${action}`), ({ request, params }) => {
        state.requests.push({ method: "POST", url: new URL(request.url) })
        const job = state.jobs.find((j) => j.id === params.jobID && j.job_type === params.jobType)
        if (!job) return errorBody(404, "failed_job.not_found")
        if (action === "retry" && !job.retryable) return errorBody(409, "failed_job.not_retryable")
        state.jobs = state.jobs.filter((j) => j !== job)
        return new HttpResponse(null, { status: 204 })
      })
    ),
  ]
  return { state, handlers }
}

// The group admin's view under /g/{slug}/failed-jobs: list and retry.
export function group(slug: string, jobs: FailedJob[] = []) {
  return backend(`/g/${encodeURIComponent(slug)}/failed-jobs`, jobs, ["retry"])
}

// The back office's view under /admin/failed-jobs: list, retry, discard.
export function admin(jobs: AdminFailedJob[] = []) {
  return backend("/admin/failed-jobs", jobs, ["retry", "discard"])
}
