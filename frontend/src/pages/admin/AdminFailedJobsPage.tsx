import { useEffect, useMemo, useState } from "react"
import { ArchiveX, CircleCheck, RotateCcw, TriangleAlert } from "lucide-react"
import { useSearchParams } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { RouteTitle } from "@/components/routing/RouteTitle"
import type { AdminFailedJob } from "@/features/admin/api"
import {
  useAdminFailedJobs,
  useAdminTenants,
  useDiscardAdminFailedJob,
  useRetryAdminFailedJob,
} from "@/features/admin/hooks"
import { FAILED_JOB_ERROR_CODES, FAILED_JOB_TYPES } from "@/features/failed-jobs/api"
import { failedJobTypeLabel } from "@/features/failed-jobs/labels"
import { useAppToast } from "@/hooks/useAppToast"
import { formatDateTime } from "@/lib/intl"
import { getServerErrorCode } from "@/lib/server-error"

import { AdminPagination } from "./AdminPagination"

// Page size of the server-side paginated list; the BE caps per_page at 100.
const PAGE_SIZE = 20

// One page of tenants fills the tenant filter, as on AdminGroupsPage.
const TENANT_FILTER_PER_PAGE = 100

const COLUMN_COUNT = 6

function parsePage(raw: string | null): number {
  const n = Number(raw)
  return Number.isInteger(n) && n >= 1 ? n : 1
}

function parseType(raw: string | null): string | undefined {
  return FAILED_JOB_TYPES.find((type) => type === raw)
}

