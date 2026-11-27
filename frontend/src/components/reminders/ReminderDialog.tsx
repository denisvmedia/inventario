import { zodResolver } from "@hookform/resolvers/zod"
import { Controller, useForm, useWatch } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { FieldError } from "@/components/FieldError"
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  REMINDER_MAX_INTERVAL,
  REMINDER_REPEAT_UNITS,
  type Reminder,
  type ReminderInput,
  type ReminderRepeatUnit,
} from "@/features/reminders/api"
import {
  parseLeadTimes,
  reminderFormSchema,
  type ReminderFormInput,
  type ReminderFormOutput,
} from "@/features/reminders/schemas"
import { useValueWhileOpen } from "@/hooks/useValueWhileOpen"
import { applyServerFieldErrors } from "@/lib/form-errors"

interface ReminderDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  initial: Reminder | null
  onSubmit: (values: ReminderInput) => Promise<void>
  submitting: boolean
}

// ReminderDialog creates or edits an item reminder (#2805). The form lives
// in the dialog content, which mounts on open and unmounts once the close
// animation ends, so every open starts from fresh defaults; the key starts
// a new form when the reminder changes. While the dialog closes it keeps the
// reminder it showed, as in MaintenanceScheduleDialog.
export function ReminderDialog({
  open,
  onOpenChange,
  initial,
  onSubmit,
  submitting,
}: ReminderDialogProps) {
  const shown = useValueWhileOpen(open, initial)
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent variant="fullscreen-mobile">
        <ReminderForm
          key={shown?.id ?? "create"}
          initial={shown}
          onOpenChange={onOpenChange}
          onSubmit={onSubmit}
          submitting={submitting}
        />
      </DialogContent>
    </Dialog>
  )
}

function buildDefaults(initial: Reminder | null): ReminderFormInput {
  return {
    title: initial?.title ?? "",
    due_on: initial?.due_on ?? "",
    remind_days_before: (initial?.remind_days_before ?? [7, 1]).join(", "),
    repeat_unit: initial?.repeat_unit ?? "none",
    repeat_interval: initial?.repeat_interval ?? 1,
  }
}

interface ReminderFormProps {
  initial: Reminder | null
  onOpenChange: (open: boolean) => void
  onSubmit: (values: ReminderInput) => Promise<void>
  submitting: boolean
}

