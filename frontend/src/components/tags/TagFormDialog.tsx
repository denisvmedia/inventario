import { zodResolver } from "@hookform/resolvers/zod"
import { useEffect } from "react"
import { Controller, useForm } from "react-hook-form"
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

import { TagBadge } from "./TagBadge"
import { TagColorPicker } from "./TagColorPicker"
import type { TagColor, TagEntity } from "@/features/tags/api"
import { normaliseSlug, tagFormSchema, type TagFormInput } from "@/features/tags/schemas"
import { applyServerFieldErrors } from "@/lib/form-errors"

export interface TagFormDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  mode: "create" | "edit"
  // In edit mode the dialog prefills from this object. Caller preserves
  // the id; the dialog only sends label/slug/color.
  initialValues?: TagEntity
  onSubmit: (values: { label: string; slug: string; color: TagColor }) => Promise<void>
  isPending?: boolean
}

const DEFAULTS: TagFormInput = { label: "", slug: "", color: "muted" }

export function TagFormDialog({
  open,
  onOpenChange,
  mode,
  initialValues,
  onSubmit,
  isPending = false,
}: TagFormDialogProps) {
  const { t } = useTranslation(["tags", "common"])
  const {
    control,
    formState: { errors, isSubmitting, dirtyFields },
    handleSubmit,
    register,
    reset,
    setError,
    setValue,
    watch,
  } = useForm<TagFormInput>({
    resolver: zodResolver(tagFormSchema),
    defaultValues: DEFAULTS,
  })

  // Re-seed the form when the dialog re-opens or the underlying entity
  // changes. Without this, switching from "edit kitchen" to "edit garden"
  // without a remount would leave the previous form values visible for
  // a frame (rhf keeps state across opens by default).
  useEffect(() => {
    if (!open) return
    if (mode === "edit" && initialValues) {
      reset({
        label: initialValues.label ?? "",
        slug: initialValues.slug ?? "",
        color: (initialValues.color ?? "muted") as TagColor,
      })
    } else {
      reset(DEFAULTS)
    }
  }, [open, mode, initialValues, reset])

  // Live-derive slug from label while creating — only while the slug
  // field has NOT been user-edited. RHF's `dirtyFields.slug` flips true
  // the moment the user types a custom slug, and never flips back
  // (until reset()), so this is a stable signal: an unedited slug
  // tracks the label; a user-edited slug stays put even if the user
  // keeps tweaking the label.
  const labelValue = watch("label")
  useEffect(() => {
    if (mode !== "create") return
    if (dirtyFields.slug) return
    setValue("slug", normaliseSlug(labelValue ?? ""), {
      shouldValidate: false,
      shouldDirty: false,
    })
  }, [labelValue, mode, dirtyFields.slug, setValue])

  const colorWatch = watch("color")
  // Preview falls back to a dedicated key — using the input placeholder
  // ("e.g. Kitchen") would render misleading sample text inside the
  // pill itself.
  const previewLabel = labelValue || t("tags:form.previewFallback")

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-md" data-testid="tag-form-dialog">
        <DialogHeader>
          <DialogTitle>
            {mode === "create" ? t("tags:form.createTitle") : t("tags:form.editTitle")}
          </DialogTitle>
          <DialogDescription className="sr-only">{t("tags:description")}</DialogDescription>
        </DialogHeader>

        <form
          className="flex flex-col gap-4"
          onSubmit={handleSubmit(async (values) => {
            try {
              await onSubmit({
                label: values.label,
                slug: values.slug,
                color: values.color as TagColor,
              })
            } catch (err) {
              // Host toasts a summary and re-throws; map the BE's field-level
              // 422 (e.g. duplicate slug) onto the inputs.
              applyServerFieldErrors(err, setError, { fields: ["label", "slug", "color"] })
            }
          })}
        >
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="tag-form-label">{t("tags:form.label")}</Label>
            <Input
              id="tag-form-label"
              data-testid="tag-form-label"
              placeholder={t("tags:form.labelPlaceholder")}
              {...register("label")}
            />
            {errors.label?.message ? (
              <p className="text-xs text-destructive" data-testid="tag-form-label-error">
                {t(errors.label.message)}
              </p>
            ) : null}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="tag-form-slug">{t("tags:form.slug")}</Label>
            <Input
              id="tag-form-slug"
              data-testid="tag-form-slug"
              placeholder={t("tags:form.slugPlaceholder")}
              {...register("slug")}
            />
            <p className="text-xs text-muted-foreground">{t("tags:form.slugHint")}</p>
            {errors.slug?.message ? (
              <p className="text-xs text-destructive" data-testid="tag-form-slug-error">
                {t(errors.slug.message)}
              </p>
            ) : null}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label>{t("tags:form.color")}</Label>
            <Controller
              control={control}
              name="color"
              render={({ field }) => (
                <TagColorPicker
                  value={field.value as TagColor}
                  onChange={(c) => field.onChange(c)}
                  testId="tag-form-color"
                  disabled={isPending}
                />
              )}
            />
            {errors.color?.message ? (
              <p className="text-xs text-destructive" data-testid="tag-form-color-error">
                {t(errors.color.message)}
              </p>
            ) : null}
          </div>

          <div className="flex items-center gap-2 rounded-md border bg-muted/30 px-3 py-2">
            <span className="text-xs text-muted-foreground">Preview:</span>
            <TagBadge
              label={previewLabel}
              color={colorWatch as TagColor}
              testId="tag-form-preview"
            />
          </div>

          <DialogFooter>
            <Button
              type="button"
              variant="outline"
              onClick={() => onOpenChange(false)}
              disabled={isSubmitting || isPending}
              data-testid="tag-form-cancel"
            >
              {t("tags:form.cancel")}
            </Button>
            <Button
              type="submit"
              disabled={isSubmitting || isPending}
              data-testid="tag-form-submit"
            >
              {mode === "create" ? t("tags:form.create") : t("tags:form.save")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
