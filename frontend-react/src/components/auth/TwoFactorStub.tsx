import { ShieldCheck } from "lucide-react"
import { useTranslation } from "react-i18next"

// Visible 2FA placeholder linked to #1380. The real flow (TOTP + recovery
// codes) ships there; surfacing it here keeps the design parity with the
// inventario-design AuthView while making the "tracked elsewhere" status
// explicit instead of pretending the panel is just unused.
export function TwoFactorStub() {
  const { t } = useTranslation()
  return (
    <div
      className="rounded-lg border border-dashed border-border bg-muted/40 p-3 flex items-start gap-2.5"
      data-testid="two-factor-stub"
    >
      <ShieldCheck aria-hidden="true" className="size-4 mt-0.5 text-muted-foreground" />
      <div className="space-y-0.5">
        <p className="text-xs font-medium text-foreground">{t("auth:twoFactor.title")}</p>
        <p className="text-[11px] text-muted-foreground leading-snug">
          {t("auth:twoFactor.description", { ref: "#1380" })}
        </p>
      </div>
    </div>
  )
}
