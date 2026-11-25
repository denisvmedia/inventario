import { useMemo } from "react"
import { Search } from "lucide-react"
import { useTranslation } from "react-i18next"

import { commandPaletteTriggerProps, useCommandPalette } from "@/components/CommandPaletteProvider"
import { Button } from "@/components/ui/button"
import { Kbd, KbdGroup } from "@/components/ui/kbd"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { detectIsMac } from "@/features/shortcuts"

// HeaderSearch is the TopBar's way into the command palette. From the sm
// breakpoint up it is the mock's search field: "Search…" with the shortcut
// next to it. Below that the TopBar has no room for a field, so it is an
// icon button, which also gives touch devices a way in.
export function HeaderSearch() {
  const { t } = useTranslation()
  const { setOpen } = useCommandPalette()
  const isMac = useMemo(() => detectIsMac(), [])
  const label = t("common:shell.searchButtonLabel")

  return (
    <>
      <Tooltip>
        <TooltipTrigger asChild>
          <button
            type="button"
            onClick={(event) => setOpen(true, event.currentTarget)}
            aria-label={label}
            aria-haspopup="dialog"
            aria-keyshortcuts={isMac ? "Meta+K" : "Control+K"}
            {...commandPaletteTriggerProps}
            className="hidden h-8 items-center gap-2 rounded-md border border-border bg-background px-3 text-sm text-muted-foreground outline-none transition-colors hover:bg-muted hover:text-foreground focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 sm:flex"
            data-testid="header-search"
          >
            <Search className="size-3.5 shrink-0" aria-hidden="true" />
            <span className="text-xs">{t("common:shell.searchButton")}</span>
            <KbdGroup className="ml-1" aria-hidden="true">
              <Kbd className="h-4 min-w-4 px-1 text-[10px]">{isMac ? "⌘" : "Ctrl"}</Kbd>
              <Kbd className="h-4 min-w-4 px-1 text-[10px]">K</Kbd>
            </KbdGroup>
          </button>
        </TooltipTrigger>
        <TooltipContent side="bottom">{t("common:shortcuts.entries.openPalette")}</TooltipContent>
      </Tooltip>
      <Button
        variant="outline"
        size="icon"
        className="sm:hidden"
        onClick={(event) => setOpen(true, event.currentTarget)}
        aria-label={label}
        aria-haspopup="dialog"
        {...commandPaletteTriggerProps}
        data-testid="header-search-compact"
      >
        <Search className="size-4" aria-hidden="true" />
      </Button>
    </>
  )
}
