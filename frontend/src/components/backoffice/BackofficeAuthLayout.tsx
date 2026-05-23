import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { ShieldCheck } from "lucide-react"

interface BackofficeAuthLayoutProps {
  children: ReactNode
}

// Two-pane wrapper for the back-office login screen (#1785 Phase 6).
// Visually distinct from the tenant AuthLayout: a darker, slate-toned
// branding panel and a Shield icon instead of the Package icon so an
// operator can tell at a glance whether they're on the tenant `/login`
// or the back-office `/backoffice/login`. Mistaking the two has been a
// failure mode in similar SaaS admin tools — the difference here is
// deliberate.
//
// Right column matches the tenant layout's centering / max-width so the
// shared form primitives (Input, Button, PasswordInput, MFAChallenge)
// drop in without per-page adjustments.
export function BackofficeAuthLayout({ children }: BackofficeAuthLayoutProps) {
  const { t } = useTranslation("backoffice")
  return (
    <div className="flex min-h-svh w-full">
      <div className="hidden lg:flex lg:w-[44%] bg-slate-950 flex-col justify-between p-10 relative overflow-hidden">
        <div
          aria-hidden="true"
          className="absolute inset-0 opacity-[0.05]"
          style={{
            backgroundImage:
              "repeating-linear-gradient(0deg,transparent,transparent 31px,currentColor 31px,currentColor 32px),repeating-linear-gradient(90deg,transparent,transparent 31px,currentColor 31px,currentColor 32px)",
            color: "white",
          }}
        />
        <div className="relative z-10 flex items-center gap-2.5">
          <div className="flex size-8 items-center justify-center rounded-lg bg-white/10">
            <ShieldCheck className="size-4 text-white" />
          </div>
          <span className="text-lg font-semibold text-white">{t("layout.brand")}</span>
        </div>
        <div className="relative z-10 space-y-3">
          <div className="inline-flex items-center gap-1.5 rounded-full bg-white/10 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-wider text-white">
            <span className="block size-1.5 rounded-full bg-amber-300" aria-hidden="true" />
            {t("layout.restrictedBadge")}
          </div>
          <blockquote className="text-2xl font-semibold leading-snug text-white">
            {t("layout.tagline")}
          </blockquote>
          <p className="text-sm text-white/60">{t("layout.subtitle")}</p>
        </div>
        <div className="relative z-10 text-xs text-white/40">{t("layout.footer")}</div>
      </div>
      <div className="flex flex-1 flex-col items-center justify-center bg-background px-6 py-12">
        <div className="mb-8 flex items-center gap-2 lg:hidden">
          <div className="flex size-7 items-center justify-center rounded-md bg-slate-950">
            <ShieldCheck className="size-4 text-white" />
          </div>
          <span className="text-base font-semibold">{t("layout.brand")}</span>
        </div>
        <div className="w-full max-w-sm">{children}</div>
      </div>
    </div>
  )
}
