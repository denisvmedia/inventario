import { zodResolver } from "@hookform/resolvers/zod"
import { useEffect } from "react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { useNavigate, useParams } from "react-router-dom"

import { TagsInput } from "@/components/files/TagsInput"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Skeleton } from "@/components/ui/skeleton"
import { useFile, useUpdateFile } from "@/features/files/hooks"
import {
  fileMetadataSchema,
  type FileMetadataFormInput,
} from "@/features/files/schemas"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"

// Standalone metadata edit page deep-linked from the detail sheet.
// Touches only the JSON metadata — the actual file content lives under
// the BE-managed signed URLs and is never replaced through this form.
export function FileEditPage() {
  const { t } = useTranslation()
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const groupSlug = currentGroup?.slug ?? ""
  const toast = useAppToast()

  const query = useFile(id, { enabled: !!id })
  const update = useUpdateFile(id ?? "")

  const form = useForm<FileMetadataFormInput>({
    resolver: zodResolver(fileMetadataSchema),
    defaultValues: {
      title: "",
      description: "",
      path: "",
      category: "other",
      tags: [],
    },
  })

  useEffect(() => {
    if (!query.data?.file) return
    const f = query.data.file
    form.reset({
      title: f.title ?? "",
      description: f.description ?? "",
      path: f.path ?? "",
      category: f.category ?? "other",
      tags: f.tags ?? [],
    })
  }, [query.data?.file, form])

  function onCancel() {
    if (groupSlug) {
      navigate(`/g/${encodeURIComponent(groupSlug)}/files`)
    } else {
      navigate("/files")
    }
  }

  async function onSubmit(values: FileMetadataFormInput) {
    if (!id) return
    try {
      await update.mutateAsync({
        title: values.title || "",
        description: values.description || "",
        path: values.path,
        category: values.category,
        tags: values.tags,
      })
      toast.success(t("files:edit.save"))
      onCancel()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className="mx-auto max-w-2xl space-y-6" data-testid="page-file-edit">
      <RouteTitle title={t("files:edit.title", { defaultValue: "Edit file" })} />
      <header>
        <h1 className="text-2xl font-semibold tracking-tight">{t("files:edit.title")}</h1>
        <p className="text-sm text-muted-foreground">{t("files:edit.subtitle")}</p>
      </header>

      {query.isLoading ? (
        <Card>
          <CardContent className="space-y-4 p-6">
            <Skeleton className="h-8 w-1/2" />
            <Skeleton className="h-24 w-full" />
            <Skeleton className="h-8 w-1/3" />
          </CardContent>
        </Card>
      ) : query.error ? (
        <Alert variant="destructive">
          <AlertTitle>
            {t("common:errors.generic", { defaultValue: "Something went wrong" })}
          </AlertTitle>
          <AlertDescription>{(query.error as Error).message}</AlertDescription>
        </Alert>
      ) : (
        <Card>
          <CardContent className="p-6">
            <form className="flex flex-col gap-4" onSubmit={form.handleSubmit(onSubmit)}>
              <div className="flex flex-col gap-1.5">
                <Label htmlFor="file-title">{t("files:edit.fields.title")}</Label>
                <Input
                  id="file-title"
                  data-testid="file-edit-title"
                  {...form.register("title")}
                />
                <p className="text-xs text-muted-foreground">
                  {t("files:edit.fields.titleHint")}
                </p>
              </div>

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="file-path">{t("files:edit.fields.path")}</Label>
                <Input
                  id="file-path"
                  data-testid="file-edit-path"
                  {...form.register("path")}
                />
                <p className="text-xs text-muted-foreground">{t("files:edit.fields.pathHint")}</p>
                {form.formState.errors.path ? (
                  <p className="text-xs text-destructive">
                    {form.formState.errors.path.message}
                  </p>
                ) : null}
              </div>

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="file-description">{t("files:edit.fields.description")}</Label>
                <textarea
                  id="file-description"
                  data-testid="file-edit-description"
                  rows={4}
                  className="rounded-md border border-input bg-transparent px-3 py-2 text-sm"
                  {...form.register("description")}
                />
              </div>

              <div className="flex flex-col gap-1.5">
                <Label htmlFor="file-category">{t("files:edit.fields.category")}</Label>
                <Controller
                  control={form.control}
                  name="category"
                  render={({ field }) => (
                    <select
                      id="file-category"
                      data-testid="file-edit-category"
                      value={field.value}
                      onChange={(e) => field.onChange(e.target.value)}
                      className="rounded-md border border-input bg-transparent px-3 py-2 text-sm"
                    >
                      <option value="photos">
                        {t("files:categoryPhotos", { defaultValue: "Photos" })}
                      </option>
                      <option value="invoices">
                        {t("files:categoryInvoices", { defaultValue: "Invoices" })}
                      </option>
                      <option value="documents">
                        {t("files:categoryDocuments", { defaultValue: "Documents" })}
                      </option>
                      <option value="other">
                        {t("files:categoryOther", { defaultValue: "Other" })}
                      </option>
                    </select>
                  )}
                />
              </div>

              <Controller
                control={form.control}
                name="tags"
                render={({ field }) => (
                  <TagsInput
                    label={t("files:edit.fields.tags")}
                    values={field.value ?? []}
                    onChange={field.onChange}
                    testId="file-edit-tags"
                  />
                )}
              />

              <div className="flex justify-end gap-2 pt-2">
                <Button type="button" variant="outline" onClick={onCancel}>
                  {t("files:edit.cancel")}
                </Button>
                <Button
                  type="submit"
                  disabled={update.isPending || !form.formState.isDirty}
                  data-testid="file-edit-save"
                >
                  {t("files:edit.save")}
                </Button>
              </div>
            </form>
          </CardContent>
        </Card>
      )}
    </div>
  )
}
