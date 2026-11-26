import { CircleCheck, RotateCcw } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { FAILED_JOB_ERROR_CODES, type FailedJob } from "@/features/failed-jobs/api"
import { useGroupFailedJobs, useRetryGroupFailedJob } from "@/features/failed-jobs/hooks"
import { failedJobTypeLabel } from "@/features/failed-jobs/labels"
import { useAppToast } from "@/hooks/useAppToast"
import { formatDateTime } from "@/lib/intl"
import { getServerErrorCode } from "@/lib/server-error"
import { cn } from "@/lib/utils"

// The group view lists the newest failures only; a group with more says
// how many it leaves out.
const PAGE_SIZE = 50

interface FailedJobsCardProps {
  groupSlug: string
  className?: string
}

// FailedJobsCard lists the group's background jobs that failed for good
// (#1307): exports, imports, restores, thumbnails, intake scans and
// currency migrations. A group admin can put a job back in its queue; a
// type that cannot run again says so instead. Discarding a job is left to
// the system admin's Failed jobs page. Rendered for admins only: the
// endpoint answers 403 to the other roles.
export function FailedJobsCard({ groupSlug, className }: FailedJobsCardProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const query = useGroupFailedJobs(groupSlug, { perPage: PAGE_SIZE })
  const retry = useRetryGroupFailedJob(groupSlug)

  const jobs = query.data?.jobs ?? []
  const total = query.data?.meta.total ?? jobs.length

  function onRetry(job: FailedJob) {
    retry.mutate(job, {
      onSuccess: () => toast.success(t("groups:settings.failedJobs.retried")),
      onError: (err) => {
        const code = getServerErrorCode(err)
        if (code === FAILED_JOB_ERROR_CODES.notFound) {
          toast.info(t("groups:settings.failedJobs.gone"))
        } else if (code === FAILED_JOB_ERROR_CODES.notRetryable) {
          toast.error(t("groups:settings.failedJobs.notRetryable"))
        } else {
          toast.error(t("groups:settings.failedJobs.retryError"))
        }
      },
    })
  }

  return (
    <div
      className={cn("rounded-xl border border-border bg-card p-6 space-y-4", className)}
      data-testid="failed-jobs-card"
    >
      <div>
        <h2 className="text-base font-semibold">{t("groups:settings.failedJobs.title")}</h2>
        <p className="text-sm text-muted-foreground mt-0.5">
          {t("groups:settings.failedJobs.subtitle")}
        </p>
      </div>

      {query.isLoading ? (
        <p className="text-sm text-muted-foreground">{t("groups:settings.failedJobs.loading")}</p>
      ) : query.isError ? (
        <p className="text-sm text-destructive" data-testid="failed-jobs-error">
          {t("groups:settings.failedJobs.loadError")}
        </p>
      ) : jobs.length === 0 ? (
        <div
          className="flex flex-col items-center justify-center gap-2 py-6 text-center"
          data-testid="failed-jobs-empty"
        >
          <CircleCheck className="size-8 text-muted-foreground/30" aria-hidden="true" />
          <p className="text-sm text-muted-foreground">{t("groups:settings.failedJobs.empty")}</p>
        </div>
      ) : (
        <ul className="divide-y divide-border" data-testid="failed-jobs-list">
          {jobs.map((job) => (
            <FailedJobRow
              key={`${job.job_type}:${job.id}`}
              job={job}
              disabled={retry.isPending}
              onRetry={() => onRetry(job)}
            />
          ))}
        </ul>
      )}

      {total > jobs.length && jobs.length > 0 ? (
        <p className="text-xs text-muted-foreground" data-testid="failed-jobs-more">
          {t("groups:settings.failedJobs.more", { count: total - jobs.length })}
        </p>
      ) : null}
    </div>
  )
}

interface FailedJobRowProps {
  job: FailedJob
  disabled: boolean
  onRetry: () => void
}

function FailedJobRow({ job, disabled, onRetry }: FailedJobRowProps) {
  const { t } = useTranslation()
  const typeLabel = failedJobTypeLabel(t, job.job_type)
  const title = job.title || typeLabel
  return (
    <li className="flex items-start justify-between gap-4 py-3.5" data-testid="failed-job-row">
      <div className="min-w-0 space-y-1">
        <div className="flex min-w-0 items-center gap-2">
          <Badge variant="outline" className="h-5 shrink-0 text-xs font-medium">
            {typeLabel}
          </Badge>
          <p className="truncate text-sm font-medium">{title}</p>
        </div>
        <p className="text-xs text-muted-foreground">
          {job.failed_at
            ? t("groups:settings.failedJobs.failedAt", { date: formatDateTime(job.failed_at) })
            : null}
          {job.failed_at && job.attempts ? " · " : null}
          {job.attempts ? t("groups:settings.failedJobs.attempts", { count: job.attempts }) : null}
        </p>
        {job.error ? (
          <p className="text-xs text-destructive break-words line-clamp-3" title={job.error}>
            {job.error}
          </p>
        ) : null}
      </div>
      {job.retryable ? (
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="shrink-0 gap-1.5"
          onClick={onRetry}
          disabled={disabled}
          aria-label={t("groups:settings.failedJobs.retryLabel", { title })}
          data-testid="failed-job-retry"
        >
          <RotateCcw className="size-3.5" aria-hidden="true" />
          {t("groups:settings.failedJobs.retry")}
        </Button>
      ) : (
        <p
          className="max-w-44 shrink-0 text-right text-xs text-muted-foreground"
          data-testid="failed-job-not-retryable"
        >
          {t("groups:settings.failedJobs.cannotRetry")}
        </p>
      )}
    </li>
  )
}
