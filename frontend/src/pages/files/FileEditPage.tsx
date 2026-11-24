import { zodResolver } from "@hookform/resolvers/zod"
import { useMemo, useState } from "react"
import { Controller, useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useParams, useSearchParams } from "react-router-dom"
import { ArrowLeft, ChevronRight, Download, ExternalLink } from "lucide-react"

import { FilePreview } from "@/components/files/FilePreview"
import { ListPager } from "@/components/ListPager"
import { ImageViewer } from "@/components/files/ImageViewer"
import { PdfFullViewer } from "@/components/files/PdfFullViewer"
import { TagsInput } from "@/components/files/TagsInput"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Page, PageHeader } from "@/components/ui/page"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import type { FileEntity, URLData } from "@/features/files/api"
import { isImageMime, isPdfMime } from "@/features/files/constants"
import { useFile, useUpdateFile } from "@/features/files/hooks"
import {
  fileInListHref,
  fileReviewHref,
  filesListPath,
  filesListSource,
  isFilesListContext,
} from "@/features/files/listContext"
import { fileMetadataSchema, type FileMetadataFormInput } from "@/features/files/schemas"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useTagLabel } from "@/features/tags/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import {
  useListNavigation,
  type ListNavigation,
  type ListNeighbor,
} from "@/hooks/useListNavigation"
import { useUnsavedChangesGuard } from "@/hooks/useUnsavedChangesGuard"
import { applyServerFieldErrors, shouldShowGenericError } from "@/lib/form-errors"
import {
  listContextHref,
  readListContext,
  withListContextPage,
  type ListContext,
} from "@/lib/list-context"

type ReviewTab = "preview" | "details"

// File review page: the file's preview beside its metadata form.
//
// Opened from the Files list it carries that list in `?from=` (see
// lib/list-context.ts) and offers Previous, Next and Save and next within the
// list's filters and order (hooks/useListNavigation.ts). Back, Cancel and
// Save return to the list as the user left it, with this file's detail panel
// open. Opened from anywhere else (an item's Files tab, a direct link) it
// returns to where it came from, or to the file's detail panel on the Files
// page, and hides the list controls.
//
// On narrow screens Preview and Details are tabs. Both stay mounted, so
// switching keeps the entered values and the PDF page and zoom.
export function FileEditPage() {
  const { t } = useTranslation()
  const { id = "", groupSlug = "" } = useParams<{ id: string; groupSlug: string }>()
  const [searchParams] = useSearchParams()
  const { currentGroup } = useCurrentGroup()
  // The list queries wait for the group, like the Files list does, so both
  // pages build the same query keys and share the cached pages.
  const groupReady = !!currentGroup?.slug
  const slug = currentGroup?.slug ?? groupSlug
  const [tab, setTab] = useState<ReviewTab>("details")

  const from = useMemo(
    () => (slug ? readListContext(searchParams, `/g/${encodeURIComponent(slug)}`) : null),
    [searchParams, slug]
  )
  const filesList = from && isFilesListContext(from, slug) ? from : null
  const source = useMemo(
    () => (filesList && groupReady ? filesListSource(filesList, slug) : null),
    [filesList, groupReady, slug]
  )
  const nav = useListNavigation({ source, currentId: id || undefined })

  const query = useFile(id, { enabled: !!id })
  const file = query.data?.file
  // Rendering the review only once the loaded file is the one in the URL
  // keeps the previous file's values off screen while the next one loads.
  const ready = !!file && file.id === id

  let returnHref: string
  if (filesList) returnHref = fileInListHref(withListContextPage(filesList, nav.page), id)
  else if (from) returnHref = listContextHref(from)
  else returnHref = `${filesListPath(slug)}/${encodeURIComponent(id)}`

  const pageTitle = t("files:edit.title")

  return (
    <Page width="wide" data-testid="page-file-edit">
      <RouteTitle title={pageTitle} />
      {ready ? (
        <FileReview
          key={file.id}
          file={file}
          signedUrl={query.data?.signedUrl}
          slug={slug}
          filesList={filesList}
          nav={nav}
          returnHref={returnHref}
          tab={tab}
          onTabChange={setTab}
        />
      ) : (
        <>
          <PageHeader size="detail" title={pageTitle} subtitle={t("files:edit.subtitle")} />
          {query.error ? (
            <Alert variant="destructive">
              <AlertTitle>
                {t("common:errors.generic", { defaultValue: "Something went wrong" })}
              </AlertTitle>
              <AlertDescription>{(query.error as Error).message}</AlertDescription>
            </Alert>
          ) : (
            <div className="grid gap-6 lg:grid-cols-5" data-testid="file-review-loading">
              <Skeleton className="aspect-[4/3] w-full lg:col-span-3" />
              <Card className="lg:col-span-2">
                <CardContent className="space-y-4 p-6">
                  <Skeleton className="h-8 w-1/2" />
                  <Skeleton className="h-24 w-full" />
                  <Skeleton className="h-8 w-1/3" />
                </CardContent>
              </Card>
            </div>
          )}
        </>
      )}
    </Page>
  )
}

