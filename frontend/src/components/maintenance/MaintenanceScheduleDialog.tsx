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
import type { MaintenanceScheduleEntity } from "@/features/maintenance/api"
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
// TagFormDialog). The form lives in a child keyed on (open,
// initial?.id) so each open cycle / create→edit switch gets a fresh
// useForm pass with the right defaultValues — that avoids the
// cascading-setState-in-useEffect anti-pattern flagged by the
// react-hooks/set-state-in-effect lint rule.
export function MaintenanceScheduleDialog({
  open,
  onOpenChange,
  initial,
  onSubmit,
  submitting,
}: MaintenanceScheduleDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        {open ? (
          <MaintenanceScheduleForm
            key={initial?.id ?? "create"}
            initial={initial}
            onOpenChange={onOpenChange}
            onSubmit={onSubmit}
            submitting={submitting}
          />
        ) : null}
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
          maxLength={200}
          data-testid="maintenance-title-input"
          aria-invalid={!!errors.title}
          {...register("title")}
        />
        {errors.title?.message ? (
          <p className="text-xs text-destructive">{t(errors.title.message)}</p>
        ) : null}
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
            {...register("interval_days")}
          />
          {errors.interval_days?.message ? (
            <p className="text-xs text-destructive">{t(errors.interval_days.message)}</p>
          ) : null}
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
            {...register("next_due_at")}
          />
          {errors.next_due_at?.message ? (
            <p className="text-xs text-destructive">{t(errors.next_due_at.message)}</p>
          ) : null}
        </div>
      </div>
      <div className="space-y-2">
        <Label htmlFor="maintenance-notes">
          {t("maintenance:dialog.notesLabel", { defaultValue: "Notes" })}
        </Label>
        <Textarea
          id="maintenance-notes"
          rows={3}
          maxLength={1000}
          placeholder={t("maintenance:dialog.notesPlaceholder", {
            defaultValue: "Use NSF-53 filter, comes in 2-packs",
          })}
          aria-invalid={!!errors.notes}
          {...register("notes")}
        />
        {errors.notes?.message ? (
          <p className="text-xs text-destructive">{t(errors.notes.message)}</p>
        ) : null}
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
