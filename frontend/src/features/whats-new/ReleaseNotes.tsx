import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { OPERATORS_SECTION, type ChangelogRelease, type ChangelogSection } from "@/lib/changelog"
import { formatDate } from "@/lib/intl"

import { ChangelogText } from "./ChangelogText"

// ReleaseNotes lists the releases of the What's new page, one card each,
// in the order given (newest first). The text is the changelog's own, in
// English; only the section headings are translated. The Operators section
// renders only when `showOperators` is set (system administrators).
export function ReleaseNotes({
  releases,
  showOperators,
}: {
  releases: ChangelogRelease[]
  showOperators: boolean
}) {
  const { t } = useTranslation()
  return (
    <div className="space-y-6" data-testid="whats-new-releases">
      {releases.map((release) => {
        const sections = release.sections.filter(
          (s) => showOperators || s.title !== OPERATORS_SECTION
        )
        const headingId = `release-${release.version}`
        return (
          <Card
            key={release.version}
            role="region"
            aria-labelledby={headingId}
            data-testid={`whats-new-release-${release.version}`}
            className="gap-4"
          >
            <CardHeader>
              <CardTitle>
                <h2 id={headingId} className="text-lg font-semibold tracking-tight">
                  {t("settings:whatsNew.releaseTitle", { version: release.version })}
                </h2>
              </CardTitle>
              {release.date ? (
                <CardDescription>
                  <time dateTime={release.date}>{formatDate(release.date, { style: "long" })}</time>
                </CardDescription>
              ) : null}
            </CardHeader>
            <CardContent className="space-y-5 text-sm leading-relaxed">
              {release.paragraphs.map((p, i) => (
                <p key={i}>
                  <ChangelogText inlines={p} />
                </p>
              ))}
              {sections.map((section) => (
                <ReleaseSection key={section.title} section={section} label={sectionLabel(t, section.title)} />
              ))}
              {sections.length === 0 && release.paragraphs.length === 0 ? (
                <p className="text-muted-foreground" data-testid="whats-new-no-user-changes">
                  {t("settings:whatsNew.noUserChanges")}
                </p>
              ) : null}
            </CardContent>
          </Card>
        )
      })}
    </div>
  )
}

function ReleaseSection({ section, label }: { section: ChangelogSection; label: string }) {
  const { t } = useTranslation()
  const isOperators = section.title === OPERATORS_SECTION
  return (
    <section data-testid={`whats-new-section-${section.title.toLowerCase()}`} className="space-y-2">
      <h3 className="text-sm font-semibold">{label}</h3>
      {isOperators ? (
        <p className="text-xs text-muted-foreground">{t("settings:whatsNew.operatorsNote")}</p>
      ) : null}
      {section.paragraphs.map((p, i) => (
        <p key={i}>
          <ChangelogText inlines={p} />
        </p>
      ))}
      {section.entries.length > 0 ? (
        <ul className="list-disc space-y-2 pl-5 marker:text-muted-foreground">
          {section.entries.map((entry, i) => (
            <li key={i}>
              <ChangelogText inlines={entry} />
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  )
}

// The Keep a Changelog headings and Operators are translated as page
// chrome; any other heading shows as written.
function sectionLabel(t: TFunction, title: string): string {
  switch (title) {
    case "Added":
      return t("settings:whatsNew.sections.added")
    case "Changed":
      return t("settings:whatsNew.sections.changed")
    case "Deprecated":
      return t("settings:whatsNew.sections.deprecated")
    case "Removed":
      return t("settings:whatsNew.sections.removed")
    case "Fixed":
      return t("settings:whatsNew.sections.fixed")
    case "Security":
      return t("settings:whatsNew.sections.security")
    case OPERATORS_SECTION:
      return t("settings:whatsNew.sections.operators")
    default:
      return title
  }
}
