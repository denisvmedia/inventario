import type { ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { EyeOff } from "lucide-react"

import { RouteTitle } from "@/components/routing/RouteTitle"
import { Button } from "@/components/ui/button"
import { Page } from "@/components/ui/page"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { sectionLabel } from "@/features/group/section-labels"
import { useSectionVisible, type GroupSection } from "@/features/group/sections"

interface SectionRouteProps {
  section: GroupSection
  children: ReactNode
}

// SectionRoute renders its page while the active group shows the section,
// and a notice in its place while the group hides it (#1607). The notice
// keeps the URL, so a bookmark or a link in an email explains itself
// instead of landing somewhere else, and it links to the group settings
// where an admin shows the section again.
export function SectionRoute({ section, children }: SectionRouteProps) {
  const visible = useSectionVisible(section)
  if (visible) return children
  return <HiddenSectionNotice section={section} />
}

function HiddenSectionNotice({ section }: { section: GroupSection }) {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const name = sectionLabel(t, section)
  const settingsHref = currentGroup?.id
    ? `/groups/${encodeURIComponent(currentGroup.id)}/settings?section=sections`
    : null
  const dashboardHref = currentGroup?.slug ? `/g/${encodeURIComponent(currentGroup.slug)}` : "/"

  return (
    <>
      <RouteTitle title={name} />
      <Page width="wide" data-testid="hidden-section-notice" data-section={section}>
        <div className="flex flex-1 flex-col items-center justify-center gap-6 py-24 px-6 text-center">
          <div className="relative flex size-32 items-center justify-center">
            <div className="absolute size-32 rounded-full bg-muted/60" />
            <div className="absolute size-20 rounded-full bg-muted" />
            <EyeOff className="relative size-10 text-muted-foreground/50" aria-hidden="true" />
          </div>
          <div className="max-w-sm space-y-2">
            <h1 className="text-lg font-semibold tracking-tight">
              {t("groups:hiddenSection.title", { section: name })}
            </h1>
            <p className="text-sm text-muted-foreground leading-relaxed">
              {t("groups:hiddenSection.description")}
            </p>
          </div>
          <div className="flex items-center gap-2">
            {settingsHref ? (
              <Button asChild size="sm">
                <Link to={settingsHref} data-testid="hidden-section-settings">
                  {t("groups:hiddenSection.openSettings")}
                </Link>
              </Button>
            ) : null}
            <Button asChild size="sm" variant="outline">
              <Link to={dashboardHref}>{t("groups:hiddenSection.goToDashboard")}</Link>
            </Button>
          </div>
        </div>
      </Page>
    </>
  )
}
