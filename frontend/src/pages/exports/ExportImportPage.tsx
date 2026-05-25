import { ArrowLeft, FileUp, Loader2 } from "lucide-react"
import { useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router-dom"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Page, PageHeader } from "@/components/ui/page"
import { Skeleton } from "@/components/ui/skeleton"
import { useImportBackup, useUploadRestoreFile } from "@/features/export/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"
import { formatBytes } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"

export function ExportImportPage() {
  const { t } = useTranslation(["exports", "common"])
  const navigate = useNavigate()
  const toast = useAppToast()
  const { currentGroup } = useCurrentGroup()
  const groupReady = !!currentGroup
  const slug = currentGroup?.slug ?? ""

  const fileInputRef = useRef<HTMLInputElement>(null)
  const [description, setDescription] = useState("")
  const [file, setFile] = useState<File | null>(null)

  const uploadMutation = useUploadRestoreFile()
  const importMutation = useImportBackup()
  const submitting = uploadMutation.isPending || importMutation.isPending

  function onFileChange(picked: File | null) {
    setFile(picked)
  }

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault()
    if (!file || !groupReady) return
    try {
      const upload = await uploadMutation.mutateAsync(file)
      const created = await importMutation.mutateAsync({
        description,
        source_file_path: upload.sourceFilePath,
      })
      toast.success(t("exports:importView.success"))
      // Imported exports always start at status=completed (the BE inflates
      // metadata synchronously). Send the user straight to the restore form
      // — the only meaningful next step is to actually run the restore.
      navigate(`/g/${encodeURIComponent(slug)}/exports/${encodeURIComponent(created.id)}/restore`)
    } catch (err) {
      // Same JSON:API extraction as ExportNewPage — let the user see the
      // BE's `errors[].detail` (e.g. "Description must be 500 chars or
      // fewer") instead of the bare HTTP wrapper.
      const message = parseServerError(err, String(err))
      const key = uploadMutation.isError
        ? "exports:errors.uploadFailed"
        : "exports:errors.importFailed"
      toast.error(t(key, { error: message }))
    }
  }

  if (!groupReady) {
    return (
      <Page width="narrow" className="gap-4" data-testid="page-export-import-loading">
        <Skeleton className="h-8 w-1/2" />
        <Skeleton className="h-64 w-full" />
      </Page>
    )
  }

  return (
    <Page width="narrow" data-testid="page-export-import">
      <PageHeader
        size="detail"
        title={t("exports:importView.title")}
        subtitle={t("exports:importView.intro")}
        subtitleClassName="max-w-prose text-sm"
        backLink={
          <Link
            to={`/g/${encodeURIComponent(slug)}/exports`}
            className="inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            {t("exports:list.title")}
          </Link>
        }
      />

      {(uploadMutation.isError || importMutation.isError) && (
        <Alert variant="destructive" data-testid="import-error">
          <AlertTitle>
            {uploadMutation.isError
              ? t("exports:errors.uploadFailed", {
                  error:
                    uploadMutation.error instanceof Error
                      ? uploadMutation.error.message
                      : "unknown",
                })
              : t("exports:errors.importFailed", {
                  error:
                    importMutation.error instanceof Error
                      ? importMutation.error.message
                      : "unknown",
                })}
          </AlertTitle>
        </Alert>
      )}

      <form onSubmit={onSubmit} className="flex flex-col gap-5" data-testid="import-form">
        <div className="flex flex-col gap-2">
          <Label htmlFor="import-file">{t("exports:importView.fileLabel")}</Label>
          <div
            className="flex cursor-pointer flex-col items-center gap-2 rounded-md border-2 border-dashed bg-muted/20 p-6 text-center text-sm text-muted-foreground hover:bg-muted/30"
            onClick={() => fileInputRef.current?.click()}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault()
                fileInputRef.current?.click()
              }
            }}
            role="button"
            tabIndex={0}
            data-testid="import-dropzone"
            aria-label={t("exports:importView.fileLabel")}
          >
            <FileUp className="size-6" aria-hidden="true" />
            <span>{t("exports:importView.dropHint")}</span>
            {file && (
              <span className="font-medium text-foreground" data-testid="import-file-chosen">
                {t("exports:importView.fileChosen", {
                  name: file.name,
                  size: formatBytes(file.size),
                })}
              </span>
            )}
          </div>
          <Input
            ref={fileInputRef}
            id="import-file"
            type="file"
            accept=".xml,application/xml,text/xml"
            className="sr-only"
            onChange={(e) => onFileChange(e.target.files?.[0] ?? null)}
            data-testid="import-file-input"
          />
        </div>

        <div className="flex flex-col gap-2">
          <Label htmlFor="import-description">{t("exports:importView.description")}</Label>
          <Input
            id="import-description"
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            placeholder={t("exports:importView.descriptionPlaceholder")}
            maxLength={500}
            data-testid="import-description"
          />
          <p className="text-xs text-muted-foreground">{t("exports:importView.descriptionHint")}</p>
        </div>

        <div className="flex justify-end gap-2">
          <Button asChild variant="ghost" type="button">
            <Link to={`/g/${encodeURIComponent(slug)}/exports`}>{t("exports:wizard.cancel")}</Link>
          </Button>
          <Button type="submit" disabled={!file || submitting} data-testid="import-submit">
            {submitting && <Loader2 className="mr-1.5 size-4 animate-spin" aria-hidden="true" />}
            {submitting ? t("exports:importView.submitting") : t("exports:importView.submit")}
          </Button>
        </div>

        {uploadMutation.isPending && (
          <AlertDescription data-testid="import-uploading">
            {t("exports:importView.uploadProgress")}
          </AlertDescription>
        )}
      </form>
    </Page>
  )
}