interface FileReviewProps {
  file: FileEntity & { id: string }
  signedUrl?: URLData
  slug: string
  filesList: ListContext | null
  nav: ListNavigation
  returnHref: string
  tab: ReviewTab
  onTabChange: (tab: ReviewTab) => void
}

const FORM_FIELDS = ["title", "description", "path", "category", "tags"] as const

function FileReview({
  file,
  signedUrl,
  slug,
  filesList,
  nav,
  returnHref,
  tab,
  onTabChange,
}: FileReviewProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const toast = useAppToast()
  const update = useUpdateFile(file.id)
  const tagLabel = useTagLabel("file")
  const [imageViewerOpen, setImageViewerOpen] = useState(false)
  const [pdfViewerOpen, setPdfViewerOpen] = useState(false)
  // Touch the validation key once at the source so i18next-cli's
  // static extractor sees it — the schema returns the key as a string
  // literal which the parser can't follow into JSX.
  void t("files:validation.pathRequired")

  const form = useForm<FileMetadataFormInput>({
    resolver: zodResolver(fileMetadataSchema),
    defaultValues: {
      title: file.title ?? "",
      description: file.description ?? "",
      path: file.path ?? "",
      category: file.category ?? "other",
      tags: file.tags ?? [],
    },
  })
  const dirty = form.formState.isDirty

  const title = file.title?.trim() || file.path?.trim() || file.id
  const url = signedUrl?.url
  const inlineUrl = signedUrl?.inline_url ?? url

  function recordHref(target: ListNeighbor): string {
    return fileReviewHref(slug, target.id, filesList && withListContextPage(filesList, target.page))
  }

  async function persist(values: FileMetadataFormInput): Promise<boolean> {
    try {
      await update.mutateAsync({
        title: values.title || "",
        description: values.description || "",
        path: values.path,
        category: values.category,
        tags: values.tags,
      })
      form.reset(values)
      toast.success(t("files:review.saved"))
      return true
    } catch (err) {
      // Map BE field-level 422s onto the inputs; only toast for non-field
      // errors (or anything that couldn't be placed on a field).
      const result = applyServerFieldErrors(err, form.setError, { fields: FORM_FIELDS })
      if (shouldShowGenericError(result)) {
        toast.error(err instanceof Error ? err.message : String(err))
      }
      onTabChange("details")
      return false
    }
  }

  // Validates, then saves. Resolves false on a validation or server error;
  // the form keeps the entered values and shows the errors.
  async function save(): Promise<boolean> {
    let ok = false
    await form.handleSubmit(
      async (values) => {
        ok = await persist(values)
      },
      () => onTabChange("details")
    )()
    return ok
  }

  const { guard, dialog } = useUnsavedChangesGuard({ dirty, onSave: save })

  async function onSave() {
    // Enter in a field submits the form; with nothing changed there is
    // nothing to save, as the disabled Save button says.
    if (!dirty) return
    if (await save()) navigate(returnHref)
  }

  async function onSaveAndNext() {
    // The target is read before saving: an edit that takes this file out of
    // the list's filter must not change which file comes next.
    const target = nav.next
    if (!target) return
    if (dirty && !(await save())) return
    navigate(recordHref(target))
  }

  const pathError = form.formState.errors.path

  return (
    <>
      <PageHeader
        size="detail"
        title={<span className="break-all">{title}</span>}
        subtitle={t("files:edit.subtitle")}
        subtitleClassName="text-sm"
        backLink={
          <Link
            to={returnHref}
            className="inline-flex items-center gap-1.5 text-muted-foreground transition-colors hover:text-foreground"
            data-testid="file-review-back"
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            {filesList ? t("files:review.backToList") : t("common:actions.back")}
          </Link>
        }
        actions={
          nav.active ? (
            <ListPager
              nav={nav}
              hrefFor={recordHref}
              labels={{
                pager: t("files:review.pagerLabel"),
                previous: t("files:review.previous"),
                next: t("files:review.next"),
              }}
              testIdPrefix="file-review"
            />
          ) : null
        }
      />

      {nav.active && !nav.loading && !nav.inList ? (
        <p className="-mt-3 text-sm text-muted-foreground" data-testid="file-review-not-in-list">
          {t("files:review.notInList")}
        </p>
      ) : null}

      <Tabs value={tab} onValueChange={(next) => onTabChange(next as ReviewTab)} className="gap-4">
        <TabsList className="w-full lg:hidden" data-testid="file-review-tabs">
          <TabsTrigger value="preview" data-testid="file-review-tab-preview">
            {t("files:review.tabPreview")}
          </TabsTrigger>
          <TabsTrigger value="details" data-testid="file-review-tab-details">
            {t("files:review.tabDetails")}
          </TabsTrigger>
        </TabsList>

        {/* One column below lg: the active tab's panel, then the action row,
            which sticks to the bottom of the screen while the panel scrolls.
            From lg up: preview on the left across both rows, the form and
            the action row under it on the right. */}
        <div className="grid gap-6 lg:grid-cols-5 lg:grid-rows-[auto_1fr]">
          <TabsContent
            value="preview"
            forceMount
            className="flex min-w-0 flex-col gap-3 data-[state=inactive]:hidden lg:col-span-3 lg:row-span-2 lg:data-[state=inactive]:flex"
            data-testid="file-review-preview"
          >
            <FilePreview
              variant="pane"
              mime={file.mime_type}
              url={url}
              alt={title}
              onExpandImage={() => setImageViewerOpen(true)}
              onExpandPdf={() => setPdfViewerOpen(true)}
            />
            {url ? (
              <div className="flex flex-wrap gap-2">
                <Button asChild variant="outline" size="sm" className="gap-1.5">
                  <a
                    href={inlineUrl}
                    target="_blank"
                    rel="noreferrer"
                    data-testid="file-review-open"
                  >
                    <ExternalLink className="size-3.5" aria-hidden="true" />
                    {t("files:detail.openInNewTab")}
                  </a>
                </Button>
                <Button asChild variant="outline" size="sm" className="gap-1.5">
                  <a href={url} download data-testid="file-review-download">
                    <Download className="size-3.5" aria-hidden="true" />
                    {t("files:detail.download")}
                  </a>
                </Button>
              </div>
            ) : null}
          </TabsContent>

          <TabsContent
            value="details"
            forceMount
            className="min-w-0 data-[state=inactive]:hidden lg:col-span-2 lg:col-start-4 lg:row-start-1 lg:data-[state=inactive]:block"
            data-testid="file-review-details"
          >
            <Card>
              <CardContent className="p-6">
                <form
                  id="file-review-form"
                  className="flex flex-col gap-4"
                  onSubmit={(e) => {
                    e.preventDefault()
                    void onSave()
                  }}
                >
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
                      aria-invalid={pathError ? true : undefined}
                      aria-describedby={pathError ? "file-path-error" : "file-path-hint"}
                      {...form.register("path")}
                    />
                    <p id="file-path-hint" className="text-xs text-muted-foreground">
                      {t("files:edit.fields.pathHint")}
                    </p>
                    {pathError ? (
                      <p id="file-path-error" className="text-xs text-destructive">
                        {/* Schema emits the i18n key (see schemas.ts); the
                            page resolves it via t() at render so cs/ru can
                            translate. */}
                        {t(pathError.message ?? "")}
                      </p>
                    ) : null}
                  </div>

                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="file-description">{t("files:edit.fields.description")}</Label>
                    <Textarea
                      id="file-description"
                      data-testid="file-edit-description"
                      rows={4}
                      {...form.register("description")}
                    />
                  </div>

                  <div className="flex flex-col gap-1.5">
                    <Label htmlFor="file-category">{t("files:edit.fields.category")}</Label>
                    <Controller
                      control={form.control}
                      name="category"
                      render={({ field }) => (
                        // eslint-disable-next-line no-restricted-syntax -- file-edit category field; native <select> retained (has native-select coverage). shadcn migration tracked as a forms follow-up — see devdocs/frontend/forms.md
                        <select
                          id="file-category"
                          data-testid="file-edit-category"
                          value={field.value}
                          onChange={(e) => field.onChange(e.target.value)}
                          className="h-9 rounded-md border border-input bg-transparent px-3 text-sm"
                        >
                          <option value="images">
                            {t("files:categoryImages", { defaultValue: "Images" })}
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
                        autocomplete
                        kind="file"
                        labelOf={tagLabel}
                      />
                    )}
                  />
                </form>
              </CardContent>
            </Card>
          </TabsContent>

          <div
            className="sticky bottom-0 z-20 -mx-6 flex flex-wrap items-center justify-end gap-2 border-t border-border bg-background px-6 py-3 lg:static lg:col-span-2 lg:col-start-4 lg:row-start-2 lg:mx-0 lg:self-start lg:border-0 lg:bg-transparent lg:p-0"
            data-testid="file-review-actions"
          >
            <Button
              type="button"
              variant="ghost"
              className="mr-auto"
              onClick={() => guard(() => navigate(returnHref))}
              data-testid="file-review-cancel"
            >
              {t("files:edit.cancel")}
            </Button>
            <Button
              type="submit"
              form="file-review-form"
              variant={nav.active ? "outline" : "default"}
              disabled={update.isPending || !dirty}
              data-testid="file-edit-save"
            >
              {t("files:edit.save")}
            </Button>
            {nav.active ? (
              <Button
                type="button"
                onClick={onSaveAndNext}
                disabled={update.isPending || !nav.next}
                className="gap-1"
                data-testid="file-review-save-next"
              >
                {t("files:review.saveAndNext")}
                <ChevronRight className="size-4" aria-hidden="true" />
              </Button>
            ) : null}
          </div>
        </div>
      </Tabs>

      {dialog}

      {url && isImageMime(file.mime_type) ? (
        <ImageViewer
          open={imageViewerOpen}
          onOpenChange={setImageViewerOpen}
          url={url}
          alt={title}
        />
      ) : null}
      {url && isPdfMime(file.mime_type) ? (
        <Dialog open={pdfViewerOpen} onOpenChange={setPdfViewerOpen}>
          <DialogContent
            className="flex h-screen w-screen max-w-none flex-col gap-0 rounded-none border-0 p-0 [&>button]:hidden sm:max-w-none"
            data-testid="file-review-pdf-fullscreen"
          >
            <DialogHeader className="sr-only">
              <DialogTitle>{title}</DialogTitle>
              <DialogDescription>{t("files:edit.title")}</DialogDescription>
            </DialogHeader>
            <PdfFullViewer url={url} title={title} onClose={() => setPdfViewerOpen(false)} />
          </DialogContent>
        </Dialog>
      ) : null}
    </>
  )
}
