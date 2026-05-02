import { useEffect, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Link, Navigate, useNavigate, useSearchParams } from "react-router-dom"
import { ArrowRight, Mail } from "lucide-react"

import { AuthLayout } from "@/components/auth/AuthLayout"
import { OAuthRow } from "@/components/auth/OAuthRow"
import { PasswordInput } from "@/components/auth/PasswordInput"
import { TwoFactorStub } from "@/components/auth/TwoFactorStub"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAuth } from "@/features/auth/AuthContext"
import { useAcceptInvite } from "@/features/invite/hooks"
import { useLogin } from "@/features/auth/hooks"
import { consumePendingInvite, peekPendingInvite } from "@/features/auth/inviteHandoff"
import { loginSchema, type LoginInput } from "@/features/auth/schemas"
import { sanitizeRedirectPath } from "@/lib/safe-redirect"
import { parseServerError } from "@/lib/server-error"
import { RouteTitle } from "@/components/routing/RouteTitle"

const SESSION_REASON_KEY: Record<string, string> = {
  session_expired: "auth:session.expired",
  auth_required: "auth:session.authRequired",
}

// LoginPage — owns the sign-in form, the post-login redirect chain, and the
// invite handoff (#1285): if the user came in through /invite/<token>, we
// auto-accept after login and land them inside the joined group.
export function LoginPage() {
  const { t } = useTranslation()
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const { isAuthenticated } = useAuth()
  const loginMutation = useLogin()
  const acceptInviteMutation = useAcceptInvite()

  const [pendingInvite] = useState(() => peekPendingInvite())
  const [serverError, setServerError] = useState<string | null>(null)

  const form = useForm<LoginInput>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
    mode: "onSubmit",
  })

  const reason = params.get("reason")
  const sessionMessage = reason ? t(SESSION_REASON_KEY[reason] ?? "auth:session.ended") : null

  // Reset the server error whenever the user edits a field, so a stale
  // notice doesn't sit on top of valid input.
  useEffect(() => {
    const sub = form.watch(() => {
      if (serverError) setServerError(null)
    })
    return () => sub.unsubscribe()
  }, [form, serverError])

  // If a deep-link or refresh lands an already-authenticated user here,
  // skip the form and bounce to wherever they were trying to go. Placed
  // after the hooks above to keep the Rules-of-Hooks call order stable.
  // sanitizeRedirectPath rejects absolute / protocol-relative URLs so a
  // crafted ?redirect= query can't open-redirect off the app.
  if (isAuthenticated) {
    return <Navigate to={sanitizeRedirectPath(params.get("redirect"))} replace />
  }

  const isPending =
    loginMutation.isPending || acceptInviteMutation.isPending || form.formState.isSubmitting

  async function onSubmit(values: LoginInput) {
    setServerError(null)
    try {
      await loginMutation.mutateAsync({
        email: values.email.trim(),
        password: values.password,
      })
    } catch (err) {
      setServerError(parseServerError(err, t("auth:login.errorGeneric")))
      return
    }
    if (pendingInvite) {
      try {
        await acceptInviteMutation.mutateAsync(pendingInvite.token)
        consumePendingInvite()
        navigate("/", { replace: true })
        return
      } catch {
        // Fall through to the normal redirect — the user can retry from
        // /invite/<token> manually. We deliberately don't surface this as
        // an error on the login page; the login itself succeeded.
      }
    }
    navigate(sanitizeRedirectPath(params.get("redirect")), { replace: true })
  }

  return (
    <AuthLayout>
      <RouteTitle title={t("stubs:login")} />
      <div className="space-y-6" data-testid="login-page">
        <header className="space-y-1.5">
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth:login.title")}</h1>
          <p className="text-sm text-muted-foreground">{t("auth:login.subtitle")}</p>
        </header>

        {sessionMessage ? (
          <Alert className="session-message" data-testid="session-message">
            <AlertDescription>{sessionMessage}</AlertDescription>
          </Alert>
        ) : null}

        {pendingInvite ? (
          <Alert data-testid="pending-invite-banner">
            <Mail aria-hidden="true" />
            <AlertDescription>
              {pendingInvite.groupName
                ? t("auth:invite.joinGroup", { name: pendingInvite.groupName })
                : t("auth:invite.signInToAccept")}
            </AlertDescription>
          </Alert>
        ) : null}

        <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)} noValidate>
          <div className="space-y-1.5">
            <Label htmlFor="login-email">{t("auth:fields.email")}</Label>
            <div className="relative">
              <Mail
                aria-hidden="true"
                className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground"
              />
              <Input
                id="login-email"
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
              <p className="text-xs text-destructive" data-testid="email-error">
                {t(form.formState.errors.email.message ?? "")}
              </p>
            ) : null}
          </div>

          <div className="space-y-1.5">
            <div className="flex items-center justify-between">
              <Label htmlFor="login-password">{t("auth:fields.password")}</Label>
              <Link
                to="/forgot-password"
                className="text-xs text-muted-foreground hover:text-foreground transition-colors"
              >
                {t("auth:login.forgotPassword")}
              </Link>
            </div>
            <PasswordInput
              id="login-password"
              autoComplete="current-password"
              placeholder={t("auth:fields.passwordPlaceholder")}
              disabled={isPending}
              aria-invalid={!!form.formState.errors.password}
              data-testid="password"
              {...form.register("password")}
            />
            {form.formState.errors.password ? (
              <p className="text-xs text-destructive" data-testid="password-error">
                {t(form.formState.errors.password.message ?? "")}
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
            disabled={isPending}
            data-testid="login-button"
          >
            {isPending ? t("auth:login.submitting") : t("auth:login.submit")}
            {!isPending ? <ArrowRight className="size-4" /> : null}
          </Button>
        </form>

        <OAuthRow />
        <TwoFactorStub />

        <p className="text-center text-sm text-muted-foreground">
          {t("auth:login.dontHaveAccount")}{" "}
          <Link
            to="/register"
            className="font-medium text-foreground hover:underline underline-offset-4"
          >
            {t("auth:login.createOne")}
          </Link>
        </p>
      </div>
    </AuthLayout>
  )
}
