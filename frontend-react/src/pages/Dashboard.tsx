import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { RouteTitle } from "@/components/routing/RouteTitle"

// DashboardPage is the bare /g/:groupSlug/ index. It renders group-scoped
// totals (counts, recent additions, expiring warranties) — the actual data
// lands in #1408. For now it's a placeholder so the rest of the routing
// foundation has a real component to mount.
//
// Translation keys carry an explicit `<namespace>:` prefix (e.g.
// `dashboard:scaffold.heading`) so i18next-parser routes them into the
// right per-namespace JSON without relying on cross-call scope tracking,
// which is fragile when a single component reads from two namespaces.
export function DashboardPage() {
  const { t } = useTranslation()
  return (
    <>
      <RouteTitle title={t("dashboard:documentTitle")} />
      <section
        aria-labelledby="dashboard-title"
        className="flex flex-col gap-4 max-w-md w-full text-center"
      >
        <h1 id="dashboard-title" className="scroll-m-20 text-3xl font-semibold tracking-tight">
          {t("dashboard:scaffold.heading")}
        </h1>
        <p className="text-muted-foreground text-sm">{t("dashboard:scaffold.description")}</p>
        <div className="flex justify-center pt-2">
          <Button variant="default" size="default">
            {t("common:actions.getStarted")}
          </Button>
        </div>
      </section>
    </>
  )
}
