import { useState } from "react"
import { ArrowRight, MapPin, X } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"

interface AssignLocationBannerProps {
  // Whether the prompt applies: the commodity has no area AND the group
  // has at least one location to file it under. False hides the banner.
  show: boolean
  // Opens the assignment surface — the edit dialog, where the
  // location/area picker lives (#1987).
  onAssign: () => void
}

// Non-blocking, dismissible notice shown on an unassigned commodity's
// detail page (#1987). After the create dialog stops asking for a
// location, this offers to file the item under one — without forcing the
// choice. In-session dismiss only (returns on the next mount); a brand-new
// item lands here straight after create, which is exactly when the nudge
// is most useful.
export function AssignLocationBanner({ show, onAssign }: AssignLocationBannerProps) {
  const [dismissed, setDismissed] = useState(false)
  const { t } = useTranslation()

  if (!show || dismissed) return null

  return (
    <div
      className="flex items-center gap-3 rounded-xl border border-border bg-primary/5 px-4 py-2.5"
      role="status"
      data-testid="assign-location-banner"
    >
      <div className="flex size-6 shrink-0 items-center justify-center rounded-full bg-primary/15">
        <MapPin className="size-3.5 text-primary" />
      </div>
      <p className="flex-1 text-sm text-foreground">
        {t("commodities:form.assignLocationBanner.title")}
      </p>
      <Button
        variant="ghost"
        size="sm"
        className="h-7 shrink-0 gap-1.5 text-xs"
        onClick={onAssign}
        data-testid="assign-location-banner-cta"
      >
        {t("commodities:form.assignLocationBanner.cta")}
        <ArrowRight className="size-3" />
      </Button>
      <button
        type="button"
        onClick={() => setDismissed(true)}
        className="shrink-0 text-muted-foreground transition-colors hover:text-foreground"
        aria-label={t("commodities:form.assignLocationBanner.dismiss")}
      >
        <X className="size-4" />
      </button>
    </div>
  )
}
