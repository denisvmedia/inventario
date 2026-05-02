import { useTranslation } from "react-i18next"

import { RouteTitle } from "@/components/routing/RouteTitle"

import { SURFACES, trackerUrl, type SurfaceKey } from "./registry"

interface ComingSoonPageProps {
  // Which surface this page represents. The registry holds the icon + the
  // tracker number; i18n holds the title + description copy.
  surface: SurfaceKey
  // data-testid for route-level tests. Defaults to
  // `coming-soon-page-<surface>` so test selectors are stable without
  // relying on heading text.
  testId?: string
}

// Full-page stub for a feature whose backend isn't implemented yet — the
// page exists in the route tree (so deep links don't 404 and the sidebar /
// command palette can target it) but renders only an informational panel
// with the tracker link. Mirrors PlaceholderPage in shape, but keeps a
// distinct testId scheme + visual treatment so a permanent "no backend
// yet" surface is easy to spot in screenshots and audits vs. a generic
// placeholder waiting for its implementation issue to land.
export function ComingSoonPage({ surface, testId }: ComingSoonPageProps) {
  const { t } = useTranslation()
  const { icon: Icon, tracker } = SURFACES[surface]
  const title = t(`stubs:surfaces.${surface}.title`)
  return (
    <>
      <RouteTitle title={title} />
      <section
        data-testid={testId ?? `coming-soon-page-${surface}`}
        data-surface={surface}
        aria-labelledby={`coming-soon-${surface}-title`}
        className="mx-auto flex w-full max-w-md flex-col items-center gap-6 py-16 text-center"
      >
        <div className="relative flex size-20 items-center justify-center">
          <div aria-hidden="true" className="absolute size-20 rounded-full bg-muted/60" />
          <div aria-hidden="true" className="absolute size-14 rounded-full bg-muted" />
          <Icon aria-hidden="true" className="relative size-8 text-muted-foreground/70" />
        </div>
        <div className="space-y-2">
          <h1
            id={`coming-soon-${surface}-title`}
            className="scroll-m-20 text-2xl font-semibold tracking-tight"
          >
            {title}
          </h1>
          <p className="text-sm text-muted-foreground leading-relaxed">
            {t(`stubs:surfaces.${surface}.description`)}
          </p>
        </div>
        <a
          href={trackerUrl(surface)}
          target="_blank"
          rel="noreferrer noopener"
          className="text-xs font-medium text-muted-foreground hover:text-foreground underline underline-offset-4 transition-colors"
        >
          {t("common:trackedBy", { ref: `#${tracker}` })}
        </a>
      </section>
    </>
  )
}
