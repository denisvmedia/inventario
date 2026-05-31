import { useTranslation } from "react-i18next"
import { LayoutGrid, List } from "lucide-react"

import { Button } from "@/components/ui/button"
import type { FilesViewMode } from "@/features/files/useFilesViewMode"
import { cn } from "@/lib/utils"

// Shared grid/list switch used by every file-listing surface. Lifted
// verbatim from the global Files page toolbar (#1966) so the control
// looks and behaves identically wherever files are listed.
export interface FileViewToggleProps {
  value: FilesViewMode
  onChange: (mode: FilesViewMode) => void
  className?: string
  // Overridable so multiple toggles on distinct surfaces keep stable,
  // surface-scoped test ids. Emits `${testIdPrefix}-list` / `-grid`.
  testIdPrefix?: string
}

export function FileViewToggle({
  value,
  onChange,
  className,
  testIdPrefix = "files-view",
}: FileViewToggleProps) {
  const { t } = useTranslation()
  return (
    <div className={cn("flex gap-1", className)}>
      <Button
        variant={value === "list" ? "secondary" : "ghost"}
        size="icon"
        className="size-8"
        onClick={() => onChange("list")}
        aria-label={t("files:view.list", { defaultValue: "List view" })}
        aria-pressed={value === "list"}
        data-testid={`${testIdPrefix}-list`}
      >
        <List className="size-4" aria-hidden="true" />
      </Button>
      <Button
        variant={value === "grid" ? "secondary" : "ghost"}
        size="icon"
        className="size-8"
        onClick={() => onChange("grid")}
        aria-label={t("files:view.grid", { defaultValue: "Grid view" })}
        aria-pressed={value === "grid"}
        data-testid={`${testIdPrefix}-grid`}
      >
        <LayoutGrid className="size-4" aria-hidden="true" />
      </Button>
    </div>
  )
}
