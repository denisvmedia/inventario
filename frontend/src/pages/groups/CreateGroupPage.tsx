import { useEffect, useState } from "react"
import { useForm, Controller } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { ArrowLeft, ArrowRight, Building2 } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Page, PageHeader } from "@/components/ui/page"
import { CurrencyCombobox } from "@/components/CurrencyCombobox"
import { FieldError } from "@/components/FieldError"
import { ServerErrorBanner } from "@/components/ServerErrorBanner"
import { IconPicker } from "@/components/groups/IconPicker"
import { useCreateGroup } from "@/features/group/hooks"
import { createGroupSchema, type CreateGroupInput } from "@/features/group/schemas"
import { useAppToast } from "@/hooks/useAppToast"
import { applyServerFieldErrors, shouldShowGenericError } from "@/lib/form-errors"
import { classifyServerError, type ClassifiedServerError } from "@/lib/server-error"
import { RouteTitle } from "@/components/routing/RouteTitle"

// /groups/new — create-group form. Renders a single panel (name + icon
// picker + currency) and POSTs /groups on submit. The server picks the
// slug; we navigate to /g/{slug} on success so the new group is the
// active one immediately.
export function CreateGroupPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const mutation = useCreateGroup()
  const toast = useAppToast()
  const [serverError, setServerError] = useState<ClassifiedServerError | null>(null)

  const form = useForm<CreateGroupInput>({
    resolver: zodResolver(createGroupSchema),
    defaultValues: { name: "", icon: "", group_currency: "USD" },
  })

  useEffect(() => {
    const sub = form.watch(() => {
      if (serverError) setServerError(null)
    })
    return () => sub.unsubscribe()
  }, [form, serverError])

  async function onSubmit(values: CreateGroupInput) {
    setServerError(null)
    try {
      const created = await mutation.mutateAsync({
        name: values.name.trim(),
        icon: values.icon || undefined,
        group_currency: values.group_currency.toUpperCase(),
      })
      // The BE invariant is that every persisted group carries a slug
      // (assigned at create-time inside the transaction). A response
      // without one means something is off — surface it as an error
      // and stay on the form rather than silently navigating to
      // /no-group, which would *also* drop the user's active group
      // context when they came from GroupSelector → "Create new
      // group" (the previous fallback). #1886.
      if (!created.slug) {
        // Dedicated key so the user isn't told to "try again" (the group
        // exists; a retry would duplicate it) — they need to reload so
        // RootRedirect can resolve the freshly-created group via the
        // invalidated /groups cache.
        setServerError({ kind: "unknown", message: t("groups:create.errorMissingSlug") })
        return
      }
      toast.success(t("groups:create.successToast"))
      navigate(`/g/${encodeURIComponent(created.slug)}`)
    } catch (err) {
      const fieldResult = applyServerFieldErrors(err, form.setError, {
        fields: Object.keys(createGroupSchema.shape),
      })
      setServerError(
        shouldShowGenericError(fieldResult)
          ? classifyServerError(err, t("groups:create.errorGeneric"))
          : null
      )
    }
  }

  return (
    <>
      <RouteTitle title={t("groups:create.title")} />
      <Page width="narrow" className="gap-8" data-testid="create-group-page">
        <PageHeader
          size="detail"
          title={t("groups:create.title")}
          subtitle={t("groups:create.subtitle")}
          icon={
            <span className="flex size-10 items-center justify-center rounded-lg bg-primary/10">
              <Building2 className="size-5 text-primary" aria-hidden="true" />
            </span>
          }
          /*
            "Back" returns to wherever the user came from rather than
            hard-coding /no-group: this page is reachable both from
            onboarding (zero groups) and from the GroupSelector (creating
            an additional group), and either label/destination would be
            wrong half the time.
          */
          backLink={
            <button
              type="button"
              onClick={() => navigate(-1)}
              data-testid="create-group-back"
              className="inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground transition-colors"
            >
              <ArrowLeft className="size-4" aria-hidden="true" />
              {t("common:actions.back")}
            </button>
          }
        />

        <form
          className="space-y-5 rounded-xl border border-border bg-card p-5"
          onSubmit={form.handleSubmit(onSubmit)}
          noValidate
        >
          <div className="space-y-1.5">
            <Label htmlFor="group-name">{t("groups:create.nameLabel")}</Label>
            <Input
              id="group-name"
              placeholder={t("groups:create.namePlaceholder")}
              autoComplete="off"
              maxLength={100}
              disabled={mutation.isPending}
              aria-invalid={!!form.formState.errors.name}
              aria-describedby={form.formState.errors.name ? "group-name-error" : undefined}
              data-testid="group-name-input"
              {...form.register("name")}
            />
            <FieldError
              id="group-name-error"
              testId="group-name-error"
              message={form.formState.errors.name?.message}
            />
          </div>

          <div className="space-y-1.5">
            <Label>{t("groups:create.iconLabel")}</Label>
            <p className="text-[11px] text-muted-foreground">{t("groups:create.iconHelp")}</p>
            <Controller
              control={form.control}
              name="icon"
              render={({ field }) => (
                <IconPicker
                  value={field.value}
                  onChange={field.onChange}
                  disabled={mutation.isPending}
                  testId="group-create-icon-picker"
                />
              )}
            />
            <FieldError testId="group-icon-error" message={form.formState.errors.icon?.message} />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="group-currency">{t("groups:create.currencyLabel")}</Label>
            <Controller
              control={form.control}
              name="group_currency"
              render={({ field }) => (
                <CurrencyCombobox
                  id="group-currency"
                  value={field.value}
                  onChange={(next) => field.onChange(next)}
                  disabled={mutation.isPending}
                  ariaInvalid={!!form.formState.errors.group_currency}
                />
              )}
            />
            <p className="text-[11px] text-muted-foreground">{t("groups:create.currencyHelp")}</p>
            <FieldError
              testId="group-currency-error"
              message={form.formState.errors.group_currency?.message}
            />
          </div>

          <ServerErrorBanner error={serverError} testId="create-group-server-error" />

          <div className="flex justify-end gap-2 pt-2">
            <Button
              type="button"
              variant="ghost"
              onClick={() => navigate(-1)}
              data-testid="create-group-cancel"
            >
              {t("groups:create.cancel")}
            </Button>
            <Button
              type="submit"
              className="gap-2"
              disabled={mutation.isPending}
              data-testid="create-group-submit"
            >
              {mutation.isPending ? t("groups:create.submitting") : t("groups:create.submit")}
              {!mutation.isPending ? <ArrowRight className="size-4" /> : null}
            </Button>
          </div>
        </form>
      </Page>
    </>
  )
}
