import { useState } from "react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAdminFailedJobsBulkCount, useBulkDiscardAdminFailedJobs } from "@/features/admin/hooks"
import { failedJobTypeLabel } from "@/features/failed-jobs/labels"
import { useAppToast } from "@/hooks/useAppToast"
import { getServerErrorCode } from "@/lib/server-error"

// The BE refuses a bulk discard by an operator without the platform_admin
// role, and a day after today.
const ROLE_REQUIRED_CODE = "admin.role_required"
const INVALID_FAILED_BEFORE_CODE = "failed_job.invalid_failed_before"

// The BE reads a date as its midnight UTC, so the latest day it takes is
// today in UTC.
function todayUTC(): string {
  return new Date().toISOString().slice(0, 10)
}

export interface AdminFailedJobsBulkDiscardDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  // The filters of the page: the discard takes the same type and tenant.
  type?: string
  tenantID?: string
  tenantName?: string
}

// AdminFailedJobsBulkDiscardDialog discards every failed job of the page's
// filters that failed before a day (#2946). Once a day is picked it counts
// the jobs with a dry run, and the confirm button names the count.
export function AdminFailedJobsBulkDiscardDialog({
  open,
  onOpenChange,
  type,
  tenantID,
  tenantName,
}: AdminFailedJobsBulkDiscardDialogProps) {
  const { t } = useTranslation("admin")
  const toast = useAppToast()
  const [failedBefore, setFailedBefore] = useState("")
  const params = { type, tenantID, failedBefore }
  const count = useAdminFailedJobsBulkCount(params, { enabled: open })
  const discard = useBulkDiscardAdminFailedJobs()
  const total = count.data?.discarded ?? 0

  function close(next: boolean) {
    if (next || discard.isPending) return
    setFailedBefore("")
    onOpenChange(false)
  }

  function onConfirm() {
    discard.mutate(params, {
      onSuccess: (result) => {
        toast.success(t("failedJobs.bulk.discarded", { count: result.discarded }))
        close(false)
      },
      onError: (err) => {
        const code = getServerErrorCode(err)
        if (code === ROLE_REQUIRED_CODE) toast.error(t("failedJobs.roleRequired"))
        else if (code === INVALID_FAILED_BEFORE_CODE) toast.error(t("failedJobs.bulk.future"))
        else toast.error(t("failedJobs.bulk.error"))
      },
    })
  }

  let countLine: string
  if (!failedBefore) countLine = t("failedJobs.bulk.pickDate")
  else if (count.isPending) countLine = t("failedJobs.bulk.counting")
  else if (count.isError) countLine = t("failedJobs.bulk.countError")
  else if (total === 0) countLine = t("failedJobs.bulk.none")
  else countLine = t("failedJobs.bulk.count", { count: total })

  return (
    <Dialog open={open} onOpenChange={close}>
      <DialogContent data-testid="admin-failed-jobs-bulk-dialog">
        <DialogHeader>
          <DialogTitle>{t("failedJobs.bulk.title")}</DialogTitle>
          <DialogDescription>{t("failedJobs.bulk.body")}</DialogDescription>
        </DialogHeader>
        <dl
          className="grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 rounded-lg border border-border bg-muted/40 px-3 py-2.5 text-sm"
          data-testid="admin-failed-jobs-bulk-scope"
        >
          <dt className="text-muted-foreground">{t("failedJobs.bulk.scopeType")}</dt>
          <dd className="font-medium">
            {type ? failedJobTypeLabel(t, type) : t("failedJobs.filter.allTypes")}
          </dd>
          <dt className="text-muted-foreground">{t("failedJobs.bulk.scopeTenant")}</dt>
          <dd className="font-medium">
            {tenantID ? tenantName || tenantID : t("failedJobs.filter.allTenants")}
          </dd>
        </dl>
        <div className="flex flex-col gap-1.5">
          <Label htmlFor="admin-failed-jobs-bulk-before">{t("failedJobs.bulk.dateLabel")}</Label>
          <Input
            id="admin-failed-jobs-bulk-before"
            type="date"
            max={todayUTC()}
            value={failedBefore}
            onChange={(e) => setFailedBefore(e.target.value)}
            aria-describedby="admin-failed-jobs-bulk-before-hint"
            data-testid="admin-failed-jobs-bulk-before"
          />
          <p id="admin-failed-jobs-bulk-before-hint" className="text-xs text-muted-foreground">
            {t("failedJobs.bulk.dateHint")}
          </p>
        </div>
        <p className="text-sm" aria-live="polite" data-testid="admin-failed-jobs-bulk-count">
          {countLine}
        </p>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => close(false)}
            disabled={discard.isPending}
          >
            {t("failedJobs.bulk.cancel")}
          </Button>
          <Button
            type="button"
            variant="destructive"
            onClick={onConfirm}
            disabled={discard.isPending || !count.isSuccess || total === 0}
            data-testid="admin-failed-jobs-bulk-confirm"
          >
            {total > 0
              ? t("failedJobs.bulk.confirm", { count: total })
              : t("failedJobs.bulk.confirmIdle")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
