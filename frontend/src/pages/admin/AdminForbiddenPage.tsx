import { ShieldAlert } from "lucide-react"
import { Link } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { RouteTitle } from "@/components/routing/RouteTitle"

// 403-style page shown when a signed-in but non-system-admin user lands on
// an /admin/* route. Rendered in-place by the RequireSystemAdmin guard —
// the user is NOT redirected, so a hand-typed admin URL fails loudly.
//
// The design mock has no dedicated 403 surface; this replicates the
// EmptyStatesView `NotFoundView` empty-state pattern (concentric muted
// circles + glyph + heading + lede + actions). Logged as a deviation in
// devdocs/frontend/design-deviations.md.
export function AdminForbiddenPage() {
  const { t } = useTranslation("admin")
  return (
    <>
      <RouteTitle title={t("forbidden.title")} />
      <section
        aria-labelledby="admin-forbidden-title"
        data-testid="admin-forbidden"
        className="flex flex-1 flex-col items-center justify-center gap-6 py-24 px-6 text-center"
      >
        <div className="relative flex items-center justify-center size-32">
          <div className="absolute size-32 rounded-full bg-muted/60" />
          <div className="absolute size-20 rounded-full bg-muted" />
          <ShieldAlert className="relative size-10 text-muted-foreground/50" />
        </div>
        <div className="max-w-sm space-y-2">
          <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
            403
          </p>
          <h1 id="admin-forbidden-title" className="text-2xl font-bold tracking-tight">
            {t("forbidden.title")}
          </h1>
          <p className="text-sm text-muted-foreground leading-relaxed">
            {t("forbidden.description")}
          </p>
        </div>
        <Button asChild size="sm">
          <Link to="/">{t("forbidden.back")}</Link>
        </Button>
      </section>
    </>
  )
}