// AdminFailedJobsPage is the /admin/failed-jobs route (#1307): the
// background jobs of every tenant that failed for good and that nobody
// retried or discarded, newest failure first. The type filter, the tenant
// filter and the page round-trip through the URL (?type, ?tenantID,
// ?page). A job whose type can run again has Retry; every job has Discard,
// which asks first. Both are written to the audit log by the BE.
export function AdminFailedJobsPage() {
  const { t } = useTranslation("admin")
  const toast = useAppToast()
  const [searchParams, setSearchParams] = useSearchParams()

  const type = parseType(searchParams.get("type"))
  const tenantID = searchParams.get("tenantID") ?? ""
  const page = parsePage(searchParams.get("page"))

  const tenantsQuery = useAdminTenants({ page: 1, perPage: TENANT_FILTER_PER_PAGE, sort: "name" })
  const tenantOptions = useMemo(() => tenantsQuery.data?.tenants ?? [], [tenantsQuery.data])

  const query = useAdminFailedJobs({
    type,
    tenantID: tenantID || undefined,
    page,
    perPage: PAGE_SIZE,
  })
  const jobs = useMemo(() => query.data?.jobs ?? [], [query.data])
  const total = query.data?.meta.total ?? jobs.length
  const totalPages = query.data?.meta.total_pages ?? 1

  const retry = useRetryAdminFailedJob()
  const discard = useDiscardAdminFailedJob()
  const [discarding, setDiscarding] = useState<AdminFailedJob | null>(null)

  // A pinned tenant past the first page of tenants still shows in the
  // filter, named from the loaded rows when one carries it.
  const missingTenant = useMemo(() => {
    if (!tenantID || tenantOptions.some((tenant) => tenant.id === tenantID)) return null
    const name = jobs.find((job) => job.tenant_id === tenantID)?.tenant_name
    return { id: tenantID, name: name || tenantID }
  }, [tenantID, tenantOptions, jobs])

  // A ?page past the last page (a deep link, or the last job on the page
  // was retried) snaps back to the last real page.
  useEffect(() => {
    if (query.data && page > totalPages && totalPages >= 1) goToPage(totalPages)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [query.data, page, totalPages])

  function setFilter(key: "type" | "tenantID", value: string) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (value === "all") next.delete(key)
        else next.set(key, value)
        next.delete("page")
        return next
      },
      { replace: true }
    )
  }

  function goToPage(nextPage: number) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev)
        if (nextPage <= 1) next.delete("page")
        else next.set("page", String(nextPage))
        return next
      },
      { replace: true }
    )
  }

  function onActionError(
    err: Error,
    fallbackKey: "failedJobs.retryError" | "failedJobs.discardError"
  ) {
    const code = getServerErrorCode(err)
    if (code === FAILED_JOB_ERROR_CODES.notFound) toast.info(t("failedJobs.gone"))
    else if (code === FAILED_JOB_ERROR_CODES.notRetryable) toast.error(t("failedJobs.notRetryable"))
    else toast.error(t(fallbackKey))
  }

  function onRetry(job: AdminFailedJob) {
    retry.mutate(job, {
      onSuccess: () => toast.success(t("failedJobs.retried")),
      onError: (err) => onActionError(err, "failedJobs.retryError"),
    })
  }

  function onDiscardConfirm() {
    if (!discarding) return
    discard.mutate(discarding, {
      onSuccess: () => {
        toast.success(t("failedJobs.discarded"))
        setDiscarding(null)
      },
      onError: (err) => {
        onActionError(err, "failedJobs.discardError")
        setDiscarding(null)
      },
    })
  }

  const busy = retry.isPending || discard.isPending
  const filtered = !!type || !!tenantID

  return (
    <>
      <RouteTitle title={t("failedJobs.title")} />
      <div className="flex flex-col gap-6" data-testid="admin-failed-jobs-page">
        <div className="flex items-start gap-3">
          <div className="flex size-9 items-center justify-center rounded-lg bg-primary/10 shrink-0">
            <TriangleAlert className="size-5 text-primary" />
          </div>
          <div>
            <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">
              {t("failedJobs.title")}
            </h1>
            <p className="mt-1 text-muted-foreground">{t("failedJobs.subtitle")}</p>
          </div>
        </div>

        <div className="flex flex-col gap-3 sm:flex-row">
          <Select value={type ?? "all"} onValueChange={(value) => setFilter("type", value)}>
            <SelectTrigger
              className="sm:w-52"
              aria-label={t("failedJobs.filter.typeLabel")}
              data-testid="admin-failed-jobs-type-filter"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("failedJobs.filter.allTypes")}</SelectItem>
              {FAILED_JOB_TYPES.map((jobType) => (
                <SelectItem key={jobType} value={jobType}>
                  {failedJobTypeLabel(t, jobType)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={tenantID || "all"} onValueChange={(value) => setFilter("tenantID", value)}>
            <SelectTrigger
              className="sm:w-52"
              aria-label={t("failedJobs.filter.tenantLabel")}
              data-testid="admin-failed-jobs-tenant-filter"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">{t("failedJobs.filter.allTenants")}</SelectItem>
              {tenantOptions.map((tenant) =>
                tenant.id ? (
                  <SelectItem key={tenant.id} value={tenant.id}>
                    {tenant.name ?? tenant.slug ?? tenant.id}
                  </SelectItem>
                ) : null
              )}
              {missingTenant ? (
                <SelectItem value={missingTenant.id}>{missingTenant.name}</SelectItem>
              ) : null}
            </SelectContent>
          </Select>
        </div>

        {query.isError ? (
          <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-6 text-sm text-destructive">
            {t("failedJobs.loadError")}
          </div>
        ) : (
          <>
            <div className="rounded-xl border border-border bg-card overflow-hidden">
              <Table data-testid="admin-failed-jobs-list">
                <TableHeader>
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="pl-4">{t("failedJobs.table.job")}</TableHead>
                    <TableHead>{t("failedJobs.table.tenant")}</TableHead>
                    <TableHead>{t("failedJobs.table.error")}</TableHead>
                    <TableHead>{t("failedJobs.table.failed")}</TableHead>
                    <TableHead className="text-right">{t("failedJobs.table.attempts")}</TableHead>
                    <TableHead className="pr-4 text-right">
                      <span className="sr-only">{t("failedJobs.table.actions")}</span>
                    </TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {query.isLoading ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell
                        colSpan={COLUMN_COUNT}
                        className="h-32 text-center text-sm text-muted-foreground"
                      >
                        {t("failedJobs.loading")}
                      </TableCell>
                    </TableRow>
                  ) : jobs.length === 0 ? (
                    <TableRow className="hover:bg-transparent">
                      <TableCell colSpan={COLUMN_COUNT} className="h-32 text-center">
                        <div
                          className="flex flex-col items-center justify-center gap-2"
                          data-testid="admin-failed-jobs-empty"
                        >
                          <CircleCheck className="size-8 text-muted-foreground/30" />
                          <p className="text-sm text-muted-foreground">
                            {filtered ? t("failedJobs.emptyFiltered") : t("failedJobs.empty")}
                          </p>
                        </div>
                      </TableCell>
                    </TableRow>
                  ) : (
                    jobs.map((job) => (
                      <FailedJobRow
                        key={`${job.job_type}:${job.id}`}
                        job={job}
                        disabled={busy}
                        onRetry={() => onRetry(job)}
                        onDiscard={() => setDiscarding(job)}
                      />
                    ))
                  )}
                </TableBody>
              </Table>
            </div>

            {!query.isLoading && jobs.length > 0 ? (
              <AdminPagination
                page={page}
                totalPages={totalPages}
                total={total}
                pageSize={PAGE_SIZE}
                onPageChange={goToPage}
              />
            ) : null}
          </>
        )}
      </div>

      <Dialog
        open={discarding !== null}
        onOpenChange={(open) => {
          if (!open && !discard.isPending) setDiscarding(null)
        }}
      >
        <DialogContent data-testid="admin-failed-job-discard-dialog">
          <DialogHeader>
            <DialogTitle>{t("failedJobs.discardDialog.title")}</DialogTitle>
            <DialogDescription>{t("failedJobs.discardDialog.body")}</DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => setDiscarding(null)}
              disabled={discard.isPending}
            >
              {t("failedJobs.discardDialog.cancel")}
            </Button>
            <Button
              type="button"
              variant="destructive"
              onClick={onDiscardConfirm}
              disabled={discard.isPending}
              data-testid="admin-failed-job-discard-confirm"
            >
              {t("failedJobs.discardDialog.confirm")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

function FailedJobRow({
  job,
  disabled,
  onRetry,
  onDiscard,
}: {
  job: AdminFailedJob
  disabled: boolean
  onRetry: () => void
  onDiscard: () => void
}) {
  const { t } = useTranslation("admin")
  const typeLabel = failedJobTypeLabel(t, job.job_type)
  const title = job.title || typeLabel
  return (
    <TableRow data-testid="admin-failed-job-row">
      <TableCell className="pl-4 py-3.5 align-top">
        <div className="flex flex-col gap-1">
          <Badge variant="outline" className="h-5 w-fit text-xs font-medium">
            {typeLabel}
          </Badge>
          <span className="max-w-56 truncate text-sm font-medium">{title}</span>
          <span className="font-mono text-xs text-muted-foreground">{job.id}</span>
        </div>
      </TableCell>
      <TableCell className="py-3.5 align-top">
        <div className="flex flex-col gap-0.5">
          <span className="text-sm">{job.tenant_name || job.tenant_id || "—"}</span>
          <span className="text-xs text-muted-foreground">
            {job.group_name || job.group_slug || "—"}
          </span>
        </div>
      </TableCell>
      <TableCell className="py-3.5 align-top">
        <p
          className="max-w-xs whitespace-normal break-words text-xs text-muted-foreground line-clamp-3"
          title={job.error}
        >
          {job.error || "—"}
        </p>
      </TableCell>
      <TableCell className="py-3.5 align-top text-sm text-muted-foreground">
        {job.failed_at ? formatDateTime(job.failed_at) : "—"}
      </TableCell>
      <TableCell className="py-3.5 align-top text-right text-sm tabular-nums">
        {job.attempts ?? 0}
      </TableCell>
      <TableCell className="pr-4 py-3.5 align-top">
        <div className="flex justify-end gap-2">
          {job.retryable ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="gap-1.5"
              onClick={onRetry}
              disabled={disabled}
              aria-label={t("failedJobs.retryLabel", { title })}
              data-testid="admin-failed-job-retry"
            >
              <RotateCcw className="size-3.5" aria-hidden="true" />
              {t("failedJobs.retry")}
            </Button>
          ) : null}
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="gap-1.5"
            onClick={onDiscard}
            disabled={disabled}
            aria-label={t("failedJobs.discardLabel", { title })}
            data-testid="admin-failed-job-discard"
          >
            <ArchiveX className="size-3.5" aria-hidden="true" />
            {t("failedJobs.discard")}
          </Button>
        </div>
      </TableCell>
    </TableRow>
  )
}
