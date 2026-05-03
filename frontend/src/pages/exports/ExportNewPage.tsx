import { ArrowLeft, ArrowRight, CheckCircle2, Loader2, XCircle } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useSearchParams } from "react-router-dom"

import { ExportStatusBadge } from "@/components/exports/ExportStatusBadge"
import { SelectedItemsPicker } from "@/components/exports/SelectedItemsPicker"
import { Alert, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  type ExportSelectedItem,
  type ExportType,
  exportDownloadPath,
  isExportTerminal,
} from "@/features/export/api"
import { useCreateExport, useExport } from "@/features/export/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { cn } from "@/lib/utils"

type WizardStep = 1 | 2 | 3

function parseStep(raw: string | null): WizardStep {
  if (raw === "2") return 2
  if (raw === "3") return 3
  return 1
}

interface WizardState {
  type: "full_database" | "selected_items"
  description: string
  include_file_data: boolean
  selected_items: ExportSelectedItem[]
}

const initialState: WizardState = {
  type: "full_database",
  description: "",
  include_file_data: true,
  selected_items: [],
}

export function ExportNewPage() {
  const { t } = useTranslation(["exports", "common"])
  const toast = useAppToast()
  const [searchParams, setSearchParams] = useSearchParams()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""

  const step = parseStep(searchParams.get("step"))
  const createdId = searchParams.get("id") ?? undefined

  const [state, setState] = useState<WizardState>(initialState)
  const [scopeError, setScopeError] = useState<string | null>(null)

  const createMutation = useCreateExport()
  // Once we've created an export, the wizard hangs on step 3 polling its
  // status. We only enable the query when an id is in the URL — that
  // way reloading on step 1/2 doesn't fire a /exports/{id} request.
  const exportQuery = useExport(createdId, { enabled: !!createdId })

  // Functional updater so the closure-captured `searchParams` from a stale
  // render can't drop the freshly-set step+id when this fires from a
  // mutation callback after `await`.
  function patchStep(next: WizardStep, extra: Record<string, string> = {}) {
    setSearchParams(
      (prev) => {
        const params = new URLSearchParams(prev)
        params.set("step", String(next))
        for (const [k, v] of Object.entries(extra)) params.set(k, v)
        return params
      },
      { replace: true }
    )
  }

  function onScopeNext() {
    if (state.type === "selected_items" && state.selected_items.length === 0) {
      setScopeError(t("exports:validation.selectedItemsRequired"))
      return
    }
    setScopeError(null)
    patchStep(2)
  }

  function onConfirmSubmit() {
    createMutation.mutate(
      {
        type: state.type as ExportType,
        description: state.description,
        include_file_data: state.include_file_data,
        selected_items: state.type === "selected_items" ? state.selected_items : undefined,
      },
      {
        onSuccess: (created) => {
          if (!created.id) {
            toast.error(t("exports:errors.createFailed", { error: "missing id" }))
            return
          }
          patchStep(3, { id: created.id })
        },
        onError: (err) => {
          const message = err instanceof Error ? err.message : String(err)
          toast.error(t("exports:errors.createFailed", { error: message }))
        },
      }
    )
  }

  return (
    <div className="flex flex-col gap-6 p-6" data-testid="page-export-new">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-1.5">
          <h1 className="text-2xl font-semibold tracking-tight">{t("exports:wizard.title")}</h1>
          <p className="max-w-prose text-sm text-muted-foreground">{t("exports:wizard.intro")}</p>
        </div>
        <Button asChild variant="ghost" size="sm">
          <Link to={`/g/${encodeURIComponent(slug)}/exports`}>{t("exports:wizard.cancel")}</Link>
        </Button>
      </header>

      <WizardSteps step={step} />

      {step === 1 && (
        <Step1
          state={state}
          setState={setState}
          errorMessage={scopeError ?? undefined}
          onNext={onScopeNext}
        />
      )}

      {step === 2 && (
        <Step2
          state={state}
          setState={setState}
          isPending={createMutation.isPending}
          onBack={() => patchStep(1)}
          onSubmit={onConfirmSubmit}
        />
      )}

      {step === 3 && <Step3 createdId={createdId} exportQuery={exportQuery} groupSlug={slug} />}
    </div>
  )
}

