import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Package } from "lucide-react"

import { isFeatureEnabled } from "@/lib/feature-flags"

interface AuthLayoutProps {
  children: ReactNode
}

// Two-pane wrapper for every auth screen — decorative branding panel on the
// left (lg+ only), form column on the right. The right column auto-centers
// its child and caps width at `max-w-sm` so each page can pass its full
// content tree without re-implementing the layout.
//
// Styling is taken straight from the inventario-design `AuthView` mock; the
// stats-teaser strip in the bottom-left is hidden behind a feature flag
// (#1390) until the values become real.
export function AuthLayout({ children }: AuthLayoutProps) {
  const { t } = useTranslation()
  return (
    <div className="flex min-h-svh w-full">
      <div className="hidden lg:flex lg:w-[44%] bg-primary flex-col justify-between p-10 relative overflow-hidden">
        <div
          aria-hidden="true"
          className="absolute inset-0 opacity-[0.04]"
          style={{
            backgroundImage:
              "repeating-linear-gradient(0deg,transparent,transparent 39px,currentColor 39px,currentColor 40px),repeating-linear-gradient(90deg,transparent,transparent 39px,currentColor 39px,currentColor 40px)",
          }}
        />
        <div className="relative z-10 flex items-center gap-2.5">
          <div className="flex size-8 items-center justify-center rounded-lg bg-primary-foreground/10">
            <Package className="size-4 text-primary-foreground" />
          </div>
          <span className="text-lg font-semibold text-primary-foreground">{t("common:brand")}</span>
        </div>
        <div className="relative z-10 space-y-4">
          <blockquote className="text-2xl font-semibold leading-snug text-primary-foreground">
            {t("auth:layout.tagline")}
          </blockquote>
          <div className="flex items-center gap-3">
            <div aria-hidden="true" className="size-9 rounded-full bg-primary-foreground/15" />
            <div>
              <p className="text-sm font-medium text-primary-foreground">
                {t("auth:layout.taglineAttribution")}
              </p>
              <p className="text-xs text-primary-foreground/60">
                {t("auth:layout.taglineSubtitle")}
              </p>
            </div>
          </div>
        </div>
        {isFeatureEnabled("AUTH_STATS_TEASER") ? (
          <StatsTeaserStub />
        ) : (
          <div className="relative z-10" />
        )}
      </div>
      <div className="flex flex-1 flex-col items-center justify-center bg-background px-6 py-12">
        <div className="mb-8 flex items-center gap-2 lg:hidden">
          <div className="flex size-7 items-center justify-center rounded-md bg-primary">
            <Package className="size-4 text-primary-foreground" />
          </div>
          <span className="text-base font-semibold">{t("common:brand")}</span>
        </div>
        <div className="w-full max-w-sm">{children}</div>
      </div>
    </div>
  )
}

// Static placeholder strip — real numbers land in #1390. Kept here rather
// than a separate component because nothing else renders it.
function StatsTeaserStub() {
  const { t } = useTranslation()
  const items = [
    { label: t("auth:layout.statsTeaser.itemsTracked"), value: "—" },
    { label: t("auth:layout.statsTeaser.warrantiesActive"), value: "—" },
    { label: t("auth:layout.statsTeaser.estValue"), value: "—" },
  ]
  return (
    <div className="relative z-10 flex gap-3" data-testid="auth-stats-teaser">
      {items.map((s) => (
        <div
          key={s.label}
          className="flex-1 rounded-xl bg-primary-foreground/8 border border-primary-foreground/10 p-3 backdrop-blur-sm"
        >
          <p className="text-xl font-bold text-primary-foreground">{s.value}</p>
          <p className="text-[11px] text-primary-foreground/60 mt-0.5">{s.label}</p>
        </div>
      ))}
    </div>
  )
}
