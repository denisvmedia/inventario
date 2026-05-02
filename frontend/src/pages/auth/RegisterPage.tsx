import { useEffect, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Link, useNavigate } from "react-router-dom"
import { ArrowRight, CheckCircle2, Mail, User } from "lucide-react"

import { AuthLayout } from "@/components/auth/AuthLayout"
import { OAuthRow } from "@/components/auth/OAuthRow"
import { PasswordInput } from "@/components/auth/PasswordInput"
import { PasswordStrengthMeter } from "@/components/auth/PasswordStrengthMeter"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAcceptInvite } from "@/features/invite/hooks"
import { useLogin, useRegister } from "@/features/auth/hooks"
import { consumePendingInvite, peekPendingInvite } from "@/features/auth/inviteHandoff"
import { registerSchema, type RegisterInput } from "@/features/auth/schemas"
import { parseServerError } from "@/lib/server-error"
import { RouteTitle } from "@/components/routing/RouteTitle"

// RegisterPage — sign-up form with the optional invite-to-handoff path.
// The server returns the same generic message whether the email is taken or
// not (anti-enumeration), so we always surface success unless it 4xx/5xxs.
export function RegisterPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const registerMutation = useRegister()
  const loginMutation = useLogin()
  const acceptInviteMutation = useAcceptInvite()

  const [pendingInvite] = useState(() => peekPendingInvite())
  const [serverError, setServerError] = useState<string | null>(null)
  const [successMessage, setSuccessMessage] = useState<string | null>(null)
  const [autoAcceptError, setAutoAcceptError] = useState<string | null>(null)

  const form = useForm<RegisterInput>({
    resolver: zodResolver(registerSchema),
    defaultValues: {
      name: "",
      email: "",
      password: "",
      acceptTerms: false,
    },
  })

  const passwordValue = form.watch("password") ?? ""

  // Reset stale server error on edits.
  useEffect(() => {
    const sub = form.watch(() => {
      if (serverError) setServerError(null)
    })
    return () => sub.unsubscribe()
  }, [form, serverError])

  const isPending =
    registerMutation.isPending ||
    loginMutation.isPending ||
    acceptInviteMutation.isPending ||
    form.formState.isSubmitting

  async function onSubmit(values: RegisterInput) {
    setServerError(null)
    setAutoAcceptError(null)
    let message: string
    try {
      message = await registerMutation.mutateAsync({
        email: values.email.trim(),
        password: values.password,
        name: values.name.trim(),
        invite_token: pendingInvite?.token,
      })
    } catch (err) {
      setServerError(parseServerError(err, t("auth:register.errorGeneric")))
      return
    }
    setSuccessMessage(message || t("auth:register.successFallback"))
    if (pendingInvite) {
      try {
        await loginMutation.mutateAsync({
          email: values.email.trim(),
          password: values.password,
        })
        await acceptInviteMutation.mutateAsync(pendingInvite.token)
        consumePendingInvite()
        navigate("/", { replace: true })
      } catch (err) {
        setAutoAcceptError(parseServerError(err, t("auth:register.autoAcceptError")))
      }
    }
  }

  if (successMessage) {
    return (
      <AuthLayout>
        <RouteTitle title={t("stubs:register")} />
        <div className="success-message space-y-6 text-center" data-testid="register-success">
          <div className="flex justify-center">
            <div className="flex size-16 items-center justify-center rounded-full bg-primary/10">
              <CheckCircle2 className="size-8 text-primary" aria-hidden="true" />
            </div>
          </div>
          <div className="space-y-1.5">
            <h1 className="text-2xl font-semibold tracking-tight">
              {t("auth:register.successTitle")}
            </h1>
            <p className="text-sm text-muted-foreground">{successMessage}</p>
          </div>
          {pendingInvite && !autoAcceptError ? (
            <p className="text-sm text-muted-foreground">{t("auth:register.joiningGroup")}</p>
          ) : null}
          {autoAcceptError ? (
            <Alert variant="destructive" data-testid="auto-accept-error">
              <AlertDescription>
                {autoAcceptError}{" "}
                <Link
                  to={`/login?redirect=/invite/${encodeURIComponent(pendingInvite?.token ?? "")}`}
                  className="font-medium underline underline-offset-4"
                >
                  {t("auth:register.signInManually")}
                </Link>
              </AlertDescription>
            </Alert>
          ) : null}
          {!pendingInvite ? (
            <p className="text-sm text-muted-foreground">
              <Link
                to="/login"
                className="font-medium text-foreground hover:underline underline-offset-4"
              >
                {t("auth:forgot.backToSignIn")}
              </Link>
            </p>
          ) : null}
        </div>
      </AuthLayout>
    )
  }

  return (
    <AuthLayout>
      <RouteTitle title={t("stubs:register")} />
      <div className="space-y-6" data-testid="register-page">
        <header className="space-y-1.5">
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth:register.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("auth:register.subtitle")}</p>
        </header>

        {pendingInvite ? (
          <Alert data-testid="pending-invite-banner">
            <Mail aria-hidden="true" />
            <AlertDescription>
              {t("auth:invite.joinGroup", {
                name: pendingInvite.groupName ?? "",
              })}
            </AlertDescription>
          </Alert>
        ) : null}

        <form
          className="register-form-content space-y-4"
          onSubmit={form.handleSubmit(onSubmit)}
          noValidate
        >
          <div className="space-y-1.5">
            <Label htmlFor="register-name">{t("auth:fields.name")}</Label>
            <div className="relative">
              <User
                aria-hidden="true"
                className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              />
              <Input
                id="register-name"
                autoComplete="name"
                placeholder={t("auth:fields.namePlaceholder")}
                className="pl-9"
                disabled={isPending}
                aria-invalid={!!form.formState.errors.name}
                data-testid="name"
                {...form.register("name")}
              />
            </div>
            {form.formState.errors.name ? (
              <p className="error-message text-xs text-destructive" data-testid="name-error">
                {t(form.formState.errors.name.message ?? "")}
              </p>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="register-email">{t("auth:fields.email")}</Label>
            <div className="relative">
              <Mail
                aria-hidden="true"
                className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              />
              <Input
                id="register-email"
                type="email"
                autoComplete="email"
                placeholder={t("auth:fields.emailPlaceholder")}
                className="pl-9"
                disabled={isPending}
                aria-invalid={!!form.formState.errors.email}
                data-testid="email"
                {...form.register("email")}
              />
            </div>
            {form.formState.errors.email ? (
              <p className="error-message text-xs text-destructive" data-testid="email-error">
                {t(form.formState.errors.email.message ?? "")}
              </p>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="register-password">{t("auth:fields.password")}</Label>
            <PasswordInput
              id="register-password"
              autoComplete="new-password"
              placeholder={t("auth:fields.newPasswordPlaceholder")}
              disabled={isPending}
              aria-invalid={!!form.formState.errors.password}
              data-testid="password"
              {...form.register("password")}
            />
            <PasswordStrengthMeter password={passwordValue} testId="register-password-strength" />
            {form.formState.errors.password ? (
              <p className="error-message text-xs text-destructive" data-testid="password-error">
                {t(form.formState.errors.password.message ?? "")}
              </p>
            ) : null}
          </div>

          <div className="flex items-start gap-2 pt-1">
            <Checkbox
              id="register-terms"
              checked={form.watch("acceptTerms")}
              onCheckedChange={(v) =>
                form.setValue("acceptTerms", v === true, {
                  shouldValidate: form.formState.isSubmitted,
                })
              }
              aria-invalid={!!form.formState.errors.acceptTerms}
              data-testid="terms"
            />
            <Label
              htmlFor="register-terms"
              className="text-sm font-normal text-muted-foreground leading-relaxed"
            >
              {t("auth:register.termsAccept")}
            </Label>
          </div>
          {form.formState.errors.acceptTerms ? (
            <p className="error-message text-xs text-destructive" data-testid="terms-error">
              {t(form.formState.errors.acceptTerms.message ?? "")}
            </p>
          ) : null}

          {serverError ? (
            <Alert variant="destructive" className="error-message" data-testid="server-error">
              <AlertDescription>{serverError}</AlertDescription>
            </Alert>
          ) : null}

          <Button
            type="submit"
            className="w-full gap-2"
            // Stay disabled until name + email + password are all non-empty
            // so the submit button only lights up when the form has minimal
            // payload. Terms acceptance is intentionally NOT in the disabled
            // condition — the button must still be clickable so the zod
            // resolver can surface a `terms-error` field message; the e2e
            // suite drives this exact path on an unchecked terms box.
            disabled={
              isPending ||
              !form.watch("name")?.trim() ||
              !form.watch("email")?.trim() ||
              !form.watch("password")
            }
            data-testid="register-button"
          >
            {isPending ? t("auth:register.submitting") : t("auth:register.submit")}
            {!isPending ? <ArrowRight className="size-4" /> : null}
          </Button>
        </form>

        <OAuthRow />

        <p className="text-center text-sm text-muted-foreground">
          {t("auth:register.alreadyHaveAccount")}{" "}
          <Link
            to="/login"
            className="font-medium text-foreground hover:underline underline-offset-4"
          >
            {t("auth:register.signIn")}
          </Link>
        </p>
      </div>
    </AuthLayout>
  )
}