function WizardSteps({ step }: { step: WizardStep }) {
  const { t } = useTranslation(["exports"])
  const items: Array<{ index: WizardStep; titleKey: string }> = [
    { index: 1, titleKey: "exports:wizard.step1Title" },
    { index: 2, titleKey: "exports:wizard.step2Title" },
    { index: 3, titleKey: "exports:wizard.step3Title" },
  ]
  return (
    <ol
      className="flex flex-wrap items-center gap-2 text-sm"
      data-testid="wizard-steps"
      aria-label="wizard"
    >
      {items.map((item, idx) => {
        const active = item.index === step
        const done = item.index < step
        return (
          <li key={item.index} className="flex items-center gap-2">
            <span
              className={cn(
                "inline-flex size-6 items-center justify-center rounded-full border text-xs font-semibold",
                active && "border-primary bg-primary text-primary-foreground",
                done && !active && "border-primary/40 bg-primary/10 text-primary",
                !active && !done && "border-muted text-muted-foreground"
              )}
              data-testid={`wizard-step-${item.index}`}
              data-active={active || undefined}
            >
              {item.index}
            </span>
            <span className={cn("text-sm", active ? "font-medium" : "text-muted-foreground")}>
              {t(item.titleKey)}
            </span>
            {idx < items.length - 1 && (
              <span aria-hidden="true" className="px-2 text-muted-foreground">
                /
              </span>
            )}
          </li>
        )
      })}
    </ol>
  )
}

interface Step1Props {
  state: WizardState
  setState: (next: WizardState) => void
  errorMessage?: string
  onNext: () => void
}

function Step1({ state, setState, errorMessage, onNext }: Step1Props) {
  const { t } = useTranslation(["exports"])
  return (
    <section className="flex flex-col gap-5" data-testid="wizard-step-1-content">
      <fieldset className="flex flex-col gap-3">
        <legend className="mb-2 text-sm font-medium">{t("exports:wizard.step1Title")}</legend>
        <ScopeRadio
          value={state.type}
          onChange={(type) => setState({ ...state, type })}
          option="full_database"
          titleKey="exports:wizard.scope.fullDatabase"
          hintKey="exports:wizard.scope.fullDatabaseHint"
        />
        <ScopeRadio
          value={state.type}
          onChange={(type) => setState({ ...state, type })}
          option="selected_items"
          titleKey="exports:wizard.scope.selectedItems"
          hintKey="exports:wizard.scope.selectedItemsHint"
        />
      </fieldset>

      {state.type === "selected_items" && (
        <div className="flex flex-col gap-2 rounded-md border bg-muted/20 p-3">
          <p className="text-sm font-medium">{t("exports:wizard.scopePicker.title")}</p>
          <SelectedItemsPicker
            value={state.selected_items}
            onChange={(selected_items) => setState({ ...state, selected_items })}
            errorMessage={errorMessage}
          />
        </div>
      )}

      <label className="inline-flex items-center gap-2 text-sm">
        <input
          type="checkbox"
          className="size-4"
          checked={state.include_file_data}
          onChange={(e) => setState({ ...state, include_file_data: e.target.checked })}
          data-testid="wizard-include-file-data"
        />
        {t("exports:wizard.toggleIncludeFileData")}
      </label>

      <div className="flex justify-end gap-2">
        <Button type="button" onClick={onNext} data-testid="wizard-next">
          {t("exports:wizard.next")}
          <ArrowRight className="ml-1.5 size-4" aria-hidden="true" />
        </Button>
      </div>
    </section>
  )
}

interface ScopeRadioProps {
  value: WizardState["type"]
  onChange: (next: WizardState["type"]) => void
  option: WizardState["type"]
  titleKey: string
  hintKey: string
}

function ScopeRadio({ value, onChange, option, titleKey, hintKey }: ScopeRadioProps) {
  const { t } = useTranslation(["exports"])
  const checked = value === option
  const id = `wizard-scope-${option}`
  return (
    // eslint-disable-next-line jsx-a11y/label-has-associated-control -- the title <span> below carries the visible text; the rule's traversal misses it because t() returns a string at runtime, not a literal at parse time.
    <label
      htmlFor={id}
      className={cn(
        "flex cursor-pointer items-start gap-3 rounded-md border bg-card px-4 py-3",
        checked && "border-primary/60 bg-primary/5"
      )}
      data-testid={id}
    >
      <input
        id={id}
        type="radio"
        className="mt-1 size-4"
        checked={checked}
        onChange={() => onChange(option)}
        name="wizard-scope"
        value={option}
      />
      <span className="flex flex-col gap-0.5">
        <span className="text-sm font-medium">{t(titleKey)}</span>
        <span className="text-xs text-muted-foreground">{t(hintKey)}</span>
      </span>
    </label>
  )
}

interface Step2Props {
  state: WizardState
  setState: (next: WizardState) => void
  isPending: boolean
  onBack: () => void
  onSubmit: () => void
}

