import { BellRing, Pencil, Plus, Trash2 } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import type { Reminder, ReminderInput } from "@/features/reminders/api"
import {
  useCreateReminder,
  useDeleteReminder,
  useRemindersForCommodity,
  useUpdateReminder,
} from "@/features/reminders/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatDate } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"
import { cn } from "@/lib/utils"

import { ReminderDialog } from "./ReminderDialog"
import { useReminderText } from "./useReminderText"

interface CommodityRemindersSectionProps {
  commodityId: string
  className?: string
}

// CommodityRemindersSection lists an item's reminders on the Details tab of
// the item page and hosts the create / edit / delete flow (#2805). It is
// self-contained: it fetches its own data, so the page only mounts it.
export function CommodityRemindersSection({
  commodityId,
  className,
}: CommodityRemindersSectionProps) {
  const { t } = useTranslation(["reminders", "common"])
  const toast = useAppToast()
  const confirm = useConfirm()
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<Reminder | null>(null)

  const list = useRemindersForCommodity(commodityId)
  const create = useCreateReminder(commodityId)
  const update = useUpdateReminder()
  const remove = useDeleteReminder()
  const reminders = list.data ?? []

  async function handleDelete(reminder: Reminder) {
    const ok = await confirm({
      title: t("reminders:confirm.delete"),
      confirmLabel: t("common:actions.delete"),
      destructive: true,
    })
    if (!ok) return
    try {
      await remove.mutateAsync(reminder.id)
      toast.success(t("reminders:toast.deleted"))
    } catch (err) {
      toast.error(parseServerError(err, t("reminders:toast.deleteError")))
    }
  }

  async function handleSubmit(values: ReminderInput) {
    try {
      if (editing) {
        await update.mutateAsync({ reminderID: editing.id, input: values })
        toast.success(t("reminders:toast.updated"))
      } else {
        await create.mutateAsync(values)
        toast.success(t("reminders:toast.created"))
      }
      setDialogOpen(false)
      setEditing(null)
    } catch (err) {
      toast.error(parseServerError(err, t("reminders:toast.saveError")))
      // Re-throw so the dialog maps field-level 422s and stays open.
      throw err
    }
  }

  return (
    <Card className={className} data-testid="commodity-reminders">
      <CardHeader className="flex flex-row items-start justify-between gap-4">
        <div className="space-y-1.5">
          <CardTitle className="text-base">{t("reminders:section.title")}</CardTitle>
          <CardDescription>{t("reminders:section.description")}</CardDescription>
        </div>
        <Button
          variant="outline"
          size="sm"
          className="shrink-0"
          onClick={() => {
            setEditing(null)
            setDialogOpen(true)
          }}
          data-testid="reminder-add"
        >
          <Plus className="size-4" aria-hidden="true" />
          {t("reminders:section.add")}
        </Button>
      </CardHeader>
      <CardContent>
        {list.isPending ? (
          <div className="space-y-2">
            <Skeleton className="h-12 w-full" />
            <Skeleton className="h-12 w-full" />
          </div>
        ) : list.isError ? (
          <Alert variant="destructive" data-testid="reminders-error">
            <AlertDescription>{t("reminders:section.loadError")}</AlertDescription>
          </Alert>
        ) : reminders.length === 0 ? (
          <p className="text-sm text-muted-foreground" data-testid="reminders-empty">
            {t("reminders:section.empty")}
          </p>
        ) : (
          <ul data-testid="reminders-list">
            {reminders.map((r, i) => (
              <li key={r.id}>
                {i > 0 ? <Separator /> : null}
                <ReminderRow
                  reminder={r}
                  onEdit={() => {
                    setEditing(r)
                    setDialogOpen(true)
                  }}
                  onDelete={() => void handleDelete(r)}
                />
              </li>
            ))}
          </ul>
        )}
      </CardContent>

      <ReminderDialog
        open={dialogOpen}
        onOpenChange={(open) => {
          setDialogOpen(open)
          if (!open) setEditing(null)
        }}
        initial={editing}
        onSubmit={handleSubmit}
        submitting={create.isPending || update.isPending}
      />
    </Card>
  )
}

interface ReminderRowProps {
  reminder: Reminder
  onEdit: () => void
  onDelete: () => void
}

function ReminderRow({ reminder, onEdit, onDelete }: ReminderRowProps) {
  const { t } = useTranslation(["reminders", "common"])
  const text = useReminderText()
  const next = reminder.next_occurrence
  const days = reminder.days_until_next
  const soon = days !== undefined && days <= 7

  return (
    <div
      className="flex flex-col gap-3 py-3 sm:flex-row sm:items-center"
      data-testid={`reminder-row-${reminder.id}`}
    >
      <BellRing
        className={cn(
          "hidden size-5 shrink-0 sm:block",
          soon ? "text-primary" : "text-muted-foreground"
        )}
        aria-hidden="true"
      />
      <div className="min-w-0 flex-1 space-y-1">
        <p className="truncate text-sm font-medium">{reminder.title}</p>
        <p className="text-xs text-muted-foreground">
          {text.repeat(reminder.repeat_unit, reminder.repeat_interval)}
          {" · "}
          {text.leadTimes(reminder.remind_days_before ?? [])}
        </p>
      </div>
      <div className="flex items-center gap-3 sm:text-right">
        <div className="min-w-0">
          {next ? (
            <>
              <p className="text-sm font-medium" data-testid={`reminder-row-${reminder.id}-next`}>
                {formatDate(next)}
              </p>
              {days !== undefined ? (
                <p className={cn("text-xs", soon ? "text-primary" : "text-muted-foreground")}>
                  {text.daysLeft(days)}
                </p>
              ) : null}
            </>
          ) : (
            <>
              <p className="text-sm font-medium">
                {reminder.due_on ? formatDate(reminder.due_on) : "—"}
              </p>
              <Badge variant="outline" className="mt-0.5">
                {t("reminders:row.passed")}
              </Badge>
            </>
          )}
        </div>
        <div className="ml-auto flex shrink-0 gap-1">
          <Button
            variant="ghost"
            size="icon"
            onClick={onEdit}
            aria-label={t("reminders:row.editLabel", { title: reminder.title })}
            data-testid={`reminder-row-${reminder.id}-edit`}
          >
            <Pencil className="size-4" aria-hidden="true" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            onClick={onDelete}
            aria-label={t("reminders:row.deleteLabel", { title: reminder.title })}
            data-testid={`reminder-row-${reminder.id}-delete`}
          >
            <Trash2 className="size-4" aria-hidden="true" />
          </Button>
        </div>
      </div>
    </div>
  )
}
