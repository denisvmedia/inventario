import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useSearchParams } from "react-router-dom"
import { ArrowRight, CheckCircle2, Clock, XCircle } from "lucide-react"

import { AuthLayout } from "@/components/auth/AuthLayout"
import { Button } from "@/components/ui/button"
import { useVerifyEmail } from "@/features/auth/hooks"
import { HttpError } from "@/lib/http"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { cn } from "@/lib/utils"

type VerifyState = "verifying" | "success" | "expired" | "invalid" | "missing"

// Heuristic: distinguish "expired" from "generic invalid" so the page can
// offer "request new link" instead of just "back to sign in". The backend
// surfaces both as 4xx with a string body, so we look at the message text.
function classifyError(err: unknown): "expired" | "invalid" {
  if (err instanceof HttpError) {
    const data = err.data
    const text = typeof data === "string" ? data : ""
    if (/expir/i.test(text)) return "expired"
  }
  return "invalid"
}

export function VerifyEmailPage() {
  const { t } = useTranslation()
  const [params] = useSearchParams()
  const token = params.get("token") ?? ""
  const verifyMutation = useVerifyEmail()
  const [state, setState] = useState<VerifyState>(token ? "verifying" : "missing")
  const [successMessage, setSuccessMessage] = useState<string | null>(null)

  useEffect(() => {
    if (!token) {
      setState("missing")
      setSuccessMessage(null)
      return
    }
    // Token swap (rare, but possible if the user hand-edits the URL or the
    // route remounts with a different ?token=) — wipe the previous outcome
    // before firing the new request so the user doesn't see stale "success"
    // copy while the new token is verifying.
    setState("verifying")
    setSuccessMessage(null)
    let cancelled = false
    verifyMutation
      .mutateAsync(token)
      .then((message) => {
        if (cancelled) return
        setSuccessMessage(message || t("auth:verify.successFallback"))
        setState("success")
      })
      .catch((err) => {
        if (cancelled) return
        setState(classifyError(err))
      })
    return () => {
      cancelled = true
    }
    // mutateAsync identity is stable per render of the mutation hook; we
    // want this effect to run on token change only.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [token])

  const config = pickConfig(state, t, successMessage)

  return (
    <AuthLayout>
      <RouteTitle title={t("stubs:verifyEmail")} />
      <div
        className={cn(
          "status-message space-y-6 text-center",
          state,
          // The Vue-era class set lumped "invalid" and "expired" both under
          // `.error`; preserve that alias for selectors that don't care
          // which 4xx variant the BE returned.
          (state === "invalid" || state === "expired") && "error"
        )}
        data-testid={`verify-${state}`}
      >
        {config.icon ? (
          <div className="flex justify-center">
            <div
              className={cn("flex size-16 items-center justify-center rounded-full", config.iconBg)}
            >
              <config.icon className={cn("size-8", config.iconColor)} aria-hidden="true" />
            </div>
          </div>
        ) : null}
        <div className="space-y-1.5">
          <h1 className="text-2xl font-semibold tracking-tight">{config.title}</h1>
          <p className="text-sm text-muted-foreground">{config.body}</p>
        </div>
        {config.action ? (
          <Button asChild className="w-full gap-2">
            <Link to={config.action.to}>
              {config.action.label}
              <ArrowRight className="size-4" />
            </Link>
          </Button>
        ) : null}
        {state !== "success" && state !== "verifying" ? (
          <p className="text-sm text-muted-foreground">
            <Link
              to="/login"
              className="font-medium text-foreground hover:underline underline-offset-4"
            >
              {t("auth:verify.backToSignIn")}
            </Link>
          </p>
        ) : null}
      </div>
    </AuthLayout>
  )
}

interface ScreenConfig {
  icon: typeof CheckCircle2 | null
  iconBg: string
  iconColor: string
  title: string
  body: string
  action: { to: string; label: string } | null
}

function pickConfig(
  state: VerifyState,
  t: (key: string) => string,
  successMessage: string | null
): ScreenConfig {
  switch (state) {
    case "verifying":
      return {
        icon: null,
        iconBg: "",
        iconColor: "",
        title: t("auth:verify.verifying"),
        body: "",
        action: null,
      }
    case "success":
      return {
        icon: CheckCircle2,
        iconBg: "bg-emerald-500/10",
        iconColor: "text-emerald-500",
        title: t("auth:verify.successTitle"),
        body: successMessage ?? t("auth:verify.successFallback"),
        action: { to: "/login", label: t("auth:verify.continue") },
      }
    case "expired":
      return {
        icon: Clock,
        iconBg: "bg-amber-500/10",
        iconColor: "text-amber-500",
        title: t("auth:verify.expiredTitle"),
        body: t("auth:verify.expiredBody"),
        action: null,
      }
    case "invalid":
      return {
        icon: XCircle,
        iconBg: "bg-destructive/10",
        iconColor: "text-destructive",
        title: t("auth:verify.invalidTitle"),
        body: t("auth:verify.invalidBody"),
        action: null,
      }
    case "missing":
      return {
        icon: XCircle,
        iconBg: "bg-amber-500/10",
        iconColor: "text-amber-500",
        title: t("auth:verify.missingTitle"),
        body: t("auth:verify.missingBody"),
        action: null,
      }
  }
}
