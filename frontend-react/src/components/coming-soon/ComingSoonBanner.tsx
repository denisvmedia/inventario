import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"

import { SURFACES, trackerUrl, type SurfaceKey } from "./registry"

interface ComingSoonBannerProps {
  // Which stub surface to render. Each surface in `SURFACES` knows its
  // icon and tracker; the i18n catalog provides the title + description.
  surface: SurfaceKey
  // Extra classes — useful when the host page wants to constrain width
  // or change the inline spacing.
  className?: string
  // data-testid override. Defaults to `coming-soon-banner-<surface>` so
  // tests can target a specific stub without inspecting JSX text.
  testId?: string
}

// Small inline panel for a stubbed feature embedded inside a real page —
// e.g. the 2FA card on /login, "Connected accounts" on /profile, the
// notification-preferences rows on /settings. Renders muted card chrome,
// a lucide icon, the title + short description, and a tracker link to
// the GitHub issue that owns the real implementation.
//
// No buttons / no inputs by design (per #1417 acceptance criteria —
// "no controls that look enabled"). The tracker link is the single
// affordance and only opens the issue page.
export function ComingSoonBanner({ surface, className, testId }: ComingSoonBannerProps) {
  const { t } = useTranslation()
  const { icon: Icon, tracker } = SURFACES[surface]
  return (
    <div
      className={cn(
        "rounded-lg border border-dashed border-border bg-muted/40 p-3 flex items-start gap-2.5",
        className
      )}
      data-testid={testId ?? `coming-soon-banner-${surface}`}
      data-surface={surface}
    >
      <Icon aria-hidden="true" className="size-4 mt-0.5 text-muted-foreground" />
      <div className="space-y-0.5 min-w-0">
        <p className="text-xs font-medium text-foreground">
          {t(`stubs:surfaces.${surface}.title`)}
        </p>
        <p className="text-[11px] text-muted-foreground leading-snug">
          {t(`stubs:surfaces.${surface}.description`)}
        </p>
        <a
          href={trackerUrl(surface)}
          target="_blank"
          rel="noreferrer noopener"
          className="text-[11px] font-medium text-muted-foreground hover:text-foreground underline underline-offset-4 transition-colors"
        >
          {t("common:trackedBy", { ref: `#${tracker}` })}
        </a>
      </div>
    </div>
  )
}
