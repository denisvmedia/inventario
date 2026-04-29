import { useTranslation } from "react-i18next"

import enStubs from "@/i18n/locales/en/stubs.json"
import { RouteTitle } from "@/components/routing/RouteTitle"

// StubKey is the union of every key in en/stubs.json — derived from the
// catalog itself so router.tsx can't pass a key that doesn't exist. The
// dynamic `t(`stubs:${titleKey}`)` lookup below cannot be checked by the
// extractor (the AST sees a template literal, not a string literal), so the
// TS union is the complementary safety net: `npm run typecheck` fails the
// moment a route names a missing stub.
export type StubKey = keyof typeof enStubs

interface PlaceholderPageProps {
  // Translation key resolved against the `stubs` namespace. Narrowed to
  // `StubKey` so a typo or removed entry surfaces at compile time.
  titleKey: StubKey
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
