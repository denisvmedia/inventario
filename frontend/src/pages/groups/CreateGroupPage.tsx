import { useEffect, useState } from "react"
import { useForm, Controller } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { ArrowLeft, ArrowRight, Building2 } from "lucide-react"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { CurrencyCombobox } from "@/components/CurrencyCombobox"
import { IconPicker } from "@/components/groups/IconPicker"
import { useCreateGroup } from "@/features/group/hooks"
import { createGroupSchema, type CreateGroupInput } from "@/features/group/schemas"
import { useAppToast } from "@/hooks/useAppToast"
import { parseServerError } from "@/lib/server-error"
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
  const [serverError, setServerError] = useState<string | null>(null)

  const form = useForm<CreateGroupInput>({
    resolver: zodResolver(createGroupSchema),
    defaultValues: { name: "", icon: "", main_currency: "USD" },
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
        main_currency: values.main_currency.toUpperCase(),
      })
      toast.success(t("groups:create.successToast"))
      // Server-generated slug is the canonical address. If the response
      // is missing one (defensive), fall back to /no-group rather than
      // build a broken URL.
      if (created.slug) {
        navigate(`/g/${encodeURIComponent(created.slug)}`)
      } else {
        navigate("/no-group")
      }
    } catch (err) {
      setServerError(parseServerError(err, t("groups:create.errorGeneric")))
    }
  }

  return (
    <>
      <RouteTitle title={t("groups:create.title")} />
      <div className="mx-auto flex w-full max-w-xl flex-col gap-8" data-testid="create-group-page">
        <div className="space-y-1">
          {/* "Back" returns to wherever the user came from rather than
              hard-coding /no-group: this page is reachable both from
              onboarding (zero groups) and from the GroupSelector (creating
              an additional group), and either label/destination would be
              wrong half the time. */}
          <button
            type="button"
            onClick={() => navigate(-1)}
            data-testid="create-group-back"
            className="inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
          >
            <ArrowLeft className="size-4" aria-hidden="true" />
            {t("common:actions.back")}
          </button>
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10">
              <Building2 className="size-5 text-primary" aria-hidden="true" />
            </div>
            <div>
              <h1 className="text-2xl font-semibold tracking-tight">{t("groups:create.title")}</h1>
              <p className="text-sm text-muted-foreground">{t("groups:create.subtitle")}</p>
            </div>
          </div>
        </div>

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
              data-testid="group-name-input"
              {...form.register("name")}
            />
            {form.formState.errors.name ? (
              <p className="text-xs text-destructive" data-testid="group-name-error">
                {t(form.formState.errors.name.message ?? "")}
              </p>
            ) : null}
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
            {form.formState.errors.icon ? (
              <p className="text-xs text-destructive" data-testid="group-icon-error">
                {t(form.formState.errors.icon.message ?? "")}
              </p>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="group-currency">{t("groups:create.currencyLabel")}</Label>
            <Controller
              control={form.control}
              name="main_currency"
              render={({ field }) => (
                <CurrencyCombobox
                  id="group-currency"
                  value={field.value}
                  onChange={(next) => field.onChange(next)}
                  disabled={mutation.isPending}
                  ariaInvalid={!!form.formState.errors.main_currency}
                />
              )}
            />
            <p className="text-[11px] text-muted-foreground">{t("groups:create.currencyHelp")}</p>
            {form.formState.errors.main_currency ? (
              <p className="text-xs text-destructive" data-testid="group-currency-error">
                {t(form.formState.errors.main_currency.message ?? "")}
              </p>
            ) : null}
          </div>

          {serverError ? (
            <Alert variant="destructive" data-testid="create-group-server-error">
              <AlertDescription>{serverError}</AlertDescription>
            </Alert>
          ) : null}

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
      </div>
    </>
  )
}
