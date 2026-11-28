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

// The back office's view under /admin/failed-jobs: list, retry, discard,
// and the bulk discard (#2946), which takes the jobs of ?type and ?tenantID
// that failed before ?failed_before (a date, read as its midnight UTC) and
// with ?dry_run=true only counts them. A day after today answers 400.
export function admin(jobs: AdminFailedJob[] = []) {
  const b = backend("/admin/failed-jobs", jobs, ["retry", "discard"])
  const bulk = http.post(apiUrl("/admin/failed-jobs/bulk-discard"), ({ request }) => {
    const url = new URL(request.url)
    b.state.requests.push({ method: "POST", url })
    const raw = url.searchParams.get("failed_before") ?? ""
    const before = Date.parse(`${raw}T00:00:00Z`)
    if (Number.isNaN(before) || before > Date.now()) {
      return errorBody(400, "failed_job.invalid_failed_before")
    }
    const type = url.searchParams.get("type")
    const tenantID = url.searchParams.get("tenantID")
    const taken = b.state.jobs.filter(
      (job) =>
        (!type || job.job_type === type) &&
        (!tenantID || job.tenant_id === tenantID) &&
        !!job.failed_at &&
        Date.parse(job.failed_at) < before
    )
    const byType: Record<string, number> = {}
    for (const job of taken) byType[job.job_type ?? ""] = (byType[job.job_type ?? ""] ?? 0) + 1
    const dryRun = url.searchParams.get("dry_run") === "true"
    if (!dryRun) b.state.jobs = b.state.jobs.filter((job) => !taken.includes(job))
    return HttpResponse.json({
      meta: { discarded: taken.length, by_type: byType, dry_run: dryRun },
    })
  })
  return { state: b.state, handlers: [bulk, ...b.handlers] }
}
