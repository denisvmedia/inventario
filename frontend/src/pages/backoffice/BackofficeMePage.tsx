import { useTranslation } from "react-i18next"
import { LogOut, ShieldCheck } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Page, PageHeader } from "@/components/ui/page"
import { useBackofficeAuth } from "@/features/backoffice/auth/context"
import { useBackofficeLogout } from "@/features/backoffice/auth/hooks"
import { hardRedirect } from "@/lib/navigation"
import { RouteTitle } from "@/components/routing/RouteTitle"

// BackofficeMePage is the operator profile card at /backoffice/me. Small,
// read-only — surfaces who the back-office plane is signed in as
// (email, role, MFA-enforced flag, last login) and offers a sign-out
// action. The full "users" area in admin chrome already covers tenant
// users; this page is intentionally just an operator self-card so the
// operator has a definitive place to confirm + end their back-office
// session without leaving the platform UI.
export function BackofficeMePage() {
  const { t } = useTranslation("backoffice")
  const { user } = useBackofficeAuth()
  const logoutMutation = useBackofficeLogout()

  async function handleLogout() {
    try {
      await logoutMutation.mutateAsync()
    } finally {
      // A hard redirect tears every cached query down so a subsequent
      // tenant session (if any) starts from a clean slate. Soft
      // navigation would leave the back-office query observers
      // hanging on a now-undefined token.
      hardRedirect("/backoffice/login")
    }
  }

  return (
    <Page width="narrow">
      <RouteTitle title={t("me.routeTitle")} />
      <PageHeader
        size="detail"
        title={t("me.title")}
        subtitle={t("me.subtitle")}
        icon={<ShieldCheck className="size-5 text-muted-foreground" aria-hidden="true" />}
      />

      <Card data-testid="backoffice-me-card">
        <CardHeader>
          <CardTitle>{user?.name ?? user?.email ?? t("me.unknownOperator")}</CardTitle>
        </CardHeader>
        <CardContent className="space-y-3 text-sm">
          <Row label={t("me.fields.email")} value={user?.email} />
          <Row label={t("me.fields.role")} value={user?.role} />
          <Row
            label={t("me.fields.mfaEnforced")}
            value={user?.mfa_enforced ? t("me.values.yes") : t("me.values.no")}
          />
          <Row label={t("me.fields.lastLogin")} value={formatTimestamp(user?.last_login_at)} />
        </CardContent>
      </Card>

      <div className="flex justify-end">
        <Button
          variant="outline"
          onClick={handleLogout}
          disabled={logoutMutation.isPending}
          data-testid="backoffice-me-logout"
        >
          <LogOut className="size-4" aria-hidden="true" />
          {logoutMutation.isPending ? t("me.signingOut") : t("me.signOut")}
        </Button>
      </div>
    </Page>
  )
}

function Row({ label, value }: { label: string; value: string | null | undefined }) {
  return (
    <div className="flex items-baseline justify-between gap-4 border-b border-border/60 pb-2 last:border-b-0 last:pb-0">
      <span className="text-muted-foreground">{label}</span>
      <span className="font-medium text-foreground">{value ?? "—"}</span>
    </div>
  )
}

function formatTimestamp(value: string | null | undefined): string | null {
  if (!value) return null
  const d = new Date(value)
  if (Number.isNaN(d.getTime())) return value
  return d.toLocaleString()
}
