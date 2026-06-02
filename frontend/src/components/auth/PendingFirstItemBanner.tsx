import { useTranslation } from "react-i18next"
import { PackageCheck } from "lucide-react"

// Reassurance callout shown on the Login / Register pages when an anonymous
// visitor drafted their first item before signing up (#1988). The first cut
// reused the neutral one-line <Alert>, which read as just another muted
// notice and was easy to skip past. This mirrors the design mock's
// auth-screen callout card (AuthView "You're invited!" — an icon chip in a
// rounded `bg-primary/10` square + a bold title + a supporting line) and adds
// a primary tint + border so it reads as its own panel against the form,
// not a throwaway line. It only reassures; the draft replay still happens at
// /welcome via FirstItemResolver.
export function PendingFirstItemBanner() {
  const { t } = useTranslation()
  return (
    <div
      className="flex items-start gap-3 rounded-xl border border-primary/20 bg-primary/5 p-4"
      data-testid="pending-first-item-banner"
    >
      <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-primary/10">
        <PackageCheck className="size-5 text-primary" aria-hidden="true" />
      </div>
      <div className="space-y-0.5">
        <p className="text-sm font-semibold">{t("auth:firstItem.title")}</p>
        <p className="text-sm leading-relaxed text-muted-foreground">{t("auth:firstItem.body")}</p>
      </div>
    </div>
  )
}
