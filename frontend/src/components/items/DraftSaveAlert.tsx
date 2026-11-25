import { useTranslation } from "react-i18next"
import { AlertTriangle, CloudOff, RefreshCw } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"

// The state of saving the item form as a server draft (#2812).
export type DraftSaveState =
  | { kind: "idle" }
  | { kind: "saving" }
  // The save failed; the values stay in the form and on this device.
  // failedFiles names the files that did not reach the draft.
  | { kind: "error"; message: string; failedFiles: string[] }
  // The draft was saved elsewhere after this form loaded it.
  | { kind: "conflict" }
  // The draft became an item elsewhere; commodityId names it when known.
  | { kind: "promoted"; commodityId?: string }
  // The draft was discarded elsewhere.
  | { kind: "discarded" }

interface DraftSaveAlertProps {
  state: DraftSaveState
  onRetry: () => void
  onLoadNewer: () => void
  onSaveAsNew: () => void
  onOpenItem: (commodityId: string) => void
}

// DraftSaveAlert reports a server draft save that did not succeed and
// offers the way out: retry a failed save, settle a conflict with a save
// made on another device or tab, or keep the input as a new draft when the
// draft is gone.
export function DraftSaveAlert({
  state,
  onRetry,
  onLoadNewer,
  onSaveAsNew,
  onOpenItem,
}: DraftSaveAlertProps) {
  const { t } = useTranslation()
  const saveAsNew = (
    <Button
      type="button"
      size="sm"
      variant="outline"
      onClick={onSaveAsNew}
      data-testid="draft-conflict-save-new"
    >
      {t("commodities:drafts.conflictSaveNew")}
    </Button>
  )
  if (state.kind === "promoted") {
    const { commodityId } = state
    return (
      <Alert variant="destructive" data-testid="draft-save-promoted">
        <AlertTriangle className="size-4" aria-hidden="true" />
        <AlertTitle>{t("commodities:drafts.promotedTitle")}</AlertTitle>
        <AlertDescription>
          <p>{t("commodities:drafts.promotedSaveBody")}</p>
          <div className="mt-2 flex flex-wrap gap-2">
            {commodityId ? (
              <Button
                type="button"
                size="sm"
                variant="outline"
                onClick={() => onOpenItem(commodityId)}
                data-testid="draft-promoted-open-item"
              >
                {t("commodities:drafts.openItem")}
              </Button>
            ) : null}
            {saveAsNew}
          </div>
        </AlertDescription>
      </Alert>
    )
  }
  if (state.kind === "discarded") {
    return (
      <Alert variant="destructive" data-testid="draft-save-discarded">
        <AlertTriangle className="size-4" aria-hidden="true" />
        <AlertTitle>{t("commodities:drafts.discardedTitle")}</AlertTitle>
        <AlertDescription>
          <p>{t("commodities:drafts.discardedBody")}</p>
          <div className="mt-2 flex flex-wrap gap-2">{saveAsNew}</div>
        </AlertDescription>
      </Alert>
    )
  }
  if (state.kind === "error") {
    return (
      <Alert variant="destructive" data-testid="draft-save-error">
        <CloudOff className="size-4" aria-hidden="true" />
        <AlertTitle>{t("commodities:drafts.saveFailedTitle")}</AlertTitle>
        <AlertDescription>
          <p>{state.message}</p>
          {state.failedFiles.length > 0 ? (
            <p>{t("commodities:drafts.filesNotSaved", { names: state.failedFiles.join(", ") })}</p>
          ) : null}
          <p>{t("commodities:drafts.keptOnDevice")}</p>
          <Button
            type="button"
            size="sm"
            variant="outline"
            onClick={onRetry}
            className="mt-2"
            data-testid="draft-save-retry"
          >
            <RefreshCw className="size-3.5" aria-hidden="true" />
            {t("common:serverError.retry")}
          </Button>
        </AlertDescription>
      </Alert>
    )
  }
  if (state.kind === "conflict") {
    return (
      <Alert variant="destructive" data-testid="draft-save-conflict">
        <AlertTriangle className="size-4" aria-hidden="true" />
        <AlertTitle>{t("commodities:drafts.conflictTitle")}</AlertTitle>
        <AlertDescription>
          <p>{t("commodities:drafts.conflictBody")}</p>
          <div className="mt-2 flex flex-wrap gap-2">
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={onLoadNewer}
              data-testid="draft-conflict-load"
            >
              {t("commodities:drafts.conflictLoad")}
            </Button>
            {saveAsNew}
          </div>
        </AlertDescription>
      </Alert>
    )
  }
  return null
}
