import { CheckCircle2, Pencil, Plus, Trash2 } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import { daysUntilDue, type MaintenanceScheduleEntity } from "@/features/maintenance/api"
import {
  useCreateMaintenanceSchedule,
  useDeleteMaintenanceSchedule,
  useMaintenanceForCommodity,
  useMarkMaintenanceDone,
  useUpdateMaintenanceSchedule,
} from "@/features/maintenance/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatDate } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"

import { MaintenanceScheduleDialog } from "./MaintenanceScheduleDialog"

interface MaintenanceTabProps {
  commodityId: string
  // #1554: bundles (count > 1) can't carry per-instance maintenance —
  // mirror the LendTab pattern of showing an empty-state hint and
  // disabling the CTA.
  commodityCount?: number
}

type ScheduleRow = MaintenanceScheduleEntity & { id: string }

// MaintenanceTab is the per-commodity maintenance surface mounted on
// CommodityDetailPage (#1368). Renders the list of schedules ordered
// by next_due_at, surfaces a "Did this" action that advances the
// next-due date, plus add/edit/delete affordances.
export function MaintenanceTab({ commodityId, commodityCount }: MaintenanceTabProps) {
  const { t } = useTranslation(["maintenance", "common"])
  const toast = useAppToast()
  const confirm = useConfirm()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<ScheduleRow | null>(null)

  const isBundle = (commodityCount ?? 0) > 1
  const list = useMaintenanceForCommodity(commodityId, { enabled: !!commodityId && !isBundle })
  const create = useCreateMaintenanceSchedule()
  const update = useUpdateMaintenanceSchedule()
  const done = useMarkMaintenanceDone()
  const remove = useDeleteMaintenanceSchedule()

  const schedules = list.data?.schedules ?? []

  if (isBundle) {
    return (
      <Alert>
        <AlertDescription>
          {t("maintenance:bundleHint", {
            defaultValue:
              "Maintenance schedules can't be set on a bundle. Split the row into per-unit commodities first.",
          })}
        </AlertDescription>
      </Alert>
    )
  }

  async function handleDone(schedule: ScheduleRow) {
    try {
      await done.mutateAsync({ commodityID: commodityId, scheduleID: schedule.id })
      toast.success(t("maintenance:toast.doneSuccess", { defaultValue: "Marked done." }))
    } catch (err) {
      toast.error(
        parseServerError(
          err,
          t("maintenance:toast.doneError", { defaultValue: "Couldn't mark this as done." })
        )
      )
    }
  }

  async function handleDelete(schedule: ScheduleRow) {
    const ok = await confirm({
      title: t("maintenance:confirm.delete", { defaultValue: "Delete this schedule?" }),
      confirmLabel: t("common:actions.delete", { defaultValue: "Delete" }),
      destructive: true,
    })
    if (!ok) return
    try {
      await remove.mutateAsync({ commodityID: commodityId, scheduleID: schedule.id })
      toast.success(t("maintenance:toast.deleteSuccess", { defaultValue: "Schedule deleted." }))
    } catch (err) {
      toast.error(
        parseServerError(
          err,
          t("maintenance:toast.deleteError", { defaultValue: "Couldn't delete the schedule." })
        )
      )
    }
  }

  async function handleToggleEnabled(schedule: ScheduleRow, enabled: boolean) {
    try {
      await update.mutateAsync({
        commodityID: commodityId,
        scheduleID: schedule.id,
        req: { enabled },
      })
    } catch (err) {
      toast.error(
        parseServerError(
          err,
          t("maintenance:toast.updateError", { defaultValue: "Couldn't update the schedule." })
        )
      )
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-semibold">
            {t("maintenance:tab.heading", { defaultValue: "Maintenance" })}
          </h2>
          <p className="text-sm text-muted-foreground">
            {t("maintenance:tab.subheading", {
              defaultValue:
                "Recurring care reminders. We'll email you 14, 7, and 1 day before each due date.",
            })}
          </p>
        </div>
        <Button
          variant="default"
          size="sm"
          onClick={() => {
            setEditing(null)
            setDialogOpen(true)
          }}
          data-testid="maintenance-add"
        >
          <Plus className="size-4" />
          {t("maintenance:tab.add", { defaultValue: "Add schedule" })}
        </Button>
      </div>

      {list.isLoading ? (
        <div className="space-y-2">
          <Skeleton className="h-20 w-full" />
          <Skeleton className="h-20 w-full" />
        </div>
      ) : schedules.length === 0 ? (
        <Card>
          <CardContent className="py-8 text-center text-muted-foreground text-sm">
            {t("maintenance:tab.empty", {
              defaultValue:
                "No maintenance schedules yet. Add one to track recurring care like filter changes, oil changes, or descaling.",
            })}
          </CardContent>
        </Card>
      ) : (
        <ul className="space-y-3" data-testid="maintenance-list">
          {schedules.map((s) => (
            <ScheduleCard
              key={s.id}
              schedule={s}
              onDone={() => handleDone(s)}
              onEdit={() => {
                setEditing(s)
                setDialogOpen(true)
              }}
              onDelete={() => handleDelete(s)}
              onToggleEnabled={(enabled) => handleToggleEnabled(s, enabled)}
              busyDone={done.isPending}
            />
          ))}
        </ul>
      )}

      <MaintenanceScheduleDialog
        open={dialogOpen}
        onOpenChange={(o) => {
          setDialogOpen(o)
          if (!o) setEditing(null)
        }}
        commodityId={commodityId}
        initial={editing}
        onSubmit={async (values) => {
          try {
            if (editing) {
              await update.mutateAsync({
                commodityID: commodityId,
                scheduleID: editing.id,
                req: values,
              })
              toast.success(
                t("maintenance:toast.updateSuccess", { defaultValue: "Schedule updated." })
              )
            } else {
              await create.mutateAsync({ commodity_id: commodityId, ...values })
              toast.success(
                t("maintenance:toast.createSuccess", { defaultValue: "Schedule added." })
              )
            }
            setDialogOpen(false)
            setEditing(null)
          } catch (err) {
            toast.error(
              parseServerError(
                err,
                t("maintenance:toast.saveError", { defaultValue: "Couldn't save the schedule." })
              )
            )
            // Re-throw so the dialog can map field-level 422 errors onto its
            // inputs (it stays open). The toast above is the summary.
            throw err
          }
        }}
        submitting={create.isPending || update.isPending}
      />
    </div>
  )
}

interface ScheduleCardProps {
  schedule: ScheduleRow
  onDone: () => void
  onEdit: () => void
  onDelete: () => void
  onToggleEnabled: (enabled: boolean) => void
  busyDone: boolean
}

function ScheduleCard({
  schedule,
  onDone,
  onEdit,
  onDelete,
  onToggleEnabled,
  busyDone,
}: ScheduleCardProps) {
  const { t } = useTranslation(["maintenance", "common"])
  // Paused (enabled = false) schedules suppress the urgent badges —
  // the worker also skips them on the scan side, so showing
  // "Overdue" / "Due soon" on a row that won't fire any reminder
  // would be a UX lie.
  const active = !!schedule.enabled
  const days = active ? daysUntilDue(schedule) : null
  const overdue = active && days !== null && days < 0
  const dueSoon = active && days !== null && days >= 0 && days <= 14

  return (
    <li>
      <Card
        className={overdue ? "border-destructive/40" : dueSoon ? "border-amber-400/40" : undefined}
      >
        <CardHeader className="pb-2 flex flex-row items-start justify-between gap-2">
          <div>
            <CardTitle className="text-base">{schedule.title}</CardTitle>
            <p className="text-xs text-muted-foreground mt-1">
              {t("maintenance:row.intervalLabel", {
                defaultValue: "Every {{count}} days",
                count: schedule.interval_days ?? 0,
              })}
            </p>
          </div>
          <div className="flex items-center gap-2">
            {!schedule.enabled ? (
              <Badge variant="outline">
                {t("maintenance:row.paused", { defaultValue: "Paused" })}
              </Badge>
            ) : overdue ? (
              <Badge variant="destructive">
                {t("maintenance:row.overdue", { defaultValue: "Overdue" })}
              </Badge>
            ) : dueSoon ? (
              <Badge variant="secondary">
                {t("maintenance:row.dueSoon", { defaultValue: "Due soon" })}
              </Badge>
            ) : null}
            <Switch
              checked={!!schedule.enabled}
              onCheckedChange={onToggleEnabled}
              aria-label={t("maintenance:row.enabledLabel", { defaultValue: "Enabled" })}
            />
          </div>
        </CardHeader>
        <CardContent className="pt-0 space-y-3">
          <div className="grid grid-cols-2 gap-2 text-sm">
            <div>
              <p className="text-xs text-muted-foreground">
                {t("maintenance:row.nextDueLabel", { defaultValue: "Next due" })}
              </p>
              <p className="font-medium" data-testid={`schedule-${schedule.id}-next-due`}>
                {schedule.next_due_at ? formatDate(schedule.next_due_at) : "—"}
                {days !== null ? (
                  <span className="text-xs text-muted-foreground ml-2">
                    {overdue
                      ? t("maintenance:row.overdueByDays", {
                          defaultValue: "{{count}} days overdue",
                          count: Math.abs(days),
                        })
                      : t("maintenance:row.dueInDays", {
                          defaultValue: "in {{count}} days",
                          count: days,
                        })}
                  </span>
                ) : null}
              </p>
            </div>
            <div>
              <p className="text-xs text-muted-foreground">
                {t("maintenance:row.lastDoneLabel", { defaultValue: "Last done" })}
              </p>
              <p className="font-medium">
                {schedule.last_done_at ? formatDate(schedule.last_done_at) : "—"}
              </p>
            </div>
          </div>
          {schedule.notes ? (
            <p className="text-sm text-muted-foreground">{schedule.notes}</p>
          ) : null}
          <div className="flex flex-wrap gap-2">
            <Button
              variant="default"
              size="sm"
              disabled={busyDone}
              onClick={onDone}
              data-testid={`schedule-${schedule.id}-done`}
            >
              <CheckCircle2 className="size-4" />
              {t("maintenance:row.markDone", { defaultValue: "I did this" })}
            </Button>
            <Button variant="outline" size="sm" onClick={onEdit}>
              <Pencil className="size-4" />
              {t("common:actions.edit", { defaultValue: "Edit" })}
            </Button>
            <Button variant="ghost" size="sm" onClick={onDelete}>
              <Trash2 className="size-4" />
              {t("common:actions.delete", { defaultValue: "Delete" })}
            </Button>
          </div>
        </CardContent>
      </Card>
    </li>
  )
}
