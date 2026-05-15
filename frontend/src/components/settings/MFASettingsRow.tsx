import { useState } from "react"
import { useTranslation } from "react-i18next"
import { ChevronRight } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { MFASetupDialog } from "@/components/settings/MFASetupDialog"
import { MFADisableDialog } from "@/components/settings/MFADisableDialog"
import { useMFAStatus } from "@/features/auth/hooks"

// MFASettingsRow — the Privacy & Security row that replaces the
// `twoFactor` ComingSoon stub from #1644. Renders as a single row
// so the parent `PrivacySection` can drop it into the same divided
// card as the sessions / login-history links without breaking the
// design-mock layout.
//
//   - Inactive  → opens the enrollment dialog (QR + verify)
//   - Active    → opens the disable confirmation dialog
//
// Loading / error states keep the row visible with a muted badge so
// the user can still see "we tried to load this" rather than the
// row disappearing.
export function MFASettingsRow() {
  const { t } = useTranslation()
  const status = useMFAStatus()
  const [open, setOpen] = useState<"setup" | "disable" | null>(null)

  const isActive = status.data?.enabled === true
  const isLoading = status.isPending
  const badgeLabel = isLoading
    ? t("settings:privacy.mfa.statusLoading")
    : isActive
      ? t("settings:privacy.mfa.statusActive")
      : t("settings:privacy.mfa.statusInactive")
  const badgeVariant: "default" | "secondary" | "outline" = isLoading
    ? "outline"
    : isActive
      ? "default"
      : "secondary"

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(isActive ? "disable" : "setup")}
        disabled={isLoading}
        // focus-visible ring matches the rest of the app; the keyboard
        // affordance is non-negotiable since this row gates a security
        // setting and PrivacyRow's <Link> gets the same ring for free.
        className="flex w-full items-center justify-between gap-4 p-4 text-left transition-colors hover:bg-muted/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60"
        data-testid="privacy-mfa-row"
        data-mfa-state={isLoading ? "loading" : isActive ? "active" : "inactive"}
      >
        <div className="min-w-0">
          <p className="text-sm font-medium">{t("settings:privacy.mfa.label")}</p>
          <p className="mt-0.5 text-xs text-muted-foreground leading-relaxed">
            {isActive
              ? t("settings:privacy.mfa.descriptionActive")
              : t("settings:privacy.mfa.descriptionInactive")}
          </p>
          {isActive && status.data && status.data.backupCodesRemaining < 10 ? (
            <p className="mt-1 text-xs text-amber-600" data-testid="privacy-mfa-backup-warning">
              {t("settings:privacy.mfa.backupCodesRemaining", {
                count: status.data.backupCodesRemaining,
              })}
            </p>
          ) : null}
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <Badge variant={badgeVariant} data-testid="privacy-mfa-badge">
            {badgeLabel}
          </Badge>
          <ChevronRight className="size-4 text-muted-foreground" />
        </div>
      </button>

      <MFASetupDialog
        open={open === "setup"}
        onOpenChange={(next) => setOpen(next ? "setup" : null)}
      />
      <MFADisableDialog
        open={open === "disable"}
        onOpenChange={(next) => setOpen(next ? "disable" : null)}
      />
    </>
  )
}
