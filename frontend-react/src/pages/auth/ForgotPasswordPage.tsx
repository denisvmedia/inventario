import { useEffect, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { ArrowLeft, ArrowRight, Mail } from "lucide-react"

import { AuthLayout } from "@/components/auth/AuthLayout"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useForgotPassword } from "@/features/auth/hooks"
import { forgotPasswordSchema, type ForgotPasswordInput } from "@/features/auth/schemas"
import { parseServerError } from "@/lib/server-error"
import { RouteTitle } from "@/components/routing/RouteTitle"

// ForgotPasswordPage — single-field form. The server returns an identical
// success message regardless of whether the email exists, so once the form
// resolves we always render the "check your email" state.
export function ForgotPasswordPage() {
  const { t } = useTranslation()
  const mutation = useForgotPassword()
  const [serverError, setServerError] = useState<string | null>(null)
  const [submittedEmail, setSubmittedEmail] = useState<string | null>(null)
  const [successMessage, setSuccessMessage] = useState<string | null>(null)

  const form = useForm<ForgotPasswordInput>({
    resolver: zodResolver(forgotPasswordSchema),
    defaultValues: { email: "" },
  })

  useEffect(() => {
    const sub = form.watch(() => {
      if (serverError) setServerError(null)
    })
    return () => sub.unsubscribe()
  }, [form, serverError])

  async function onSubmit(values: ForgotPasswordInput) {
    setServerError(null)
    try {
      const message = await mutation.mutateAsync(values.email.trim())
      setSubmittedEmail(values.email.trim())
      setSuccessMessage(message || t("auth:forgot.successFallback"))
    } catch (err) {
      setServerError(parseServerError(err, t("auth:forgot.errorGeneric")))
    }
  }

  if (submittedEmail) {
    return (
      <AuthLayout>
        <RouteTitle title={t("stubs:forgotPassword")} />
        <div className="space-y-6 text-center" data-testid="forgot-success">
          <div className="flex justify-center">
            <div className="flex size-16 items-center justify-center rounded-full bg-primary/10">
              <Mail className="size-8 text-primary" aria-hidden="true" />
            </div>
          </div>
          <div className="space-y-1.5">
            <h1 className="text-2xl font-semibold tracking-tight">
              {t("auth:forgot.successTitle")}
            </h1>
            <p className="text-sm text-muted-foreground">
              {successMessage}
              {submittedEmail ? (
                <>
                  {" "}
                  <span className="font-medium text-foreground">{submittedEmail}</span>.
                </>
              ) : null}
            </p>
          </div>
          <div className="rounded-lg border border-border bg-muted/30 p-4 text-left space-y-2">
            <p className="text-xs font-medium text-muted-foreground">
              {t("auth:forgot.didntGetEmail")}
            </p>
            <ul className="text-xs text-muted-foreground space-y-1 list-disc list-inside">
              <li>{t("auth:forgot.checkSpam")}</li>
              <li>{t("auth:forgot.makeSureCorrect")}</li>
            </ul>
            <Button
              variant="outline"
              size="sm"
              className="w-full mt-1"
              type="button"
              disabled={mutation.isPending}
              onClick={() => {
                if (submittedEmail) mutation.mutate(submittedEmail)
              }}
            >
              {t("auth:forgot.resend")}
            </Button>
          </div>
          <Link
            to="/login"
            className="text-sm text-muted-foreground hover:text-foreground transition-colors underline underline-offset-4"
          >
            {t("auth:forgot.backToSignIn")}
          </Link>
        </div>
      </AuthLayout>
    )
  }

  return (
    <AuthLayout>
      <RouteTitle title={t("stubs:forgotPassword")} />
      <div className="space-y-6" data-testid="forgot-page">
        <Link
          to="/login"
          className="flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground transition-colors"
        >
          <ArrowLeft className="size-4" aria-hidden="true" />
          {t("auth:forgot.backToSignIn")}
        </Link>
        <header className="space-y-1.5">
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth:forgot.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("auth:forgot.subtitle")}</p>
        </header>

        <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)} noValidate>
          <div className="space-y-1.5">
            <Label htmlFor="forgot-email">{t("auth:fields.email")}</Label>
            <div className="relative">
              <Mail
                aria-hidden="true"
                className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              />
              <Input
                id="forgot-email"
                type="email"
                autoComplete="email"
                placeholder={t("auth:fields.emailPlaceholder")}
                className="pl-9"
                disabled={mutation.isPending}
                aria-invalid={!!form.formState.errors.email}
                data-testid="email"
                {...form.register("email")}
              />
            </div>
            {form.formState.errors.email ? (
              <p className="text-xs text-destructive" data-testid="email-error">
                {t(form.formState.errors.email.message ?? "")}
              </p>
            ) : null}
          </div>

          {serverError ? (
            <Alert variant="destructive" data-testid="server-error">
              <AlertDescription>{serverError}</AlertDescription>
            </Alert>
          ) : null}

          <Button
            type="submit"
            className="w-full gap-2"
            disabled={mutation.isPending}
            data-testid="submit-button"
          >
            {mutation.isPending ? t("auth:forgot.submitting") : t("auth:forgot.submit")}
            {!mutation.isPending ? <ArrowRight className="size-4" /> : null}
          </Button>
        </form>
      </div>
    </AuthLayout>
  )
}
