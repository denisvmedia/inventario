import { useTranslation } from "react-i18next"

import { RouteTitle } from "@/components/routing/RouteTitle"

interface PlaceholderPageProps {
  // Translation key resolved against the `stubs` namespace. Keeping the
  // page agnostic to the resolved string is what lets the same component
  // render every "Coming soon" page without a per-page wrapper.
  titleKey: string
  // data-testid on the wrapper so route-level tests can assert which page
  // mounted without depending on heading text.
  testId: string
  // Issue or epic the real implementation tracks against. The user-facing
  // copy is intentionally generic ("coming soon"); this prop just feeds
  // the dev-facing footer line.
  trackedBy?: string
}

// One placeholder component for every route the router ships before its
// owning issue lands. Each page is a separate <Route> mount, but the body
// is the same "Coming soon" copy so we don't duplicate three lines of JSX
// twenty-odd times.
export function PlaceholderPage({ titleKey, testId, trackedBy }: PlaceholderPageProps) {
  const { t } = useTranslation()
  // titleKey is the leaf key under the `stubs` namespace — the dynamic
  // `t(`stubs:${titleKey}`)` here is the one place the parser cannot
  // detect a key statically, but each titleKey value is a literal in
  // router.tsx and is enumerated up-front in stubs.json.
  const title = t(`stubs:${titleKey}`)
  return (
    <>
      <RouteTitle title={title} />
      <section
        aria-labelledby={`${testId}-title`}
        data-testid={testId}
        className="flex flex-col gap-3 max-w-md w-full text-center"
      >
        <h1 id={`${testId}-title`} className="scroll-m-20 text-2xl font-semibold tracking-tight">
          {title}
        </h1>
        <p className="text-muted-foreground text-sm">{t("common:comingSoon")}</p>
        {trackedBy ? (
          <p className="text-muted-foreground/70 text-xs">
            {t("common:trackedBy", { ref: trackedBy })}
          </p>
        ) : null}
      </section>
    </>
  )
}
