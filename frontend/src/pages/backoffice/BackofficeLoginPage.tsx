import { useEffect, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Navigate, useNavigate, useSearchParams } from "react-router-dom"
import { AlertTriangle, ArrowRight, Mail } from "lucide-react"

import { BackofficeAuthLayout } from "@/components/backoffice/BackofficeAuthLayout"
import { BackofficeMFAChallenge } from "@/components/backoffice/BackofficeMFAChallenge"
import { PasswordInput } from "@/components/auth/PasswordInput"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useBackofficeAuth } from "@/features/backoffice/auth/context"
import { useBackofficeLogin } from "@/features/backoffice/auth/hooks"
import { loginSchema, type LoginInput } from "@/features/auth/schemas"
import { sanitizeRedirectPath } from "@/lib/safe-redirect"
import { parseServerError } from "@/lib/server-error"
import { RouteTitle } from "@/components/routing/RouteTitle"

const SESSION_REASON_KEY: Record<string, string> = {
  session_expired: "backoffice:session.expired",
  auth_required: "backoffice:session.authRequired",
}

// BackofficeLoginPage owns sign-in for the back-office (platform-operator)
// auth plane (#1785 Phase 6). Visually + textually distinct from the tenant
// LoginPage so an operator can never confuse which surface they're on:
//   - own URL  (/backoffice/login, not /login)
//   - own brand chip + slate background
//   - own copy ("Back-office sign in" instead of "Welcome back")
//   - own MFA challenge surface bound to the back-office completeMFA mutation
//   - own redirect default (/admin/tenants, the back-office landing)
//
// The MFA flow has three terminal states:
//   1. kind="ok"              → tokens persisted, navigate onward.
//   2. kind="mfaRequired"     → swap the form for <BackofficeMFAChallenge>.
//   3. kind="mfaNotEnrolled"  → 501 enrollment-missing. Show the CLI nudge;
//                               the operator must run
//                               `inventario backoffice mfa setup --email <e>`
//                               before they can sign in (fail-closed by design).
export function BackofficeLoginPage() {
  const { t } = useTranslation("backoffice")
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { isAuthenticated } = useBackofficeAuth()
  const loginMutation = useBackofficeLogin()

  const [serverError, setServerError] = useState<string | null>(null)
  const [mfaChallenge, setMfaChallenge] = useState<{ mfaToken: string; email: string } | null>(null)
  const [mfaNotEnrolled, setMfaNotEnrolled] = useState<{ email: string } | null>(null)

  const form = useForm<LoginInput>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
    mode: "onSubmit",
  })

  const reason = params.get("reason")
  const sessionMessage = reason ? t(SESSION_REASON_KEY[reason] ?? "session.ended") : null

  // Clear the server error / enrollment nudge when the operator edits
  // a field so stale notices don't sit on top of valid input.
  useEffect(() => {
    const sub = form.watch(() => {
      if (serverError) setServerError(null)
      if (mfaNotEnrolled) setMfaNotEnrolled(null)
    })
    return () => sub.unsubscribe()
  }, [form, serverError, mfaNotEnrolled])

  // Already signed into the back-office plane? Skip the form and bounce
  // to the post-login target. sanitizeRedirectPath rejects off-app URLs
  // so a crafted ?redirect= can't open-redirect off the app. Default
  // target is /admin/tenants (the back-office landing).
  if (isAuthenticated) {
    const target = sanitizeRedirectPath(params.get("redirect"))
    return <Navigate to={target === "/" ? "/admin/tenants" : target} replace />
  }

  const isPending = loginMutation.isPending || form.formState.isSubmitting

  function finalizeLogin() {
    const sanitized = sanitizeRedirectPath(params.get("redirect"))
    navigate(sanitized === "/" ? "/admin/tenants" : sanitized, { replace: true })
  }

  async function onSubmit(values: LoginInput) {
    setServerError(null)
    setMfaNotEnrolled(null)
    try {
      const outcome = await loginMutation.mutateAsync({
        email: values.email.trim(),
        password: values.password,
      })
      if (outcome.kind === "mfaRequired") {
        setMfaChallenge({ mfaToken: outcome.mfaToken, email: outcome.email })
        return
      }
      if (outcome.kind === "mfaNotEnrolled") {
        setMfaNotEnrolled({ email: outcome.email })
        return
      }
    } catch (err) {
      setServerError(parseServerError(err, t("login.errorGeneric")))
      return
    }
    finalizeLogin()
  }

  if (mfaChallenge) {
    return (
      <BackofficeAuthLayout>
        <RouteTitle title={t("login.routeTitle")} />
        <BackofficeMFAChallenge
          mfaToken={mfaChallenge.mfaToken}
          email={mfaChallenge.email}
          onSuccess={finalizeLogin}
          onCancel={() => {
            setMfaChallenge(null)
            form.reset({ email: mfaChallenge.email, password: "" })
          }}
        />
      </BackofficeAuthLayout>
    )
  }

  return (
    <BackofficeAuthLayout>
      <RouteTitle title={t("login.routeTitle")} />
      <div className="space-y-6" data-testid="backoffice-login-page">
        <header className="space-y-1.5">
          <h1 className="text-2xl font-semibold tracking-tight">{t("login.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("login.subtitle")}</p>
        </header>

        {sessionMessage ? (
          <Alert className="session-message" data-testid="backoffice-session-message">
            <AlertDescription>{sessionMessage}</AlertDescription>
          </Alert>
        ) : null}

        {mfaNotEnrolled ? (
          <Alert variant="destructive" data-testid="backoffice-mfa-not-enrolled">
            <AlertTriangle aria-hidden="true" />
            <AlertTitle>{t("login.mfaNotEnrolled.title")}</AlertTitle>
            <AlertDescription>
              {t("login.mfaNotEnrolled.body", { email: mfaNotEnrolled.email })}{" "}
              <code className="rounded bg-muted px-1 py-0.5 text-xs">
                {t("login.mfaNotEnrolled.cli", { email: mfaNotEnrolled.email })}
              </code>
            </AlertDescription>
          </Alert>
        ) : null}

        <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)} noValidate>
          <div className="space-y-1.5">
            <Label htmlFor="backoffice-login-email">{t("fields.email")}</Label>
            <div className="relative">
              <Mail
                aria-hidden="true"
                className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              />
              <Input
                id="backoffice-login-email"
                type="email"
                autoComplete="email"
                placeholder={t("fields.emailPlaceholder")}
                className="pl-9"
                disabled={isPending}
                aria-invalid={!!form.formState.errors.email}
                data-testid="backoffice-email"
                {...form.register("email")}
              />
            </div>
            {form.formState.errors.email ? (
              <p className="text-xs text-destructive" data-testid="backoffice-email-error">
                {t(form.formState.errors.email.message ?? "")}
              </p>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="backoffice-login-password">{t("fields.password")}</Label>
            <PasswordInput
              id="backoffice-login-password"
              autoComplete="current-password"
              placeholder={t("fields.passwordPlaceholder")}
              disabled={isPending}
              aria-invalid={!!form.formState.errors.password}
              data-testid="backoffice-password"
              {...form.register("password")}
            />
            {form.formState.errors.password ? (
              <p className="text-xs text-destructive" data-testid="backoffice-password-error">
                {t(form.formState.errors.password.message ?? "")}
              </p>
            ) : null}
          </div>

          {serverError ? (
            <Alert variant="destructive" data-testid="backoffice-server-error">
              <AlertDescription>{serverError}</AlertDescription>
            </Alert>
          ) : null}

          <Button
            type="submit"
            className="w-full gap-2"
            disabled={isPending}
            data-testid="backoffice-login-button"
          >
            {isPending ? t("login.submitting") : t("login.submit")}
            {!isPending ? <ArrowRight className="size-4" /> : null}
          </Button>
        </form>

        <p className="text-center text-xs text-muted-foreground">{t("login.disclaimer")}</p>
      </div>
    </BackofficeAuthLayout>
  )
}
