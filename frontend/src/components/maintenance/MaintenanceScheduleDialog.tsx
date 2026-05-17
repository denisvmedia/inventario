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
import { Textarea } from "@/components/ui/textarea"
import type { MaintenanceScheduleEntity } from "@/features/maintenance/api"

// MaintenanceScheduleValues is the create / edit form's payload — a
// stricter shape than the BE-facing request types so the parent can
// spread it onto either a Create (commodity_id required) or an Update
// (every field optional) without TS complaints. Title + interval are
// required by the form's HTML validation, so they're never empty here.
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
// schedule. Mirrors the LendDialog shape — modal with the fields the
// schedule needs, no heavy form library for v1.
//
// The actual form is split into a child component keyed on
// (open, initial?.id) so each open cycle / create→edit switch gets a
// fresh useState pass with the right defaults. That avoids the
// cascading-setState-in-useEffect anti-pattern rejected by the
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

function MaintenanceScheduleForm({
  initial,
  onOpenChange,
  onSubmit,
  submitting,
}: MaintenanceScheduleFormProps) {
  const { t } = useTranslation(["maintenance", "common"])
  const [title, setTitle] = useState(initial?.title ?? "")
  const [intervalDays, setIntervalDays] = useState(String(initial?.interval_days ?? 90))
  const [nextDueAt, setNextDueAt] = useState(initial?.next_due_at ?? "")
  const [notes, setNotes] = useState(initial?.notes ?? "")

  const isEdit = !!initial

  function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    const intervalNum = Number.parseInt(intervalDays, 10)
    if (!title.trim() || !Number.isFinite(intervalNum) || intervalNum < 1) return
    void onSubmit({
      title: title.trim(),
      interval_days: intervalNum,
      next_due_at: nextDueAt || undefined,
      notes: notes.trim() || undefined,
    })
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4">
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
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder={t("maintenance:dialog.titlePlaceholder", {
            defaultValue: "Replace water filter",
          })}
          required
          maxLength={200}
          data-testid="maintenance-title-input"
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
            value={intervalDays}
            onChange={(e) => setIntervalDays(e.target.value)}
            required
            data-testid="maintenance-interval-input"
          />
        </div>
        <div className="space-y-2">
          <Label htmlFor="maintenance-next-due">
            {t("maintenance:dialog.nextDueLabel", { defaultValue: "Next due (optional)" })}
          </Label>
          <Input
            id="maintenance-next-due"
            type="date"
            value={nextDueAt}
            onChange={(e) => setNextDueAt(e.target.value)}
            data-testid="maintenance-next-due-input"
          />
        </div>
      </div>
      <div className="space-y-2">
        <Label htmlFor="maintenance-notes">
          {t("maintenance:dialog.notesLabel", { defaultValue: "Notes" })}
        </Label>
        <Textarea
          id="maintenance-notes"
          value={notes}
          onChange={(e) => setNotes(e.target.value)}
          rows={3}
          maxLength={1000}
          placeholder={t("maintenance:dialog.notesPlaceholder", {
            defaultValue: "Use NSF-53 filter, comes in 2-packs",
          })}
        />
      </div>
      <DialogFooter>
        <Button type="button" variant="ghost" onClick={() => onOpenChange(false)}>
          {t("common:actions.cancel", { defaultValue: "Cancel" })}
        </Button>
        <Button type="submit" disabled={submitting} data-testid="maintenance-submit">
          {isEdit
            ? t("common:actions.save", { defaultValue: "Save" })
            : t("common:actions.create", { defaultValue: "Create" })}
        </Button>
      </DialogFooter>
    </form>
  )
}
