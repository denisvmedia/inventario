import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { ArrowRight, Building2, Plus } from "lucide-react"

import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useLogout } from "@/features/auth/hooks"
import { useCreateGroup } from "@/features/group/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { parseServerError } from "@/lib/server-error"
import { RouteTitle } from "@/components/routing/RouteTitle"

// NoGroupPage — onboarding landing for an authenticated user with zero
// groups. Lives inside the Shell layout (it sits under the protected
// subtree in the router) so the user gets the sidebar/topbar even before
// they belong to a group. Pending-invite listing is tracked separately —
// the invite-list endpoint isn't on this slice (#1413), so the design
// mock's "or accept an invite" block is intentionally not rendered yet.
//
// The inline create-group flow (#1261 contract) keeps the user on
// /no-group rather than punting them to a separate /groups/new page so
// onboarding is a single screen. On success the GroupProvider refreshes
// /api/v1/groups, the GroupRequiredRoute guard sees `hasGroups=true`,
// and we navigate to / which the router redirects to /g/<slug>.
export function NoGroupPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const logoutMutation = useLogout()
  const createMutation = useCreateGroup()
  const toast = useAppToast()
  const [formOpen, setFormOpen] = useState(false)
  const [name, setName] = useState("")
  const [serverError, setServerError] = useState<string | null>(null)

  async function handleSubmit(event: React.FormEvent) {
    event.preventDefault()
    const trimmed = name.trim()
    if (!trimmed) return
    setServerError(null)
    try {
      await createMutation.mutateAsync({ name: trimmed })
      toast.success(t("groups:create.successToast"))
      // Bounce back to "/" and let RootRedirect resolve the freshly-cached
      // group into the right /g/<slug> URL — keeps the post-onboarding
      // redirect flow aligned with the rest of the shell (and matches the
      // pre-cutover Vue contract that the e2e suite watches for).
      navigate("/")
    } catch (err) {
      setServerError(parseServerError(err, t("groups:create.errorGeneric")))
    }
  }

  return (
    <>
      <RouteTitle title={t("stubs:noGroup")} />
      <div
        className="flex flex-1 flex-col items-center justify-center py-12 px-2"
        data-testid="no-group-view"
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

            {!formOpen ? (
              <Button
                type="button"
                className="w-full gap-2"
                onClick={() => setFormOpen(true)}
                data-testid="no-group-create-button"
              >
                {t("auth:noGroup.createGroupCta")}
                <ArrowRight className="size-4" />
              </Button>
            ) : (
              <form className="space-y-3" onSubmit={handleSubmit} noValidate>
                <div className="space-y-1.5">
                  <Label htmlFor="no-group-name">{t("groups:create.nameLabel")}</Label>
                  <Input
                    id="no-group-name"
                    autoComplete="off"
                    maxLength={100}
                    disabled={createMutation.isPending}
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    placeholder={t("groups:create.namePlaceholder")}
                    data-testid="no-group-name-input"
                  />
                </div>
                {serverError ? (
                  <p className="text-xs text-destructive" data-testid="no-group-server-error">
                    {serverError}
                  </p>
                ) : null}
                <div className="flex gap-2">
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={() => {
                      setFormOpen(false)
                      setServerError(null)
                      setName("")
                    }}
                    disabled={createMutation.isPending}
                  >
                    {t("common:actions.cancel")}
                  </Button>
                  <Button
                    type="submit"
                    className="ml-auto gap-2"
                    disabled={createMutation.isPending || !name.trim()}
                    data-testid="no-group-submit"
                  >
                    {createMutation.isPending
                      ? t("groups:create.submitting")
                      : t("groups:create.submit")}
                    {!createMutation.isPending ? <ArrowRight className="size-4" /> : null}
                  </Button>
                </div>
              </form>
            )}
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
