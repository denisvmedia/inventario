import { ShieldCheck } from "lucide-react"
import { NavLink, Outlet, useLocation } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { LocationsBreadcrumb } from "@/components/locations/LocationsBreadcrumb"
import { cn } from "@/lib/utils"

// The /admin/* sub-routes that get a secondary-nav pill. `end` keeps the
// match exact so /admin/tenants/:id (later sub-issues) doesn't double-
// highlight. Labels resolve against the `admin` i18n namespace.
const ADMIN_NAV = [
  { to: "/admin/tenants", labelKey: "nav.tenants" },
  { to: "/admin/users", labelKey: "nav.users" },
  { to: "/admin/groups", labelKey: "nav.groups" },
] as const

// Resolves the current section label key for the breadcrumb tail. Falls
// back to the Tenants section for an unrecognised /admin/* path.
function currentNavLabelKey(pathname: string): string {
  const match = ADMIN_NAV.find((e) => pathname === e.to || pathname.startsWith(`${e.to}/`))
  return match?.labelKey ?? ADMIN_NAV[0].labelKey
}

// AdminLayout is the chrome shared by every /admin/* page: a breadcrumb
// (Admin → <section>) and a secondary nav strip, with the active page
// rendered through <Outlet />. Mounted as a layout route in router.tsx
// under the RequireSystemAdmin guard.
//
// The design mock ships no admin layout shell — each admin view in
// design-mocks/src/views/admin/ is self-contained. This layout reuses
// the design language's tokens (overline breadcrumb, secondary-nav pills)
// and the shared LocationsBreadcrumb primitive. Logged as a deviation in
// devdocs/frontend/design-deviations.md.
export function AdminLayout() {
  const { t } = useTranslation("admin")
  const location = useLocation()
  const sectionLabelKey = currentNavLabelKey(location.pathname)

  return (
    <div className="flex flex-col gap-6">
      <div className="flex items-center gap-2">
        <div className="flex size-7 items-center justify-center rounded-lg bg-primary/10 shrink-0">
          <ShieldCheck className="size-4 text-primary" />
        </div>
        <LocationsBreadcrumb
          testId="admin-breadcrumb"
          navLabel={t("layout.breadcrumbRoot")}
          segments={[
            { label: t("layout.breadcrumbRoot"), to: "/admin/tenants" },
            { label: t(sectionLabelKey) },
          ]}
        />
      </div>

      <nav aria-label={t("nav.section")} className="flex items-center gap-1 border-b border-border">
        {ADMIN_NAV.map((entry) => (
          <NavLink
            key={entry.to}
            to={entry.to}
            end
            className={({ isActive }) =>
              cn(
                "-mb-px border-b-2 px-3 py-2 text-sm font-medium transition-colors",
                isActive
                  ? "border-primary text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              )
            }
          >
            {t(entry.labelKey)}
          </NavLink>
        ))}
      </nav>

      <Outlet />
    </div>
  )
}
