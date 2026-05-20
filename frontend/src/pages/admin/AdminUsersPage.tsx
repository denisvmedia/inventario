import { useTranslation } from "react-i18next"

import { RouteTitle } from "@/components/routing/RouteTitle"

// Thin placeholder for /admin/users. A later sub-issue fills this in with
// the real cross-tenant user listing; this issue ships the routed shell so
// the AdminLayout secondary nav has a live destination.
export function AdminUsersPage() {
  const { t } = useTranslation("admin")
  return (
    <>
      <RouteTitle title={t("placeholder.users.title")} />
      <div className="flex flex-col gap-6" data-testid="admin-users-page">
        <div>
          <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">
            {t("placeholder.users.title")}
          </h1>
          <p className="mt-1 text-muted-foreground">{t("placeholder.users.subtitle")}</p>
        </div>
        <div className="rounded-xl border border-border bg-card p-6 text-sm text-muted-foreground">
          {t("placeholder.comingSoon")}
        </div>
      </div>
    </>
  )
}
