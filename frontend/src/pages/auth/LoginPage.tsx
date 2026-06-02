import { useEffect, useState } from "react"
import { useForm } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Link, Navigate, useNavigate, useSearchParams } from "react-router-dom"
import { ArrowRight, Link2, Mail, Sparkles } from "lucide-react"

import { AuthLayout } from "@/components/auth/AuthLayout"
import { MFAChallenge } from "@/components/auth/MFAChallenge"
import { OAuthRow } from "@/components/auth/OAuthRow"
import { PasswordInput } from "@/components/auth/PasswordInput"
import { PendingFirstItemBanner } from "@/components/auth/PendingFirstItemBanner"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAuth } from "@/features/auth/AuthContext"
import { useAcceptInvite } from "@/features/invite/hooks"
import { useLogin, useRequestMagicLink } from "@/features/auth/hooks"
import { consumePendingInvite, peekPendingInvite } from "@/features/auth/inviteHandoff"
import { peekPendingFirstItem } from "@/features/auth/firstItemHandoff"
import { loginSchema, type LoginInput } from "@/features/auth/schemas"
import { useFeatureFlag } from "@/features/feature-flags/hooks"
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
  const requestMagicLinkMutation = useRequestMagicLink()

  // Passwordless sign-in entry point (#magic-link). Gated on the public
  // feature flag read at boot — same pattern as the currency-migration
  // wizard (#1616) — so the affordance stays hidden when the BE has the
  // feature off (which makes /auth/magic-link/* return a coded 404).
  const magicLinkEnabled = useFeatureFlag("magic_link_login")

  const [pendingInvite] = useState(() => peekPendingInvite())
  // Anonymous first-item handoff (#1988): when the visitor drafted an item
  // on the landing page before logging in, reassure them their entry is
  // safe and will be added after sign-in (the actual replay happens at
  // /welcome via finalizeLogin → FirstItemResolver). Peek once at mount —
  // the resolver owns consumption.
  const [pendingFirstItem] = useState(() => peekPendingFirstItem())
  const [serverError, setServerError] = useState<string | null>(null)
  // mfaChallenge holds the step-1 → step-2 handoff. When non-null,
  // <MFAChallenge> takes over the page and the password form is hidden.
  const [mfaChallenge, setMfaChallenge] = useState<{ mfaToken: string; email: string } | null>(null)
  // magicLinkSent flips to the neutral "check your inbox" confirmation
  // once the request resolves. The server returns the same 200 whether or
  // not the email maps to an active account (anti-enumeration), so we show
  // the confirmation unconditionally — mirrors ForgotPasswordPage.
  const [magicLinkSent, setMagicLinkSent] = useState(false)

  const form = useForm<LoginInput>({
    resolver: zodResolver(loginSchema),
    defaultValues: { email: "", password: "" },
    mode: "onSubmit",
  })

  const reason = params.get("reason")
  const sessionMessage = reason ? t(SESSION_REASON_KEY[reason] ?? "auth:session.ended") : null

  // OAuth link-required banner (#1394). BE 302s here with
  // `?oauth_link_required=1&email=…&provider=…` when the provider's profile
  // matched a local user whose email is NOT verified at the provider — we
  // refuse to auto-link and instead prompt the user to sign in with their
  // password so they can link the provider from Settings → Connected
  // Accounts. The banner stays visible across edits (it's a state of the
  // page, not a stale server error).
  const oauthLinkRequired = params.get("oauth_link_required") === "1"
  const oauthLinkEmail = params.get("email") ?? ""
  const oauthLinkProvider = params.get("provider") ?? ""
  // OAuth error banner (#1394). The BE can append `?oauth_error=state` for a
  // signed-state mismatch (CSRF / replay) or `?oauth_error=provider` for an
  // exchange / network failure. Both surfaces stay separate from
  // `oauth_link_required` because they need a destructive Alert variant.
  const oauthErrorCode = params.get("oauth_error")
  const oauthErrorMessage = (() => {
    if (oauthErrorCode === "state") return t("auth:oauth.errorState")
    if (oauthErrorCode === "provider")
      return t("auth:oauth.errorProvider", {
        provider: oauthLinkProvider || t("auth:oauth.providerFallback"),
      })
    return null
  })()

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
  const isMagicLinkPending = requestMagicLinkMutation.isPending
  // Both actions share the email field, so lock the whole form while
  // either is in flight to avoid a half-submitted state.
  const isBusy = isPending || isMagicLinkPending

  // Post-login redirect chain: invite-accept (#1285), then ?redirect, then /.
  // Extracted so both the password-only path and the MFA-completion path
  // call into the same logic — the only difference is who awaits it.
  async function finalizeLogin() {
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
    // Anonymous first-item handoff (#1988): if the user drafted an item on
    // the landing page before logging in, route them to /welcome where
    // FirstItemResolver replays the stash into their group. Takes
    // precedence over the ?redirect default (which the landing's Add CTA
    // sets to /welcome anyway — this also covers the case where the marker
    // is set but ?redirect was lost across an OAuth round-trip). Peek, not
    // consume: the resolver owns consumption so a failed replay can retry.
    if (peekPendingFirstItem()) {
      navigate("/welcome", { replace: true })
      return
    }
    navigate(sanitizeRedirectPath(params.get("redirect")), { replace: true })
  }

  async function onSubmit(values: LoginInput) {
    setServerError(null)
    try {
      const outcome = await loginMutation.mutateAsync({
        email: values.email.trim(),
        password: values.password,
      })
      if (outcome.kind === "mfa_required") {
        // Stash the challenge so <MFAChallenge> can take over without
        // re-asking the password — we hand it the mfa_token + email
        // and wait for the second step.
        setMfaChallenge({ mfaToken: outcome.mfaToken, email: outcome.email })
        return
      }
    } catch (err) {
      setServerError(parseServerError(err, t("auth:login.errorGeneric")))
      return
    }
    await finalizeLogin()
  }

  // "Email me a sign-in link" reuses the email field rather than adding a
  // second input. We validate just that one field through RHF (so an empty
  // email surfaces the same inline error as a normal submit) before firing
  // the request. The MFA branch is NOT handled here — magic links are
  // verified on /magic-link, where mfa_required (if any) is surfaced; the
  // request step never returns a challenge.
  async function onSendMagicLink() {
    setServerError(null)
    const valid = await form.trigger("email")
    if (!valid) return
    const email = form.getValues("email").trim()
    try {
      await requestMagicLinkMutation.mutateAsync(email)
      setMagicLinkSent(true)
    } catch (err) {
      setServerError(parseServerError(err, t("auth:magicLink.errorGeneric")))
    }
  }

  // When MFA is required, swap the form for the code-entry surface.
  // Cancelling drops the challenge state and goes back to step 1 —
  // the mfa_token becomes a dead letter, which is fine, it expires in
  // 5 minutes server-side anyway.
  if (mfaChallenge) {
    return (
      <AuthLayout>
        <RouteTitle title={t("stubs:login")} />
        <MFAChallenge
          mfaToken={mfaChallenge.mfaToken}
          email={mfaChallenge.email}
          onSuccess={() => {
            void finalizeLogin()
          }}
          onCancel={() => {
            setMfaChallenge(null)
            form.reset({ email: mfaChallenge.email, password: "" })
          }}
        />
      </AuthLayout>
    )
  }

  // Neutral "check your inbox" confirmation after a magic-link request.
  // Anti-enumeration: shown regardless of whether the email exists, with
  // no detail that could confirm an account. Mirrors ForgotPasswordPage's
  // success state.
  if (magicLinkSent) {
    return (
      <AuthLayout>
        <RouteTitle title={t("stubs:login")} />
        <div className="space-y-6 text-center" data-testid="magic-link-sent">
          <div className="flex justify-center">
            <div className="flex size-16 items-center justify-center rounded-full bg-primary/10">
              <Mail className="size-8 text-primary" aria-hidden="true" />
            </div>
          </div>
          <div className="space-y-1.5">
            <h1 className="text-2xl font-semibold tracking-tight">
              {t("auth:magicLink.checkInbox")}
            </h1>
            <p className="text-sm text-muted-foreground">{t("auth:magicLink.checkInboxBody")}</p>
          </div>
          <button
            type="button"
            onClick={() => {
              setMagicLinkSent(false)
              setServerError(null)
            }}
            className="text-sm text-muted-foreground hover:text-foreground transition-colors underline underline-offset-4"
            data-testid="magic-link-back"
          >
            {t("auth:magicLink.backToSignIn")}
          </button>
        </div>
      </AuthLayout>
    )
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

        {oauthLinkRequired ? (
          <Alert data-testid="oauth-link-required-banner">
            <Link2 aria-hidden="true" />
            <AlertDescription>
              {t("auth:oauth.linkRequired", {
                provider: oauthLinkProvider || t("auth:oauth.providerFallback"),
                email: oauthLinkEmail || t("auth:oauth.emailFallback"),
              })}
            </AlertDescription>
          </Alert>
        ) : null}

        {oauthErrorMessage ? (
          <Alert variant="destructive" data-testid="oauth-error-banner">
            <AlertDescription>{oauthErrorMessage}</AlertDescription>
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

        {pendingFirstItem ? <PendingFirstItemBanner /> : null}

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
                disabled={isBusy}
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
              disabled={isBusy}
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
            disabled={isBusy}
            data-testid="login-button"
          >
            {isPending ? t("auth:login.submitting") : t("auth:login.submit")}
            {!isPending ? <ArrowRight className="size-4" /> : null}
          </Button>

          {magicLinkEnabled ? (
            <Button
              type="button"
              variant="outline"
              className="w-full gap-2"
              disabled={isBusy}
              onClick={onSendMagicLink}
              data-testid="magic-link-button"
            >
              <Sparkles className="size-4" aria-hidden="true" />
              {isMagicLinkPending ? t("auth:magicLink.sending") : t("auth:magicLink.sendCta")}
            </Button>
          ) : null}
        </form>

        <OAuthRow />

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
