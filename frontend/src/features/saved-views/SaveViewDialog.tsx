import { useEffect, useState } from "react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
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
import { getServerErrorCode, getServerErrorMeta, parseServerError } from "@/lib/server-error"
import { cn } from "@/lib/utils"

import type { SavedView } from "./api"
import { SavedViewIcon } from "./SavedViewIcon"
import { SAVED_VIEW_ICON_NAMES } from "./view"

export interface SaveViewValues {
  name: string
  icon: string
  shared: boolean
  showInSidebar: boolean
  showOnDashboard: boolean
}

export interface SaveViewDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  mode: "create" | "edit"
  // Prefills the form in edit mode.
  view?: SavedView
  // Whether the member may share views (the user role or higher).
  canShare: boolean
  maxNameLength: number
  // Rejects with the server error, which the dialog shows.
  onSubmit: (values: SaveViewValues) => Promise<void>
}

const DEFAULTS: SaveViewValues = {
  name: "",
  icon: "bookmark",
  shared: false,
  showInSidebar: true,
  showOnDashboard: false,
}

function runeLength(value: string): number {
  return Array.from(value).length
}

// SaveViewDialog names a view and says where it shows. It saves the
// current filter in create mode; in edit mode it changes everything but
// the filter, which is saved from the items list.
export function SaveViewDialog({
  open,
  onOpenChange,
  mode,
  view,
  canShare,
  maxNameLength,
  onSubmit,
}: SaveViewDialogProps) {
  const { t } = useTranslation()
  const [serverError, setServerError] = useState<unknown>(null)
  const {
    control,
    formState: { errors, isSubmitting },
    handleSubmit,
    register,
    reset,
  } = useForm<SaveViewValues>({ defaultValues: DEFAULTS })

  useEffect(() => {
    if (!open) return
    reset(
      mode === "edit" && view
        ? {
            name: view.name,
            icon: view.icon,
            shared: view.shared,
            showInSidebar: view.showInSidebar,
            showOnDashboard: view.showOnDashboard,
          }
        : DEFAULTS
    )
  }, [open, mode, view, reset])

  // A shared view stays shared when a member who may not share edits it;
  // the box only shows what it is.
  const shareDisabled = !canShare

  function handleOpenChange(next: boolean) {
    if (!next) setServerError(null)
    onOpenChange(next)
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md" data-testid="saved-view-dialog">
        <DialogHeader>
          <DialogTitle>
            {mode === "create"
              ? t("commodities:savedViews.dialog.createTitle")
              : t("commodities:savedViews.dialog.editTitle")}
          </DialogTitle>
          <DialogDescription>
            {mode === "create"
              ? t("commodities:savedViews.dialog.createDescription")
              : t("commodities:savedViews.dialog.editDescription")}
          </DialogDescription>
        </DialogHeader>

        <form
          className="flex flex-col gap-4"
          onSubmit={handleSubmit(async (values) => {
            setServerError(null)
            try {
              await onSubmit({ ...values, name: values.name.trim() })
            } catch (err) {
              setServerError(err)
            }
          })}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="saved-view-name">{t("commodities:savedViews.dialog.name")}</Label>
            <Input
              id="saved-view-name"
              data-testid="saved-view-name"
              autoComplete="off"
              placeholder={t("commodities:savedViews.dialog.namePlaceholder")}
              aria-invalid={errors.name ? true : undefined}
              {...register("name", {
                validate: {
                  required: (v) => v.trim() !== "",
                  maxLength: (v) => runeLength(v.trim()) <= maxNameLength,
                },
              })}
            />
            {errors.name?.type === "required" ? (
              <p className="text-xs text-destructive" data-testid="saved-view-name-error">
                {t("commodities:savedViews.dialog.nameRequired")}
              </p>
            ) : errors.name?.type === "maxLength" ? (
              <p className="text-xs text-destructive" data-testid="saved-view-name-error">
                {t("commodities:savedViews.dialog.nameTooLong", { max: maxNameLength })}
              </p>
            ) : null}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label id="saved-view-icon-label">{t("commodities:savedViews.dialog.icon")}</Label>
            <Controller
              control={control}
              name="icon"
              render={({ field }) => (
                <div
                  role="radiogroup"
                  aria-labelledby="saved-view-icon-label"
                  className="grid grid-cols-8 gap-1"
                  data-testid="saved-view-icon"
                >
                  {SAVED_VIEW_ICON_NAMES.map((name) => {
                    const selected = field.value === name
                    return (
                      <button
                        key={name}
                        type="button"
                        role="radio"
                        aria-checked={selected}
                        aria-label={t(`commodities:savedViews.icons.${name}`)}
                        onClick={() => field.onChange(name)}
                        className={cn(
                          "flex size-8 items-center justify-center rounded-md border transition-colors outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50",
                          selected
                            ? "border-primary bg-primary text-primary-foreground"
                            : "border-transparent text-muted-foreground hover:bg-muted"
                        )}
                      >
                        <SavedViewIcon name={name} className="size-4" />
                      </button>
                    )
                  })}
                </div>
              )}
            />
          </div>

          <fieldset className="flex flex-col gap-3">
            <legend className="mb-2 text-sm font-medium">
              {t("commodities:savedViews.dialog.placement")}
            </legend>
            <CheckboxField
              control={control}
              name="showInSidebar"
              label={t("commodities:savedViews.dialog.showInSidebar")}
              testId="saved-view-sidebar"
            />
            <CheckboxField
              control={control}
              name="showOnDashboard"
              label={t("commodities:savedViews.dialog.showOnDashboard")}
              testId="saved-view-dashboard"
            />
            <CheckboxField
              control={control}
              name="shared"
              label={t("commodities:savedViews.dialog.shared")}
              hint={
                shareDisabled
                  ? t("commodities:savedViews.dialog.sharedNeedsRole")
                  : t("commodities:savedViews.dialog.sharedHint")
              }
              disabled={shareDisabled}
              testId="saved-view-shared"
            />
          </fieldset>

          {serverError ? <SaveViewError error={serverError} /> : null}

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => handleOpenChange(false)}
              disabled={isSubmitting}
              data-testid="saved-view-cancel"
            >
              {t("common:actions.cancel")}
            </Button>
            <Button type="submit" disabled={isSubmitting} data-testid="saved-view-submit">
              {mode === "create"
                ? t("commodities:savedViews.dialog.create")
                : t("commodities:savedViews.dialog.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}

interface CheckboxFieldProps {
  control: ReturnType<typeof useForm<SaveViewValues>>["control"]
  name: "shared" | "showInSidebar" | "showOnDashboard"
  label: string
  hint?: string
  disabled?: boolean
  testId: string
}

function CheckboxField({ control, name, label, hint, disabled, testId }: CheckboxFieldProps) {
  const id = `saved-view-${name}`
  return (
    <Controller
      control={control}
      name={name}
      render={({ field }) => (
        <div className="flex items-start gap-2">
          <Checkbox
            id={id}
            checked={field.value}
            onCheckedChange={(v) => field.onChange(v === true)}
            disabled={disabled}
            data-testid={testId}
            className="mt-0.5"
          />
          <div className="flex flex-col gap-0.5">
            <Label htmlFor={id} className="font-normal">
              {label}
            </Label>
            {hint ? <p className="text-xs text-muted-foreground">{hint}</p> : null}
          </div>
        </div>
      )}
    />
  )
}

// The server's answer to a refused save, in words: an unsupported filter, a
// limit reached, or the server's own message.
function SaveViewError({ error }: { error: unknown }) {
  const { t } = useTranslation()
  const code = getServerErrorCode(error)
  let message: string
  if (code === "query.invalid_parameter") {
    message = t("commodities:savedViews.dialog.invalidQuery", {
      parameter: getServerErrorMeta(error)?.parameter ?? "",
    })
  } else if (code === "saved_view.limit_reached") {
    message = t("commodities:savedViews.dialog.limitReached")
  } else if (code === "saved_view.forbidden") {
    message = t("commodities:savedViews.dialog.forbidden")
  } else {
    message = parseServerError(error, t("commodities:savedViews.dialog.failed"))
  }
  return (
    <Alert variant="destructive" data-testid="saved-view-error">
      <AlertDescription>{message}</AlertDescription>
    </Alert>
  )
}
