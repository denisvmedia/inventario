import { useEffect, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Link, useSearchParams } from "react-router-dom"
import { ArrowRight, CheckCircle2 } from "lucide-react"

import { AuthLayout } from "@/components/auth/AuthLayout"
import { PasswordInput } from "@/components/auth/PasswordInput"
import { PasswordStrengthMeter } from "@/components/auth/PasswordStrengthMeter"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { useResetPassword } from "@/features/auth/hooks"
import { resetPasswordSchema, type ResetPasswordInput } from "@/features/auth/schemas"
import { parseServerError } from "@/lib/server-error"
import { RouteTitle } from "@/components/routing/RouteTitle"

// ResetPasswordPage — token comes from `?token=…`. Three render states:
//   1. Empty/missing token → invalid-link notice with a shortcut to forgot.
//   2. Submitted successfully → success block with "sign in" CTA.
//   3. Otherwise → the password / confirm form.
export function ResetPasswordPage() {
  const { t } = useTranslation()
  const [params] = useSearchParams()
  const token = params.get("token") ?? ""
  const mutation = useResetPassword()
  const [serverError, setServerError] = useState<string | null>(null)
  const [successMessage, setSuccessMessage] = useState<string | null>(null)

  const form = useForm<ResetPasswordInput>({
    resolver: zodResolver(resetPasswordSchema),
    defaultValues: { password: "", confirmPassword: "" },
  })
  const passwordValue = form.watch("password") ?? ""

  useEffect(() => {
    const sub = form.watch(() => {
      if (serverError) setServerError(null)
    })
    return () => sub.unsubscribe()
  }, [form, serverError])

  async function onSubmit(values: ResetPasswordInput) {
    setServerError(null)
    try {
      const message = await mutation.mutateAsync({
        token,
        newPassword: values.password,
      })
      setSuccessMessage(message || t("auth:reset.successFallback"))
    } catch (err) {
      setServerError(parseServerError(err, t("auth:reset.errorGeneric")))
    }
  }

  if (!token) {
    return (
      <AuthLayout>
        <RouteTitle title={t("stubs:resetPassword")} />
        <div className="space-y-6 text-center" data-testid="reset-missing-token">
          <div className="space-y-1.5">
            <h1 className="text-2xl font-semibold tracking-tight">
              {t("auth:reset.missingTokenTitle")}
            </h1>
            <p className="text-sm text-muted-foreground">{t("auth:reset.missingTokenBody")}</p>
          </div>
          <Button asChild className="w-full">
            <Link to="/forgot-password">{t("auth:reset.requestNewLink")}</Link>
          </Button>
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

  if (successMessage) {
    return (
      <AuthLayout>
        <RouteTitle title={t("stubs:resetPassword")} />
        <div className="space-y-6 text-center" data-testid="reset-success">
          <div className="flex justify-center">
            <div className="flex size-16 items-center justify-center rounded-full bg-emerald-500/10">
              <CheckCircle2 className="size-8 text-emerald-500" aria-hidden="true" />
            </div>
          </div>
          <div className="space-y-1.5">
            <h1 className="text-2xl font-semibold tracking-tight">
              {t("auth:reset.successTitle")}
            </h1>
            <p className="text-sm text-muted-foreground">{successMessage}</p>
          </div>
          <Button asChild className="w-full gap-2">
            <Link to="/login">
              {t("auth:reset.signInWithNew")}
              <ArrowRight className="size-4" />
            </Link>
          </Button>
        </div>
      </AuthLayout>
    )
  }

  return (
    <AuthLayout>
      <RouteTitle title={t("stubs:resetPassword")} />
      <div className="space-y-6" data-testid="reset-page">
        <header className="space-y-1.5">
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth:reset.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("auth:reset.subtitle")}</p>
        </header>

        <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)} noValidate>
          <div className="space-y-1.5">
            <Label htmlFor="reset-password">{t("auth:fields.newPassword")}</Label>
            <PasswordInput
              id="reset-password"
              autoComplete="new-password"
              placeholder={t("auth:fields.newPasswordPlaceholder")}
              disabled={mutation.isPending}
              aria-invalid={!!form.formState.errors.password}
              data-testid="password"
              {...form.register("password")}
            />
            <PasswordStrengthMeter password={passwordValue} testId="reset-password-strength" />
            {form.formState.errors.password ? (
              <p className="text-xs text-destructive" data-testid="password-error">
                {t(form.formState.errors.password.message ?? "")}
              </p>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="reset-confirm">{t("auth:fields.confirmPassword")}</Label>
            <PasswordInput
              id="reset-confirm"
              autoComplete="new-password"
              placeholder={t("auth:fields.confirmPasswordPlaceholder")}
              disabled={mutation.isPending}
              aria-invalid={!!form.formState.errors.confirmPassword}
              data-testid="confirm-password"
              {...form.register("confirmPassword")}
            />
            {form.formState.errors.confirmPassword ? (
              <p className="text-xs text-destructive" data-testid="confirm-password-error">
                {t(form.formState.errors.confirmPassword.message ?? "")}
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
            {mutation.isPending ? t("auth:reset.submitting") : t("auth:reset.submit")}
            {!mutation.isPending ? <ArrowRight className="size-4" /> : null}
          </Button>
        </form>
      </div>
    </AuthLayout>
  )
}
