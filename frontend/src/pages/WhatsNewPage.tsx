import { useTranslation } from "react-i18next"
import { Info } from "lucide-react"
import whatsNewMeta from "virtual:whats-new/meta"
import releases from "virtual:whats-new/releases"

import { RouteTitle } from "@/components/routing/RouteTitle"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Page, PageHeader } from "@/components/ui/page"
import { useAuth } from "@/features/auth/AuthContext"
import { useMarkWhatsNewSeen } from "@/features/whats-new/hooks"
import { ReleaseNotes } from "@/features/whats-new/ReleaseNotes"
import type { ChangelogRelease, WhatsNewMeta } from "@/lib/changelog"

// /whats-new (#1386): the releases this build contains, newest first, as
// CHANGELOG.md has them (see vite-plugins/whats-new.ts). Opening the page
// marks the newest one as seen, which clears the unread dot.
export function WhatsNewPage() {
  useMarkWhatsNewSeen()
  const { user } = useAuth()
  return (
    <WhatsNewView
      meta={whatsNewMeta}
      releases={releases}
      showOperators={user?.is_system_admin === true}
    />
  )
}

// WhatsNewView is the page body without the data sources, so tests can
// hand it any changelog.
export function WhatsNewView({
  meta,
  releases,
  showOperators,
}: {
  meta: WhatsNewMeta
  releases: ChangelogRelease[]
  showOperators: boolean
}) {
  const { t } = useTranslation()
  return (
    <>
      <RouteTitle title={t("settings:whatsNew.title")} />
      <Page width="narrow" data-testid="whats-new-page">
        <PageHeader title={t("settings:whatsNew.title")} subtitle={t("settings:whatsNew.subtitle")} />
        {meta.build.kind === "development" ? (
          <Alert data-testid="whats-new-development-build">
            <Info aria-hidden="true" />
            <AlertTitle>{t("settings:whatsNew.developmentTitle")}</AlertTitle>
            <AlertDescription>
              {t("settings:whatsNew.developmentDescription", { version: meta.build.label })}
            </AlertDescription>
          </Alert>
        ) : null}
        {releases.length > 0 ? (
          <ReleaseNotes releases={releases} showOperators={showOperators} />
        ) : (
          <p
            className="rounded-xl border border-dashed border-border bg-muted/20 px-6 py-12 text-center text-sm text-muted-foreground"
            data-testid="whats-new-empty"
          >
            {t("settings:whatsNew.empty")}
          </p>
        )}
      </Page>
    </>
  )
}
