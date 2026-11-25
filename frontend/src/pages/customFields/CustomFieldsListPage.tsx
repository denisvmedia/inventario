import { ListPlus, Plus, Search, X } from "lucide-react"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useSearchParams } from "react-router-dom"

import { CustomFieldDeleteDialog } from "@/components/customFields/CustomFieldDeleteDialog"
import { CustomFieldFormDialog } from "@/components/customFields/CustomFieldFormDialog"
import { CustomFieldRow } from "@/components/customFields/CustomFieldRow"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Page, PageHeader } from "@/components/ui/page"
import { Skeleton } from "@/components/ui/skeleton"
import type { CustomField, CustomFieldTemplate } from "@/features/customFields/api"
import {
  useCreateCustomField,
  useCustomFields,
  useUpdateCustomField,
} from "@/features/customFields/hooks"
import type { CustomFieldFormValues } from "@/features/customFields/schemas"
import { useAppToast } from "@/hooks/useAppToast"
import { nameKey } from "@/lib/name-key"
import { parseServerError } from "@/lib/server-error"
import { cn } from "@/lib/utils"

// The same delay as the Tags page search.
const SEARCH_DEBOUNCE_MS = 250

interface DialogState {
  open: boolean
  field: CustomField | null
}

// The Custom fields page (#2803): the group's fields with their types and
// the number of items that have a value of each. The mock has no custom
// fields; the page follows the Item types page, which follows the Tags page
// (design-mocks TagsView): header with a create button, a search, and one
// card listing the rows.
export function CustomFieldsListPage() {
  const { t } = useTranslation()
  const toast = useAppToast()
  const [searchParams, setSearchParams] = useSearchParams()

  const urlQuery = searchParams.get("q") ?? ""
  const [pendingSearch, setPendingSearch] = useState(urlQuery)
  // Re-seed the input when the URL changes through back / forward.
  // eslint-disable-next-line react-hooks/set-state-in-effect
  useEffect(() => setPendingSearch(urlQuery), [urlQuery])
  useEffect(() => {
    const trimmed = pendingSearch.trim()
    if (trimmed === urlQuery) return
    const timer = window.setTimeout(() => {
      const next = new URLSearchParams(searchParams)
      if (trimmed) next.set("q", trimmed)
      else next.delete("q")
      setSearchParams(next, { replace: true })
    }, SEARCH_DEBOUNCE_MS)
    return () => window.clearTimeout(timer)
  }, [pendingSearch, urlQuery, searchParams, setSearchParams])

  const fieldsQuery = useCustomFields()
  const createMutation = useCreateCustomField()
  const updateMutation = useUpdateCustomField()
  const [dialog, setDialog] = useState<DialogState>({ open: false, field: null })
  const [deleting, setDeleting] = useState<CustomField | null>(null)

  const all = useMemo(() => fieldsQuery.data ?? [], [fieldsQuery.data])
  // The whole list is loaded, so the search runs here.
  const visible = useMemo(() => {
    const key = nameKey(urlQuery)
    if (!key) return all
    return all.filter((field) => nameKey(field.name).includes(key))
  }, [all, urlQuery])
  const isInitialLoading = fieldsQuery.isLoading && !fieldsQuery.data

  // A failure rejects, and the dialog maps a duplicate name and field errors
  // onto its inputs. An edit sends no type: the server keeps it.
  async function onSubmit(values: CustomFieldFormValues, templates: CustomFieldTemplate[]) {
    const options =
      values.type === "select"
        ? values.options.map((o) => (o.id ? { id: o.id, label: o.label } : { label: o.label }))
        : undefined
    const editing = dialog.field
    if (editing) {
      await updateMutation.mutateAsync({
        id: editing.id,
        input: { name: values.name, ...(options ? { options } : {}), templates },
      })
      toast.success(t("customFields:toast.updated"))
    } else {
      await createMutation.mutateAsync({
        name: values.name,
        type: values.type,
        options,
        templates,
      })
      toast.success(t("customFields:toast.created"))
    }
  }

  return (
    <Page width="wide" data-testid="page-custom-fields">
      <PageHeader
        title={t("customFields:title")}
        subtitle={t("customFields:description")}
        actions={
          <Button
            type="button"
            onClick={() => setDialog({ open: true, field: null })}
            data-testid="custom-fields-create-button"
          >
            <Plus className="mr-1.5 size-4" aria-hidden="true" />
            {t("customFields:create")}
          </Button>
        }
      />

      <div className="flex flex-col gap-4">
        <div className="relative max-w-md">
          <Label htmlFor="custom-fields-search-input" className="sr-only">
            {t("customFields:search.label")}
          </Label>
          <Search
            className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground"
            aria-hidden="true"
          />
          <Input
            id="custom-fields-search-input"
            type="search"
            value={pendingSearch}
            onChange={(e) => setPendingSearch(e.target.value)}
            placeholder={t("customFields:search.placeholder")}
            className={cn("pl-8", pendingSearch ? "pr-8" : undefined)}
            data-testid="custom-fields-search-input"
          />
          {pendingSearch ? (
            <button
              type="button"
              onClick={() => setPendingSearch("")}
              aria-label={t("customFields:search.clear")}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground transition-colors hover:text-foreground"
              data-testid="custom-fields-search-clear"
            >
              <X aria-hidden="true" className="size-3.5" />
            </button>
          ) : null}
        </div>

        {fieldsQuery.isError ? (
          <div
            className="rounded-md border border-destructive/40 bg-destructive/5 p-4 text-sm text-destructive"
            role="alert"
            data-testid="custom-fields-list-error"
          >
            {parseServerError(fieldsQuery.error, t("customFields:list.loadError"))}
          </div>
        ) : (
          <div
            className="overflow-hidden rounded-xl border border-border bg-card"
            data-testid="custom-fields-list-container"
          >
            <div className="flex items-center justify-between border-b border-border px-4 py-3">
              <p
                className="text-xs font-semibold uppercase tracking-widest text-muted-foreground"
                data-testid="custom-fields-list-count"
              >
                {urlQuery
                  ? t("customFields:list.results", { count: visible.length })
                  : t("customFields:list.total", { count: all.length })}
              </p>
            </div>

            {isInitialLoading ? (
              <div className="flex flex-col gap-2 p-4" data-testid="custom-fields-list-loading">
                {Array.from({ length: 6 }).map((_, idx) => (
                  <Skeleton key={idx} className="h-10 w-full" />
                ))}
              </div>
            ) : visible.length === 0 ? (
              <div
                className="flex flex-col items-center justify-center gap-3 py-16 text-center"
                data-testid="custom-fields-list-empty"
              >
                <ListPlus aria-hidden="true" className="size-8 text-muted-foreground/30" />
                <p className="text-sm text-muted-foreground">
                  {urlQuery
                    ? t("customFields:list.emptyFiltered", { query: urlQuery })
                    : t("customFields:list.empty")}
                </p>
              </div>
            ) : (
              <ul className="divide-y divide-border" data-testid="custom-fields-list">
                {visible.map((field) => (
                  <li key={field.id}>
                    <CustomFieldRow
                      field={field}
                      onEdit={() => setDialog({ open: true, field })}
                      onDelete={() => setDeleting(field)}
                    />
                  </li>
                ))}
              </ul>
            )}
          </div>
        )}
      </div>

      <CustomFieldFormDialog
        open={dialog.open}
        onOpenChange={(open) =>
          setDialog((prev) => (open ? prev : { open: false, field: prev.field }))
        }
        field={dialog.field}
        fields={all}
        onSubmit={onSubmit}
        isPending={createMutation.isPending || updateMutation.isPending}
      />

      <CustomFieldDeleteDialog field={deleting} onClose={() => setDeleting(null)} />
    </Page>
  )
}
