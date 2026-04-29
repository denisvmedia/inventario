import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { ArrowRight, Building2, Plus } from "lucide-react"

import { Button } from "@/components/ui/button"
import { useLogout } from "@/features/auth/hooks"
import { RouteTitle } from "@/components/routing/RouteTitle"

// NoGroupPage — onboarding landing for an authenticated user with zero
// groups. Lives inside the Shell layout (it sits under the protected
// subtree in the router) so the user gets the sidebar/topbar even before
// they belong to a group. Pending-invite listing is tracked separately —
// the invite-list endpoint isn't on this slice (#1413), so the design
// mock's "or accept an invite" block is intentionally not rendered yet.
export function NoGroupPage() {
  const { t } = useTranslation()
  const logoutMutation = useLogout()

  return (
    <>
      <RouteTitle title={t("stubs:noGroup")} />
      <div
        className="flex flex-1 flex-col items-center justify-center py-12 px-2"
        data-testid="no-group-page"
      >
        <div className="w-full max-w-md space-y-8">
          <div className="text-center space-y-3">
            <div className="flex justify-center">
              <div className="relative flex items-center justify-center size-20">
                <div aria-hidden="true" className="absolute size-20 rounded-full bg-muted/60" />
                <div aria-hidden="true" className="absolute size-14 rounded-full bg-muted" />
                <Building2
                  className="relative size-8 text-muted-foreground/60"
                  aria-hidden="true"
                />
              </div>
            </div>
            <h1 className="text-2xl font-semibold tracking-tight">{t("auth:noGroup.title")}</h1>
            <p className="text-sm text-muted-foreground leading-relaxed">
              {t("auth:noGroup.description")}
            </p>
          </div>

          <div className="rounded-xl border border-border bg-card p-5 space-y-3">
            <div className="flex items-center gap-3">
              <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 shrink-0">
                <Plus className="size-5 text-primary" aria-hidden="true" />
              </div>
              <div>
                <p className="font-semibold text-sm">{t("auth:noGroup.createGroup")}</p>
                <p className="text-xs text-muted-foreground">
                  {t("auth:noGroup.createGroupDescription")}
                </p>
              </div>
            </div>
            <Button asChild className="w-full gap-2">
              <Link to="/groups/new" data-testid="create-group-cta">
                {t("auth:noGroup.createGroupCta")}
                <ArrowRight className="size-4" />
              </Link>
            </Button>
          </div>

          <div className="text-center">
            <Button
              variant="ghost"
              size="sm"
              type="button"
              disabled={logoutMutation.isPending}
              onClick={() => logoutMutation.mutate()}
              data-testid="no-group-signout"
            >
              {t("auth:noGroup.signOut")}
            </Button>
          </div>
        </div>
      </div>
    </>
  )
}
