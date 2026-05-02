import { useTranslation } from "react-i18next"

import { cn } from "@/lib/utils"

interface AppLogoProps {
  className?: string
}

// Inline SVG (no asset URL fetch) of a stylized house with a checklist
// inside — copied 1:1 from the inventario-design mock. The glyph reads
// against any background colour because both fills resolve to theme tokens.
function LogoMark({ className }: { className?: string }) {
  return (
    <svg
      width="18"
      height="18"
      viewBox="0 0 18 18"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
      aria-hidden="true"
      className={className}
    >
      <path d="M9 2L1.5 8H3V16H15V8H16.5L9 2Z" className="fill-foreground" />
      <rect x="5.5" y="9.5" width="1.5" height="1.5" rx="0.75" className="fill-background" />
      <rect x="8" y="10" width="4.5" height="0.75" rx="0.375" className="fill-background" />
      <rect x="5.5" y="12" width="1.5" height="1.5" rx="0.75" className="fill-background" />
      <rect x="8" y="12.5" width="3" height="0.75" rx="0.375" className="fill-background" />
    </svg>
  )
}

export function AppLogo({ className }: AppLogoProps) {
  const { t } = useTranslation()
  return (
    <div className={cn("flex items-center gap-2 select-none", className)}>
      <LogoMark />
      <span className="text-sm font-semibold tracking-tight leading-none text-foreground">
        {t("common:brand")}
      </span>
    </div>
  )
}
