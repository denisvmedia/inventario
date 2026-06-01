import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { ArrowRight, Package, Sparkles } from "lucide-react"

import { AnonymousCommodityDialog } from "@/components/items/AnonymousCommodityDialog"
import { useFeatureFlag } from "@/features/feature-flags/hooks"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { cn } from "@/lib/utils"

// LandingPage is the public, unauthenticated "/" surface (#1988): an
// anonymous visitor can start adding their first item (snap a photo, let
// the public AI scan fill the details) BEFORE creating an account. On
// save the draft is stashed and the user is sent to log in; after auth
// the FirstItemResolver replays it into their group.
//
// No analogue exists in design-mocks/ (logged this as a deviation —
// "Why: not present in mock"). The hero borrows NoGroupPage's
// concentric-circle icon + `font-semibold tracking-tight` treatment, and
// the two-card grid uses the canonical icon-headed card-shell pattern
// from the mock's onboarding/empty-state surfaces. The page renders its
// own full-screen layout because it sits OUTSIDE the authenticated Shell.
//
// The "Add New Item" card is ALWAYS shown — adding your first item is the
// page's primary CTA (#1988) and must never disappear (the regression that
// motivated this change: with public_scan off the page degenerated to a
// browse-only dead-end). The `public_scan` feature flag only gates the AI
// photo-scan *accelerator*, not the ability to add an item: when the flag
// is off the dialog opens directly on manual entry (no scan endpoint is
// offered, so nothing 404s) and the card copy drops the "let AI fill it in"
// promise. The post-save hand-off (stash draft → login → replay) is
// identical either way. "Browse My Items" and the ghost login link both
// route to /login?redirect=/ so a returning user lands back on "/" (RootGate
// then resolves them to their dashboard).
export function LandingPage() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const publicScanEnabled = useFeatureFlag("public_scan")
  const [dialogOpen, setDialogOpen] = useState(false)
  // Snapshot of `public_scan` taken when the dialog OPENS, not read live.
  // `useFeatureFlag` returns the `false` fallback until the boot fetch
  // resolves, so a quick click can open the dialog in manual mode and then
  // see the flag flip false→true. Feeding that live value to the dialog
  // would change its `initialStep` mid-session, re-firing the form's
  // open-reset effect and clobbering in-progress input. Freezing the value
  // at open time pins the entry step for the whole session.
  const [dialogAiScan, setDialogAiScan] = useState(false)

  function openAddDialog() {
    // Coerce the `boolean | undefined` flag (undefined while the boot fetch
    // is in flight) to a concrete boolean for the snapshot.
    setDialogAiScan(publicScanEnabled === true)
    setDialogOpen(true)
  }

  function goToLogin() {
    navigate(`/login?redirect=${encodeURIComponent("/")}`)
  }

  return (
    <div className="flex min-h-svh w-full flex-col bg-background">
      <RouteTitle title={t("landing:hero.title")} />
      <header className="flex items-center gap-2 px-6 py-5 sm:px-10">
        <div className="flex size-7 items-center justify-center rounded-md bg-primary">
          <Package className="size-4 text-primary-foreground" aria-hidden="true" />
        </div>
        <span className="text-base font-semibold">{t("common:brand")}</span>
      </header>

      <main
        className="flex flex-1 flex-col items-center justify-center px-6 py-12"
        data-testid="landing-page"
      >
        <div className="w-full max-w-2xl space-y-10">
          <div className="space-y-3 text-center">
            <div className="flex justify-center">
              <div className="relative flex size-20 items-center justify-center">
                <div aria-hidden="true" className="absolute size-20 rounded-full bg-muted/60" />
                <div aria-hidden="true" className="absolute size-14 rounded-full bg-muted" />
                <Package className="relative size-8 text-muted-foreground/60" aria-hidden="true" />
              </div>
            </div>
            <h1 className="text-2xl font-semibold tracking-tight sm:text-3xl">
              {t("landing:hero.title")}
            </h1>
            <p className="mx-auto max-w-md text-sm leading-relaxed text-muted-foreground">
              {t("landing:hero.subtitle")}
            </p>
          </div>

          {/* Add + Browse are both always present, so the grid is always
              two-up on ≥sm. The Add card's icon + copy reflect whether the
              AI scan accelerator is available (public_scan): Sparkles +
              "let AI fill it in" when on, a plain Package + manual-entry
              copy when off. */}
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            <LandingCard
              title={t("landing:cards.addItem.title")}
              description={t(
                publicScanEnabled
                  ? "landing:cards.addItem.description"
                  : "landing:cards.addItem.descriptionManual"
              )}
              icon={publicScanEnabled ? Sparkles : Package}
              onClick={openAddDialog}
              testId="landing-add-item"
            />
            <LandingCard
              title={t("landing:cards.browse.title")}
              description={t("landing:cards.browse.description")}
              icon={Package}
              onClick={goToLogin}
              testId="landing-browse"
            />
          </div>

          <div className="text-center">
            <button
              type="button"
              onClick={goToLogin}
              className="text-sm text-muted-foreground underline-offset-4 transition-colors hover:text-foreground hover:underline"
              data-testid="landing-login-link"
            >
              {t("landing:loginCta")}
            </button>
          </div>
        </div>
      </main>

      <AnonymousCommodityDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        aiScanEnabled={dialogAiScan}
      />
    </div>
  )
}

interface LandingCardProps {
  title: string
  description: string
  icon: typeof Package
  onClick: () => void
  testId: string
}

// LandingCard is the icon-headed CTA card from the mock's onboarding
// pattern (icon chip + title + sub-line), rendered as a full-width
// button so the whole surface is the click target.
function LandingCard({ title, description, icon: Icon, onClick, testId }: LandingCardProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      data-testid={testId}
      className={cn(
        "group flex flex-col gap-3 rounded-xl border border-border bg-card p-5 text-left",
        "transition-colors hover:border-primary/40 hover:bg-muted/30",
        "focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
      )}
    >
      <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10">
        <Icon className="size-5 text-primary" aria-hidden="true" />
      </div>
      <div className="space-y-1">
        <p className="flex items-center gap-1.5 text-sm font-semibold">
          {title}
          <ArrowRight className="size-3.5 opacity-0 transition-opacity group-hover:opacity-100" />
        </p>
        <p className="text-xs leading-relaxed text-muted-foreground">{description}</p>
      </div>
    </button>
  )
}
