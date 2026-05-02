import { Rows3, Rows2, AlignVerticalSpaceAround } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { useDensity, type Density } from "@/hooks/useDensity"

// One icon per density level — picked so a quick glance at the toolbar
// shows the chosen tightness without reading. Rows3=comfortable (most
// breathing room), Rows2=cozy (mid), AlignVerticalSpaceAround=compact
// (tightest).
const DENSITY_ICONS: Record<Density, typeof Rows3> = {
  comfortable: Rows3,
  cozy: Rows2,
  compact: AlignVerticalSpaceAround,
}

export function DensityToggle() {
  const { density, setDensity } = useDensity()
  const { t } = useTranslation()
  const Icon = DENSITY_ICONS[density]
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant="outline" size="icon" aria-label={t("common:shell.toggleDensity")}>
          <Icon className="h-[1.2rem] w-[1.2rem]" />
          <span className="sr-only">{t("common:shell.toggleDensity")}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end">
        <DropdownMenuItem onClick={() => setDensity("comfortable")}>
          {t("common:shell.densityComfortable")}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setDensity("cozy")}>
          {t("common:shell.densityCozy")}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={() => setDensity("compact")}>
          {t("common:shell.densityCompact")}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
