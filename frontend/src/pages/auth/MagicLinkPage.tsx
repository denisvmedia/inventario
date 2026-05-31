import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useSearchParams } from "react-router-dom"
import { Clock } from "lucide-react"

import { AuthLayout } from "@/components/auth/AuthLayout"
import { MFAChallenge } from "@/components/auth/MFAChallenge"
import { Button } from "@/components/ui/button"
import { useMagicLinkVerify } from "@/features/auth/hooks"
import { sanitizeRedirectPath } from "@/lib/safe-redirect"
import { RouteTitle } from "@/components/routing/RouteTitle"

// MagicLinkPage — the landing surface for the one-time sign-in link. The
// backend's email points here at /magic-link?token=<token>, so we read the
// token from the query, auto-POST it on mount, and resolve into one of:
//
//   - verifying   request in flight (default while we wait).
//   - success     tokens stored — redirect to ?redirect= or "/". Rendered as
//                 a brief <Navigate> via the effect, so there's no standalone
//                 success screen (the app shell is the success state).
//   - mfa         the user has TOTP enrolled — hand off to <MFAChallenge>,
//                 exactly like password login does (no MFA bypass).
//   - error       invalid / expired / replayed token — show the "request a
//                 new link" CTA back to /login.
//
// Mirrors VerifyEmailPage's auto-verify-on-mount shape; the difference is the
// success path logs the user in (verifyMagicLink already stored the tokens)
// and the MFA branch reuses the shared <MFAChallenge> component.
type VerifyState = "verifying" | "mfa" | "error"

export function MagicLinkPage() {
  const { t } = useTranslation()
  const [params] = useSearchParams()
  const navigate = useNavigate()
  const token = params.get("token") ?? ""
  const redirectTarget = sanitizeRedirectPath(params.get("redirect"))
  const verifyMutation = useMagicLinkVerify()

  const [state, setState] = useState<VerifyState>(token ? "verifying" : "error")
  const [mfaToken, setMfaToken] = useState<string | null>(null)
  const [mfaEmail, setMfaEmail] = useState("")

  useEffect(() => {
    // Sync the external `?token=` query → local verify state, then fire the
    // mutation. The state resets up front are part of synchronising with an
    // external system (the API).
    /* eslint-disable react-hooks/set-state-in-effect */
    if (!token) {
      setState("error")
      return
    }
    setState("verifying")
    setMfaToken(null)
    /* eslint-enable react-hooks/set-state-in-effect */
    let cancelled = false
    verifyMutation
      .mutateAsync(token)
      .then((outcome) => {
        if (cancelled) return
        if (outcome.kind === "mfa_required") {
          // The user has MFA — defer the session to the second step. No
          // tokens were stored; <MFAChallenge> consumes the mfa_token and
          // stores them on success.
          setMfaToken(outcome.mfaToken)
          setMfaEmail(outcome.email)
          setState("mfa")
          return
        }
        // Non-MFA success: verifyMagicLink already persisted the tokens and
        // useMagicLinkVerify seeded the user cache. Land the user where they
        // were headed (or "/"), replacing history so Back doesn't return to
        // the now-consumed link.
        navigate(redirectTarget, { replace: true })
      })
      .catch(() => {
        if (cancelled) return
        setState("error")
      })
    return () => {
      cancelled = true
    }
    // mutateAsync identity is stable per render; re-run on token change only.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token])

  if (state === "mfa" && mfaToken) {
    return (
      <AuthLayout>
        <RouteTitle title={t("stubs:login")} />
        <MFAChallenge
          mfaToken={mfaToken}
          email={mfaEmail}
          onSuccess={() => {
            navigate(redirectTarget, { replace: true })
          }}
          onCancel={() => {
            // Cancelling the challenge drops the dead mfa_token and sends the
            // user back to the login page to start over (request a new link
            // or sign in with a password).
            navigate("/login", { replace: true })
          }}
        />
      </AuthLayout>
    )
  }

  return (
    <AuthLayout>
      <RouteTitle title={t("stubs:login")} />
      {state === "verifying" ? (
        <div className="space-y-6 text-center" data-testid="magic-link-verifying">
          <div className="space-y-1.5">
            <h1 className="text-2xl font-semibold tracking-tight">
              {t("auth:magicLink.verifying")}
            </h1>
            <p className="text-sm text-muted-foreground">{t("auth:magicLink.verifyingBody")}</p>
          </div>
        </div>
      ) : (
        <div className="space-y-6 text-center" data-testid="magic-link-error">
          <div className="flex justify-center">
            <div className="flex size-16 items-center justify-center rounded-full bg-amber-500/10">
              <Clock className="size-8 text-amber-500" aria-hidden="true" />
            </div>
          </div>
          <div className="space-y-1.5">
            <h1 className="text-2xl font-semibold tracking-tight">
              {t("auth:magicLink.expiredTitle")}
            </h1>
            <p className="text-sm text-muted-foreground">{t("auth:magicLink.expiredBody")}</p>
          </div>
          <Button asChild className="w-full">
            <Link to="/login">{t("auth:magicLink.requestNewLink")}</Link>
          </Button>
          <Link
            to="/login"
            className="text-sm text-muted-foreground hover:text-foreground transition-colors underline underline-offset-4"
          >
            {t("auth:magicLink.backToSignIn")}
          </Link>
        </div>
      )}
    </AuthLayout>
  )
}
