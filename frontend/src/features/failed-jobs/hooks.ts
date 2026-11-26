import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  listGroupFailedJobs,
  retryGroupFailedJob,
  type FailedJob,
  type FailedJobsParams,
  type FailedJobsResult,
} from "./api"
import { failedJobKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

// Lists the group's failed jobs. Callers pass enabled only for a group
// admin: the endpoint answers 403 to every other role.
export function useGroupFailedJobs(
  slug: string | null | undefined,
  params: FailedJobsParams = {},
  { enabled = true }: QueryOptions = {}
) {
  const safeSlug = slug ?? ""
  return useQuery<FailedJobsResult>({
    queryKey: failedJobKeys.groupList(safeSlug, params),
    queryFn: ({ signal }) => listGroupFailedJobs(safeSlug, params, signal),
    enabled: enabled && !!safeSlug,
  })
}

// Retries one failed job. Success or failure, the list is refetched: a
// retried job leaves it, and a 404 means someone else already acted on it.
export function useRetryGroupFailedJob(slug: string | null | undefined) {
  const qc = useQueryClient()
  const safeSlug = slug ?? ""
  return useMutation<void, Error, Pick<FailedJob, "id" | "job_type">>({
    mutationFn: (job) => retryGroupFailedJob(safeSlug, job.job_type ?? "", job.id),
    onSettled: () => qc.invalidateQueries({ queryKey: failedJobKeys.group(safeSlug) }),
  })
}
