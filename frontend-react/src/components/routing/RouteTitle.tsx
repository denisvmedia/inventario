import { useEffect } from "react"
import { useTranslation } from "react-i18next"

interface RouteTitleProps {
  // The translated page name (caller resolves via useTranslation()). Keeping
  // this as a plain string rather than a key+ns lets pages compute the
  // string once and reuse it for both the heading and the title.
  title: string
  // Override the default brand suffix. Empty string omits the separator
  // entirely (rare — used by full-screen takeover pages that want just a
  // verb in the tab).
  suffix?: string
}

// RouteTitle keeps document.title in sync with the rendered page. Place it
// inside each <Route element>. The previous title isn't restored on unmount
// because the next route mounts its own RouteTitle in the same render pass —
// trying to restore would race the new page's update.
//
// The full title pattern is owned by the i18n catalog
// (`common:documentTitle`, e.g. "{{title}} · {{brand}}") so locale-specific
// separator/quoting is a single-string change rather than a code change.
export function RouteTitle({ title, suffix }: RouteTitleProps) {
  const { t } = useTranslation()
  const brand = suffix ?? t("common:brand")
  useEffect(() => {
    document.title = brand ? t("common:documentTitle", { title, brand }) : title
  }, [title, brand, t])
  return null
}
