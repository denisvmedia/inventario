import { ShieldAlert } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { RadioGroup, RadioGroupItem } from "@/components/ui/radio-group"
import { Switch } from "@/components/ui/switch"
import { RESTORE_STRATEGIES, type RestoreStrategy } from "@/features/export/api"
import { cn } from "@/lib/utils"

type Risk = "low" | "medium" | "high"

const RISK_BY_STRATEGY: Record<RestoreStrategy, Risk> = {
  merge_add: "low",
  merge_update: "medium",
  full_replace: "high",
}

const RISK_TONE: Record<Risk, string> = {
  low: "bg-status-active/10 text-status-active",
  medium: "bg-status-expiring/10 text-status-expiring",
  high: "bg-destructive/10 text-destructive",
}

const RISK_LABEL: Record<Risk, "safe" | "moderate" | "destructive"> = {
  low: "safe",
  medium: "moderate",
  high: "destructive",
}

export interface RestoreOptionsFormValue {
  description: string
  strategy: RestoreStrategy
  include_file_data: boolean
  dry_run: boolean
}

export interface RestoreOptionsFormProps {
  value: RestoreOptionsFormValue
  onChange: (value: RestoreOptionsFormValue) => void
  disabled?: boolean
  className?: string
}

// Visual form body shared by ExportRestorePage (full standalone route)
// and RestoreDialog (in-context). Owns the input layout but not the
// submit chrome — the host wraps it in a <form> or <DialogFooter> and
// decides what the primary CTA looks like.
export function RestoreOptionsForm({
  value,
  onChange,
  disabled,
  className,
}: RestoreOptionsFormProps) {
  const { t } = useTranslation(["exports"])

  function patch(next: Partial<RestoreOptionsFormValue>) {
    onChange({ ...value, ...next })
  }

  return (
    <div className={cn("flex flex-col gap-5", className)}>
      <div className="flex flex-col gap-2">
        <Label className="text-sm font-medium">{t("exports:restore.strategy")}</Label>
        <RadioGroup
          value={value.strategy}
          onValueChange={(next) => patch({ strategy: next as RestoreStrategy })}
          disabled={disabled}
        >
          {RESTORE_STRATEGIES.map((strategy) => {
            const risk = RISK_BY_STRATEGY[strategy]
            const selected = value.strategy === strategy
            const id = `restore-strategy-input-${strategy}`
            return (
              <label
                key={strategy}
                htmlFor={id}
                data-testid={`restore-strategy-${strategy}`}
                className={cn(
                  "flex cursor-pointer items-start gap-3 rounded-lg border p-4 transition-colors",
                  selected
                    ? "border-primary bg-primary/5"
                    : "border-border hover:border-primary/30",
                  disabled && "cursor-not-allowed opacity-60"
                )}
              >
                <RadioGroupItem id={id} value={strategy} className="mt-0.5" />
                <div className="flex flex-1 flex-col gap-0.5">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-semibold">
                      {t(`exports:restore.strategyLabel.${strategy}`)}
                    </span>
                    <span
                      className={cn(
                        "rounded-full px-1.5 py-0.5 text-[10px] font-medium",
                        RISK_TONE[risk]
                      )}
                    >
                      {t(`exports:restore.riskLabel.${RISK_LABEL[risk]}`)}
                    </span>
                  </div>
                  <p className="text-xs text-muted-foreground">
                    {t(`exports:restore.strategyDescription.${strategy}`)}
                  </p>
                </div>
              </label>
            )
          })}
        </RadioGroup>
      </div>

      {value.strategy === "full_replace" && !value.dry_run && (
        <div
          className="flex items-start gap-3 rounded-lg border border-destructive/40 bg-destructive/5 p-4"
          data-testid="restore-destructive-warning"
          role="alert"
        >
          <ShieldAlert
            className="mt-0.5 size-4 shrink-0 text-destructive"
            aria-hidden="true"
          />
          <div className="flex flex-col gap-0.5">
            <p className="text-sm font-semibold text-destructive">
              {t("exports:restore.strategyLabel.full_replace")}
            </p>
            <p className="text-sm text-destructive">
              {t("exports:restore.strategyDescription.full_replace")}
            </p>
          </div>
        </div>
      )}

      <div className="flex items-center justify-between rounded-lg border bg-muted/40 px-4 py-3">
        <div className="flex flex-col gap-0.5">
          <p className="text-sm font-medium">{t("exports:restore.includeFileData")}</p>
          <p className="text-xs text-muted-foreground">
            {t("exports:restore.includeFileDataDescription")}
          </p>
        </div>
        <Switch
          checked={value.include_file_data}
          onCheckedChange={(checked) => patch({ include_file_data: checked })}
          disabled={disabled}
          data-testid="restore-include-file-data"
          aria-label={t("exports:restore.includeFileData")}
        />
      </div>

      <div className="flex items-center justify-between rounded-lg border bg-muted/40 px-4 py-3">
        <div className="flex flex-col gap-0.5">
          <p className="text-sm font-medium">{t("exports:restore.dryRunLabel")}</p>
          <p className="text-xs text-muted-foreground">
            {t("exports:restore.dryRunDescription")}
          </p>
        </div>
        <Switch
          checked={value.dry_run}
          onCheckedChange={(checked) => patch({ dry_run: checked })}
          disabled={disabled}
          data-testid="restore-dry-run"
          aria-label={t("exports:restore.dryRunLabel")}
        />
      </div>

      <div className="flex flex-col gap-2">
        <Label htmlFor="restore-description">{t("exports:restore.description")}</Label>
        <Input
          id="restore-description"
          value={value.description}
          onChange={(e) => patch({ description: e.target.value })}
          placeholder={t("exports:restore.descriptionPlaceholder")}
          maxLength={500}
          minLength={1}
          required
          disabled={disabled}
          data-testid="restore-description"
        />
        <p className="text-xs text-muted-foreground">{t("exports:restore.descriptionHint")}</p>
      </div>
    </div>
  )
}
