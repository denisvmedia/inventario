import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useParams } from "react-router-dom"
import { ArrowRight, Building2, Mail } from "lucide-react"

import { AuthLayout } from "@/components/auth/AuthLayout"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { useAuth } from "@/features/auth/AuthContext"
import { savePendingInvite } from "@/features/auth/inviteHandoff"
import { useAcceptInvite, useInviteInfo } from "@/features/invite/hooks"
import { parseServerError } from "@/lib/server-error"
import { RouteTitle } from "@/components/routing/RouteTitle"

// InviteAcceptPage — public page that shows invite preview info and offers
// "Accept" or "Sign in to accept" depending on auth state. The handoff
// pattern (#1285): unauthenticated users get the token stashed in
// sessionStorage before being sent to /login or /register, and those pages
// auto-accept after authentication completes.
export function InviteAcceptPage() {
  const { t } = useTranslation()
  const { token: rawToken } = useParams<{ token: string }>()
  const token = rawToken ?? ""
  const navigate = useNavigate()
  const { isAuthenticated } = useAuth()
  const { data: invite, isLoading, isError } = useInviteInfo(token)
  const acceptMutation = useAcceptInvite()
  const [acceptError, setAcceptError] = useState<string | null>(null)

  // Persist the handoff so unauth users can sign in and pick up where they
  // left off. Effect runs whenever we know the group name; if the user is
  // already authenticated we skip — they'll accept directly on this page.
  useEffect(() => {
    if (!token || isAuthenticated) return
    if (!invite) return
    savePendingInvite({ token, groupName: invite.group_name })
  }, [token, invite, isAuthenticated])

  if (isLoading) {
    return (
      <AuthLayout>
        <RouteTitle title={t("stubs:inviteAccept")} />
        <div className="text-center text-sm text-muted-foreground" data-testid="invite-loading">
          {t("auth:invite.loading")}
        </div>
      </AuthLayout>
    )
  }

  if (isError || !invite) {
    return (
      <AuthLayout>
        <RouteTitle title={t("stubs:inviteAccept")} />
        <ErrorPanel
          title={t("auth:invite.invalidTitle")}
          body={t("auth:invite.invalidBody")}
          ctaTo="/"
          ctaLabel={t("auth:invite.backHome")}
          testId="invite-invalid"
        />
      </AuthLayout>
    )
  }

  if (invite.expired) {
    return (
      <AuthLayout>
        <RouteTitle title={t("stubs:inviteAccept")} />
        <ErrorPanel
          title={t("auth:invite.expiredTitle")}
          body={t("auth:invite.expiredBody")}
          ctaTo="/"
          ctaLabel={t("auth:invite.backHome")}
          testId="invite-expired"
        />
      </AuthLayout>
    )
  }

  if (invite.used) {
    return (
      <AuthLayout>
        <RouteTitle title={t("stubs:inviteAccept")} />
        <ErrorPanel
          title={t("auth:invite.usedTitle")}
          body={t("auth:invite.usedBody")}
          ctaTo="/"
          ctaLabel={t("auth:invite.backHome")}
          testId="invite-used"
        />
      </AuthLayout>
    )
  }

  async function onAccept() {
    setAcceptError(null)
    try {
      await acceptMutation.mutateAsync(token)
      // The mutation invalidates the groups list — RootRedirect will pick
      // up the new membership when GroupProvider mounts post-navigation.
      navigate("/", { replace: true })
    } catch (err) {
      setAcceptError(parseServerError(err, t("auth:invite.errorGeneric")))
    }
  }

  const inviteRedirect = `/invite/${encodeURIComponent(token)}`

  return (
    <AuthLayout>
      <RouteTitle title={t("stubs:inviteAccept")} />
      <div className="space-y-6" data-testid="invite-page">
        <header className="space-y-1.5">
          <h1 className="text-2xl font-semibold tracking-tight">{t("auth:invite.youreInvited")}</h1>
          <p className="text-sm text-muted-foreground">{t("auth:invite.intro")}</p>
        </header>

        <div className="rounded-xl border border-border bg-card p-4 space-y-3">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 shrink-0">
              {invite.group_icon ? (
                <span aria-hidden="true" className="text-lg">
                  {invite.group_icon}
                </span>
              ) : (
                <Building2 className="size-5 text-primary" aria-hidden="true" />
              )}
            </div>
            <div className="min-w-0">
              <p className="font-semibold text-sm">{invite.group_name ?? "—"}</p>
            </div>
          </div>
          <Separator />
          <div className="flex items-center justify-between">
            <p className="text-xs text-muted-foreground">{t("auth:invite.yourRole")}</p>
            <Badge variant="secondary" className="text-xs">
              {t("auth:invite.memberRole")}
            </Badge>
          </div>
        </div>

        {acceptError ? (
          <Alert variant="destructive" data-testid="server-error">
            <AlertDescription>{acceptError}</AlertDescription>
          </Alert>
        ) : null}

        {isAuthenticated ? (
          <div className="flex flex-col gap-2">
            <Button
              className="w-full gap-2"
              disabled={acceptMutation.isPending}
              onClick={onAccept}
              data-testid="invite-accept-btn"
            >
              {acceptMutation.isPending ? t("auth:invite.accepting") : t("auth:invite.accept")}
              {!acceptMutation.isPending ? <ArrowRight className="size-4" /> : null}
            </Button>
            <Button
              variant="outline"
              className="w-full"
              onClick={() => navigate("/", { replace: true })}
              disabled={acceptMutation.isPending}
              data-testid="invite-decline-btn"
            >
              {t("auth:invite.decline")}
            </Button>
          </div>
        ) : (
          <div className="flex flex-col gap-2">
            <Button asChild className="w-full gap-2">
              <Link
                to={`/login?redirect=${encodeURIComponent(inviteRedirect)}`}
                data-testid="invite-login-link"
              >
                <Mail aria-hidden="true" className="size-4" />
                {t("auth:invite.signInToAccept")}
              </Link>
            </Button>
            <Button asChild variant="outline" className="w-full">
              <Link
                to={`/register?redirect=${encodeURIComponent(inviteRedirect)}`}
                data-testid="invite-register-link"
              >
                {t("auth:invite.registerToAccept")}
              </Link>
            </Button>
            <p className="text-center text-xs text-muted-foreground">
              {t("auth:invite.alreadyHaveAccount")}{" "}
              <Link
                to={`/login?redirect=${encodeURIComponent(inviteRedirect)}`}
                className="font-medium text-foreground hover:underline underline-offset-4"
              >
                {t("auth:invite.signInToAcceptLink")}
              </Link>
            </p>
          </div>
        )}
      </div>
    </AuthLayout>
  )
}

function ErrorPanel({
  title,
  body,
  ctaTo,
  ctaLabel,
  testId,
}: {
  title: string
  body: string
  ctaTo: string
  ctaLabel: string
  testId: string
}) {
  return (
    <div className="space-y-6 text-center" data-testid={testId}>
      <div className="space-y-1.5">
        <h1 className="text-2xl font-semibold tracking-tight">{title}</h1>
        <p className="text-sm text-muted-foreground">{body}</p>
      </div>
      <Button asChild className="w-full">
        <Link to={ctaTo}>{ctaLabel}</Link>
      </Button>
    </div>
  )
}