function Step2({ state, setState, isPending, onBack, onSubmit }: Step2Props) {
  const { t } = useTranslation(["exports", "common"])
  return (
    <section className="flex flex-col gap-5" data-testid="wizard-step-2-content">
      <div className="flex flex-col gap-2">
        <Label htmlFor="export-description">{t("exports:wizard.summary.description")}</Label>
        <Input
          id="export-description"
          value={state.description}
          onChange={(e) => setState({ ...state, description: e.target.value })}
          placeholder={t("exports:detail.noDescription")}
          maxLength={500}
          data-testid="wizard-description"
        />
      </div>

      <dl
        className="grid gap-4 rounded-md border bg-muted/20 p-4 sm:grid-cols-2"
        data-testid="wizard-summary"
      >
        <div className="flex flex-col gap-1">
          <dt className="text-xs uppercase text-muted-foreground">
            {t("exports:wizard.summary.type")}
          </dt>
          <dd className="text-sm font-medium">
            {state.type === "selected_items"
              ? t("exports:scope.selected_items")
              : t("exports:scope.full_database")}
          </dd>
        </div>
        <div className="flex flex-col gap-1">
          <dt className="text-xs uppercase text-muted-foreground">
            {t("exports:wizard.summary.includeFileData")}
          </dt>
          <dd className="text-sm font-medium">
            {state.include_file_data
              ? t("exports:wizard.summary.includeFileDataYes")
              : t("exports:wizard.summary.includeFileDataNo")}
          </dd>
        </div>
        {state.type === "selected_items" && (
          <div className="sm:col-span-2 flex flex-col gap-1">
            <dt className="text-xs uppercase text-muted-foreground">
              {t("exports:wizard.summary.items")}
            </dt>
            <dd className="text-sm">
              {state.selected_items.length === 0
                ? t("exports:wizard.summary.noDescription")
                : state.selected_items.map((item) => item.name || item.id).join(", ")}
            </dd>
          </div>
        )}
      </dl>

      <div className="flex justify-between gap-2">
        <Button type="button" variant="outline" onClick={onBack} data-testid="wizard-back">
          <ArrowLeft className="mr-1.5 size-4" aria-hidden="true" />
          {t("exports:wizard.back")}
        </Button>
        <Button type="button" onClick={onSubmit} disabled={isPending} data-testid="wizard-submit">
          {isPending ? t("exports:wizard.creating") : t("exports:wizard.submit")}
        </Button>
      </div>
    </section>
  )
}

interface Step3Props {
  createdId: string | undefined
  exportQuery: ReturnType<typeof useExport>
  groupSlug: string
}

function Step3({ createdId, exportQuery, groupSlug }: Step3Props) {
  const { t } = useTranslation(["exports"])
  const exp = exportQuery.data
  const detailHref = useMemo(
    () => (createdId ? `/g/${encodeURIComponent(groupSlug)}/exports/${createdId}` : "#"),
    [createdId, groupSlug]
  )
  const isTerminal = isExportTerminal(exp?.status)
  const isCompleted = exp?.status === "completed"
  const isFailed = exp?.status === "failed"

  if (!createdId) {
    return (
      <Alert variant="destructive" data-testid="wizard-step-3-missing">
        <AlertTitle>{t("exports:errors.createFailed", { error: "missing id" })}</AlertTitle>
      </Alert>
    )
  }

  return (
    <section className="flex flex-col gap-5" data-testid="wizard-step-3-content">
      <div className="flex items-center gap-3 rounded-md border bg-card p-4">
        {!isTerminal && (
          <Loader2
            className="size-5 animate-spin text-primary"
            aria-label={t("exports:status.in_progress")}
          />
        )}
        {isCompleted && (
          <CheckCircle2
            className="size-5 text-emerald-600"
            aria-label={t("exports:status.completed")}
          />
        )}
        {isFailed && (
          <XCircle className="size-5 text-destructive" aria-label={t("exports:status.failed")} />
        )}
        <div className="flex flex-1 flex-col">
          <span className="text-sm font-medium">
            {exp?.description?.trim() ? exp.description : t("exports:wizard.step3Title")}
          </span>
          {exp?.status && <ExportStatusBadge status={exp.status} className="mt-1 self-start" />}
          {isFailed && exp?.error_message && (
            <p className="mt-2 text-sm text-destructive">{exp.error_message}</p>
          )}
        </div>
      </div>

      <div className="flex flex-wrap items-center justify-end gap-2">
        <Button asChild variant="outline">
          <Link to={detailHref}>{t("exports:actions.viewDetail")}</Link>
        </Button>
        {isCompleted && (
          <Button asChild data-testid="wizard-download">
            <a href={exportDownloadPath(createdId, groupSlug)}>{t("exports:actions.download")}</a>
          </Button>
        )}
      </div>
    </section>
  )
}
