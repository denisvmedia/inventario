import { RouteTitle } from "@/components/routing/RouteTitle"

interface PlaceholderPageProps {
  // Title shown both as the <h1> and (via RouteTitle) in document.title.
  title: string
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
export function PlaceholderPage({ title, testId, trackedBy }: PlaceholderPageProps) {
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
        <p className="text-muted-foreground text-sm">Coming soon.</p>
        {trackedBy ? (
          <p className="text-muted-foreground/70 text-xs">Tracked by {trackedBy}.</p>
        ) : null}
      </section>
    </>
  )
}
