import { zodResolver } from "@hookform/resolvers/zod"
import { useForm } from "react-hook-form"
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
import { Textarea } from "@/components/ui/textarea"
import { FieldError } from "@/components/FieldError"
import type { MaintenanceScheduleEntity } from "@/features/maintenance/api"
import { useValueWhileOpen } from "@/hooks/useValueWhileOpen"
import { applyServerFieldErrors } from "@/lib/form-errors"
import {
  maintenanceFormSchema,
  type MaintenanceFormInput,
  type MaintenanceFormOutput,
} from "@/features/maintenance/schemas"

// MaintenanceScheduleValues is the normalised payload the dialog
// hands to its parent — every field non-undefined string / number so
// the parent can spread it onto either a Create (commodity_id
// required) or an Update (every field optional) without TS gymnastics.
// Empty strings become undefined at the call site; the dialog itself
// always returns the validated shape.
export interface MaintenanceScheduleValues {
  title: string
  interval_days: number
  next_due_at?: string
  notes?: string
}

interface MaintenanceScheduleDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  commodityId: string
  initial: (MaintenanceScheduleEntity & { id: string }) | null
  onSubmit: (values: MaintenanceScheduleValues) => void | Promise<void>
  submitting: boolean
}

// MaintenanceScheduleDialog is the create/edit form for a maintenance
// schedule (#1368). Backed by react-hook-form + zodResolver per the
// project standard (mirrors LendDialog / SendForServiceDialog /
// TagFormDialog). The form lives in the dialog content, which mounts on
// open and unmounts once the close animation ends, and is keyed on
// initial?.id, so each open cycle / create→edit switch gets a fresh
// useForm pass with the right defaultValues — that avoids the
// cascading-setState-in-useEffect anti-pattern flagged by the
// react-hooks/set-state-in-effect lint rule. While the dialog closes it
// keeps the schedule it showed: the parent clears `initial` on close.
export function MaintenanceScheduleDialog({
  open,
  onOpenChange,
  initial,
  onSubmit,
  submitting,
}: MaintenanceScheduleDialogProps) {
  const shown = useValueWhileOpen(open, initial)
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent variant="fullscreen-mobile">
        <MaintenanceScheduleForm
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

interface MaintenanceScheduleFormProps {
  initial: MaintenanceScheduleDialogProps["initial"]
  onOpenChange: MaintenanceScheduleDialogProps["onOpenChange"]
  onSubmit: MaintenanceScheduleDialogProps["onSubmit"]
  submitting: boolean
}

function buildDefaults(initial: MaintenanceScheduleDialogProps["initial"]): MaintenanceFormInput {
  return {
    title: initial?.title ?? "",
    // react-hook-form's number registration prefers the string form
    // — zod's `coerce.number()` parses it back to a Number at submit.
    interval_days: (initial?.interval_days ?? 90) as unknown as number,
    next_due_at: initial?.next_due_at ?? "",
    notes: initial?.notes ?? "",
  }
}

function MaintenanceScheduleForm({
  initial,
  onOpenChange,
  onSubmit,
  submitting,
}: MaintenanceScheduleFormProps) {
  const { t } = useTranslation(["maintenance", "common"])
  const {
    formState: { errors, isSubmitting },
    handleSubmit,
    register,
    setError,
  } = useForm<MaintenanceFormInput, unknown, MaintenanceFormOutput>({
    resolver: zodResolver(maintenanceFormSchema),
    defaultValues: buildDefaults(initial),
  })

  const isEdit = !!initial
  const busy = submitting || isSubmitting

  return (
    <form
      className="space-y-4"
      // noValidate: zod owns validation; matches LendDialog so the
      // browser's HTML5 validator can't silently block submission
      // on a <input type="date"> with an edge value.
      noValidate
      onSubmit={handleSubmit(async (values) => {
        try {
          await onSubmit({
            title: values.title,
            interval_days: values.interval_days,
            next_due_at: values.next_due_at ? values.next_due_at : undefined,
            notes: values.notes ? values.notes : undefined,
          })
        } catch (err) {
          // Host toasts a summary and re-throws; map field-level 422s.
          applyServerFieldErrors(err, setError, {
            fields: ["title", "interval_days", "next_due_at", "notes"],
          })
        }
      })}
    >
      <DialogHeader>
        <DialogTitle>
          {isEdit
            ? t("maintenance:dialog.editTitle", { defaultValue: "Edit maintenance schedule" })
            : t("maintenance:dialog.createTitle", { defaultValue: "Add maintenance schedule" })}
        </DialogTitle>
        <DialogDescription>
          {t("maintenance:dialog.description", {
            defaultValue:
              "Set the cadence and a starting due date. We'll email reminders 14, 7, and 1 day before each cycle.",
          })}
        </DialogDescription>
      </DialogHeader>
      <div className="space-y-2">
        <Label htmlFor="maintenance-title">
          {t("maintenance:dialog.titleLabel", { defaultValue: "Title" })}
        </Label>
        <Input
          id="maintenance-title"
          placeholder={t("maintenance:dialog.titlePlaceholder", {
            defaultValue: "Replace water filter",
          })}
          data-testid="maintenance-title-input"
          aria-invalid={!!errors.title}
          aria-describedby={errors.title ? "maintenance-title-error" : undefined}
          {...register("title")}
        />
        <FieldError
          id="maintenance-title-error"
          testId="maintenance-title-error"
          message={errors.title?.message}
        />
      </div>
      <div className="grid grid-cols-2 gap-3">
        <div className="space-y-2">
          <Label htmlFor="maintenance-interval">
            {t("maintenance:dialog.intervalLabel", { defaultValue: "Every (days)" })}
          </Label>
          <Input
            id="maintenance-interval"
            type="number"
            min={1}
            max={36500}
            inputMode="numeric"
            data-testid="maintenance-interval-input"
            aria-invalid={!!errors.interval_days}
            aria-describedby={errors.interval_days ? "maintenance-interval-error" : undefined}
            {...register("interval_days")}
          />
          <FieldError
            id="maintenance-interval-error"
            testId="maintenance-interval-error"
            message={errors.interval_days?.message}
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="maintenance-next-due">
            {t("maintenance:dialog.nextDueLabel", { defaultValue: "Next due (optional)" })}
          </Label>
          <Input
            id="maintenance-next-due"
            type="date"
            data-testid="maintenance-next-due-input"
            aria-invalid={!!errors.next_due_at}
            aria-describedby={errors.next_due_at ? "maintenance-next-due-error" : undefined}
            {...register("next_due_at")}
          />
          <FieldError
            id="maintenance-next-due-error"
            testId="maintenance-next-due-error"
            message={errors.next_due_at?.message}
          />
        </div>
      </div>
      <div className="space-y-2">
        <Label htmlFor="maintenance-notes">
          {t("maintenance:dialog.notesLabel", { defaultValue: "Notes" })}
        </Label>
        <Textarea
          id="maintenance-notes"
          rows={3}
          placeholder={t("maintenance:dialog.notesPlaceholder", {
            defaultValue: "Use NSF-53 filter, comes in 2-packs",
          })}
          aria-invalid={!!errors.notes}
          aria-describedby={errors.notes ? "maintenance-notes-error" : undefined}
          {...register("notes")}
        />
        <FieldError
          id="maintenance-notes-error"
          testId="maintenance-notes-error"
          message={errors.notes?.message}
        />
      </div>
      <DialogFooter>
        <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
          {t("common:actions.cancel", { defaultValue: "Cancel" })}
        </Button>
        <Button type="submit" disabled={busy} data-testid="maintenance-submit">
          {isEdit
            ? t("common:actions.save", { defaultValue: "Save" })
            : t("common:actions.create", { defaultValue: "Create" })}
        </Button>
      </DialogFooter>
    </form>
  )
}