function ReminderForm({ initial, onOpenChange, onSubmit, submitting }: ReminderFormProps) {
  const { t } = useTranslation(["reminders", "common"])
  const {
    control,
    formState: { errors, isSubmitting },
    handleSubmit,
    register,
    setError,
  } = useForm<ReminderFormInput, unknown, ReminderFormOutput>({
    resolver: zodResolver(reminderFormSchema),
    defaultValues: buildDefaults(initial),
  })
  const repeatUnit = useWatch({ control, name: "repeat_unit" })
  const isEdit = !!initial
  const busy = submitting || isSubmitting

  return (
    <form
      className="space-y-4"
      noValidate
      onSubmit={handleSubmit(async (values) => {
        const repeats = values.repeat_unit !== "none"
        try {
          await onSubmit({
            title: values.title,
            due_on: values.due_on,
            remind_days_before: parseLeadTimes(values.remind_days_before) ?? [],
            repeat_unit: values.repeat_unit,
            repeat_interval: repeats ? values.repeat_interval : 1,
          })
        } catch (err) {
          // The host toasts a summary and re-throws; map field-level 422s.
          applyServerFieldErrors(err, setError, {
            fields: ["title", "due_on", "remind_days_before", "repeat_unit", "repeat_interval"],
          })
        }
      })}
    >
      <DialogHeader>
        <DialogTitle>
          {isEdit ? t("reminders:dialog.editTitle") : t("reminders:dialog.createTitle")}
        </DialogTitle>
        <DialogDescription>{t("reminders:dialog.description")}</DialogDescription>
      </DialogHeader>

      <div className="space-y-1.5">
        <Label htmlFor="reminder-title">{t("reminders:dialog.titleLabel")}</Label>
        <Input
          id="reminder-title"
          placeholder={t("reminders:dialog.titlePlaceholder")}
          data-testid="reminder-title-input"
          aria-invalid={!!errors.title}
          aria-describedby={errors.title?.message ? "reminder-title-error" : undefined}
          {...register("title")}
        />
        <FieldError id="reminder-title-error" message={errors.title?.message} />
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="reminder-date">{t("reminders:dialog.dateLabel")}</Label>
          <Input
            id="reminder-date"
            type="date"
            data-testid="reminder-date-input"
            aria-invalid={!!errors.due_on}
            aria-describedby={errors.due_on?.message ? "reminder-date-error" : undefined}
            {...register("due_on")}
          />
          <FieldError id="reminder-date-error" message={errors.due_on?.message} />
        </div>
        <div className="space-y-1.5">
          <Label htmlFor="reminder-leads">{t("reminders:dialog.leadTimesLabel")}</Label>
          <Input
            id="reminder-leads"
            inputMode="numeric"
            data-testid="reminder-leads-input"
            aria-invalid={!!errors.remind_days_before}
            aria-describedby={
              errors.remind_days_before?.message ? "reminder-leads-error" : "reminder-leads-hint"
            }
            {...register("remind_days_before")}
          />
          {errors.remind_days_before ? (
            <FieldError id="reminder-leads-error" message={errors.remind_days_before.message} />
          ) : (
            <p id="reminder-leads-hint" className="text-xs text-muted-foreground">
              {t("reminders:dialog.leadTimesHint")}
            </p>
          )}
        </div>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        <div className="space-y-1.5">
          <Label htmlFor="reminder-repeat">{t("reminders:dialog.repeatLabel")}</Label>
          <Controller
            control={control}
            name="repeat_unit"
            render={({ field }) => (
              <Select
                value={field.value}
                onValueChange={(v) => field.onChange(v as ReminderRepeatUnit)}
              >
                <SelectTrigger
                  id="reminder-repeat"
                  ref={field.ref}
                  className="w-full"
                  aria-label={t("reminders:dialog.repeatLabel")}
                  data-testid="reminder-repeat-select"
                >
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {REMINDER_REPEAT_UNITS.map((unit) => (
                    <SelectItem key={unit} value={unit}>
                      {repeatUnitLabel(t, unit)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          />
        </div>
        {repeatUnit !== "none" ? (
          <div className="space-y-1.5">
            <Label htmlFor="reminder-interval">{intervalLabel(t, repeatUnit)}</Label>
            <Input
              id="reminder-interval"
              type="number"
              min={1}
              max={REMINDER_MAX_INTERVAL}
              inputMode="numeric"
              data-testid="reminder-interval-input"
              aria-invalid={!!errors.repeat_interval}
              aria-describedby={
                errors.repeat_interval?.message ? "reminder-interval-error" : undefined
              }
              {...register("repeat_interval")}
            />
            <FieldError id="reminder-interval-error" message={errors.repeat_interval?.message} />
          </div>
        ) : null}
      </div>

      <DialogFooter>
        <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
          {t("common:actions.cancel")}
        </Button>
        <Button type="submit" disabled={busy} data-testid="reminder-submit">
          {isEdit ? t("common:actions.save") : t("common:actions.create")}
        </Button>
      </DialogFooter>
    </form>
  )
}

type TFn = ReturnType<typeof useTranslation>["t"]

function intervalLabel(t: TFn, unit: ReminderRepeatUnit): string {
  switch (unit) {
    case "week":
      return t("reminders:dialog.intervalLabel.week")
    case "month":
      return t("reminders:dialog.intervalLabel.month")
    case "year":
      return t("reminders:dialog.intervalLabel.year")
    default:
      return t("reminders:dialog.intervalLabel.day")
  }
}

// One literal t() call per unit so the i18n extractor sees every key.
export function repeatUnitLabel(t: TFn, unit: ReminderRepeatUnit): string {
  switch (unit) {
    case "day":
      return t("reminders:repeat.day")
    case "week":
      return t("reminders:repeat.week")
    case "month":
      return t("reminders:repeat.month")
    case "year":
      return t("reminders:repeat.year")
    default:
      return t("reminders:repeat.none")
  }
}
