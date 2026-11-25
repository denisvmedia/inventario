import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { RotateCcw, Trash2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { formatDate } from "@/lib/intl"

// Pieces shared by the item and file tables of the Trash page.

export function TrashEmptyState({ testId }: { testId: string }) {
  const { t } = useTranslation()
  return (
    <div
      className="rounded-xl border border-border bg-card p-6 text-center space-y-3"
      data-testid={testId}
    >
      <div className="flex justify-center">
        <div className="flex size-12 items-center justify-center rounded-full bg-muted">
          <Trash2 className="size-5 text-muted-foreground" aria-hidden="true" />
        </div>
      </div>
      <div className="space-y-1">
        <p className="text-sm font-semibold">{t("trash:empty.title")}</p>
        <p className="text-xs text-muted-foreground leading-relaxed">{t("trash:empty.body")}</p>
      </div>
    </div>
  )
}

// DeletedCell shows when an entry went to Trash and who deleted it.
export function DeletedCell({ at, by }: { at?: string; by: string }) {
  const { t } = useTranslation()
  const date = at ? formatDate(at) : "—"
  return (
    <div className="flex flex-col">
      <span>{date}</span>
      {by ? (
        <span className="text-xs text-muted-foreground">
          {t("trash:table.deletedBy", { name: by })}
        </span>
      ) : null}
    </div>
  )
}

export function ExpiresCell({ at }: { at: string }) {
  return <span className="text-muted-foreground">{at ? formatDate(at) : "—"}</span>
}

interface RowActionsProps {
  name: string
  canRestore: boolean
  canPurge: boolean
  busy: boolean
  onRestore: () => void
  onPurge: () => void
  testId: string
}

export function RowActions({
  name,
  canRestore,
  canPurge,
  busy,
  onRestore,
  onPurge,
  testId,
}: RowActionsProps) {
  const { t } = useTranslation()
  return (
    <div className="flex justify-end gap-1">
      {canRestore ? (
        <Button
          variant="outline"
          size="sm"
          onClick={onRestore}
          disabled={busy}
          aria-label={t("trash:actions.restoreNamed", { name })}
          data-testid={`${testId}-restore`}
        >
          <RotateCcw className="size-3.5" aria-hidden="true" />
          {t("trash:actions.restore")}
        </Button>
      ) : null}
      {canPurge ? (
        <Button
          variant="outline"
          size="sm"
          onClick={onPurge}
          disabled={busy}
          className="text-destructive hover:bg-destructive/10 hover:text-destructive"
          aria-label={t("trash:actions.purgeNamed", { name })}
          data-testid={`${testId}-purge`}
        >
          <Trash2 className="size-3.5" aria-hidden="true" />
          {t("trash:actions.purge")}
        </Button>
      ) : null}
    </div>
  )
}

interface SelectCellProps {
  checked: boolean
  onChange: (checked: boolean) => void
  label: string
  testId: string
}

export function SelectCell({ checked, onChange, label, testId }: SelectCellProps) {
  return (
    <Checkbox
      checked={checked}
      onCheckedChange={(next) => onChange(next === true)}
      aria-label={label}
      data-testid={testId}
    />
  )
}

// headerCheckState is the state of a select-all checkbox: checked when every
// row is selected, indeterminate when some are.
export function headerCheckState(selected: number, total: number): boolean | "indeterminate" {
  if (selected === 0 || total === 0) return false
  return selected === total ? true : "indeterminate"
}

export function TableShell({ children }: { children: ReactNode }) {
  return <div className="rounded-xl border border-border overflow-hidden">{children}</div>
}
