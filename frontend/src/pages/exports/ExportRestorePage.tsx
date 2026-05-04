import { ArrowLeft, Loader2, ShieldAlert } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useParams } from "react-router-dom"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { RESTORE_STRATEGIES, type RestoreStrategy } from "@/features/export/api"
import { useCreateRestore, useExport } from "@/features/export/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { cn } from "@/lib/utils"

interface FormState {
  description: string
  strategy: RestoreStrategy
  include_file_data: boolean
  dry_run: boolean
}

const defaultState: FormState = {
  description: "",
  strategy: "merge_add",
  include_file_data: true,
  dry_run: true,
}

export function ExportRestorePage() {
  const { t } = useTranslation(["exports", "common"])
  const params = useParams()
  const navigate = useNavigate()
  const toast = useAppToast()
  const { currentGroup } = useCurrentGroup()
  const groupReady = !!currentGroup
  const slug = currentGroup?.slug ?? ""
  const exportId = params.id ?? ""

  const exportQuery = useExport(exportId, { enabled: groupReady && !!exportId })
  const createRestoreMutation = useCreateRestore()
  const [state, setState] = useState<FormState>(defaultState)

  const exp = exportQuery.data
  const detailHref = `/g/${encodeURIComponent(slug)}/exports/${encodeURIComponent(exportId)}`

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    try {
      await createRestoreMutation.mutateAsync({
        exportId,
        req: {
          description: state.description,
          options: {
            strategy: state.strategy,
            include_file_data: state.include_file_data,
            dry_run: state.dry_run,
          },
        },
      })
      toast.success(
        state.dry_run ? t("exports:restore.successDryRun") : t("exports:restore.success")
      )
      navigate(detailHref)
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      toast.error(t("exports:errors.restoreCreateFailed", { error: message }))
    }
  }

  if (!groupReady || exportQuery.isLoading) {
    return (
      <div className="flex flex-col gap-4 p-6" data-testid="page-export-restore-loading">
        <Skeleton className="h-8 w-1/2" />
        <Skeleton className="h-64 w-full" />
      </div>
    )
  }

  if (!exp) {
    return (
      <div className="flex flex-col gap-4 p-6" data-testid="page-export-restore-not-found">
        <Alert variant="destructive">
          <AlertTitle>{t("exports:detail.notFound")}</AlertTitle>
        </Alert>
        <Button asChild variant="outline" className="self-start">
          <Link to={`/g/${encodeURIComponent(slug)}/exports`}>{t("exports:list.title")}</Link>
        </Button>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6 p-6" data-testid="page-export-restore">
      <header className="flex flex-col gap-2">
        <Button asChild variant="link" className="self-start px-0">
          <Link to={detailHref}>
            <ArrowLeft className="mr-1.5 size-4" aria-hidden="true" />
            {t("exports:detail.title")}
          </Link>
        </Button>
        <h1 className="text-2xl font-semibold tracking-tight">{t("exports:restore.title")}</h1>
        <p className="max-w-prose text-sm text-muted-foreground">{t("exports:restore.intro")}</p>
      </header>

      {state.strategy === "full_replace" && !state.dry_run && (
        <Alert variant="destructive" data-testid="restore-destructive-warning">
          <ShieldAlert className="size-4" aria-hidden="true" />
          <AlertTitle>{t("exports:restore.strategyLabel.full_replace")}</AlertTitle>
          <AlertDescription>
            {t("exports:restore.strategyDescription.full_replace")}
          </AlertDescription>
        </Alert>
      )}

      <form onSubmit={onSubmit} className="flex flex-col gap-5" data-testid="restore-form">
        <fieldset className="flex flex-col gap-3">
          <legend className="text-sm font-medium">{t("exports:restore.strategy")}</legend>
          {RESTORE_STRATEGIES.map((strategy) => {
            const id = `restore-strategy-${strategy}`
            return (
              // eslint-disable-next-line jsx-a11y/label-has-associated-control -- the strategy label below carries the visible text; the rule's text-traversal misses it because t() returns a string at runtime, not a literal at parse time.
              <label
                key={strategy}
                htmlFor={id}
                className={cn(
                  "flex cursor-pointer items-start gap-3 rounded-md border bg-card px-4 py-3",
                  state.strategy === strategy && "border-primary/60 bg-primary/5"
                )}
                data-testid={id}
              >
                <input
                  id={id}
                  type="radio"
                  name="strategy"
                  className="mt-1 size-4"
                  checked={state.strategy === strategy}
                  onChange={() => setState((prev) => ({ ...prev, strategy }))}
                  value={strategy}
                />
                <span className="flex flex-col gap-0.5">
                  <span className="text-sm font-medium">
                    {t(`exports:restore.strategyLabel.${strategy}`)}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {t(`exports:restore.strategyDescription.${strategy}`)}
                  </span>
                </span>
              </label>
            )
          })}
        </fieldset>

        <label className="inline-flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            className="size-4"
            checked={state.include_file_data}
            onChange={(e) => setState((prev) => ({ ...prev, include_file_data: e.target.checked }))}
            data-testid="restore-include-file-data"
          />
          {t("exports:restore.includeFileData")}
        </label>

        <label className="inline-flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            className="size-4"
            checked={state.dry_run}
            onChange={(e) => setState((prev) => ({ ...prev, dry_run: e.target.checked }))}
            data-testid="restore-dry-run"
          />
          {t("exports:restore.dryRun")}
        </label>

        <div className="flex flex-col gap-2">
          <Label htmlFor="restore-description">{t("exports:restore.description")}</Label>
          <Input
            id="restore-description"
            value={state.description}
            onChange={(e) => setState((prev) => ({ ...prev, description: e.target.value }))}
            placeholder={t("exports:restore.descriptionPlaceholder")}
            maxLength={500}
            data-testid="restore-description"
          />
          <p className="text-xs text-muted-foreground">{t("exports:restore.descriptionHint")}</p>
        </div>

        <div className="flex justify-end gap-2">
          <Button asChild variant="ghost" type="button">
            <Link to={detailHref}>{t("exports:wizard.cancel")}</Link>
          </Button>
          <Button
            type="submit"
            disabled={createRestoreMutation.isPending}
            data-testid="restore-submit"
          >
            {createRestoreMutation.isPending && (
              <Loader2 className="mr-1.5 size-4 animate-spin" aria-hidden="true" />
            )}
            {createRestoreMutation.isPending
              ? t("exports:restore.submitting")
              : t("exports:restore.submit")}
          </Button>
        </div>
      </form>
    </div>
  )
}
