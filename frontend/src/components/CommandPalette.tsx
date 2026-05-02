import { useEffect, useState } from "react"
import {
  FolderOpen,
  HardDriveDownload,
  LayoutDashboard,
  MapPin,
  Package,
  Search,
  Settings,
  ShieldCheck,
  SlidersHorizontal,
  Tag,
  User,
  Users,
} from "lucide-react"
import { useNavigate, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"

import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command"
import { useNavLabel } from "@/lib/nav-labels"

interface PaletteEntry {
  // Translation key (under common:nav.*). The palette label is the same
  // string as the sidebar.
  labelKey: string
  // Resolves to a path; null hides the entry (used for group-scoped routes
  // when the user is on a non-group route and there's no slug to plug in).
  to: (groupSlug: string | null) => string | null
  icon: typeof Search
}

const NAVIGATION: PaletteEntry[] = [
  {
    labelKey: "common:nav.dashboard",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}` : null),
    icon: LayoutDashboard,
  },
  {
    labelKey: "common:nav.search",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/search` : null),
    icon: Search,
  },
  {
    labelKey: "common:nav.locations",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/locations` : null),
    icon: MapPin,
  },
  {
    labelKey: "common:nav.items",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/commodities` : null),
    icon: Package,
  },
  {
    labelKey: "common:nav.warranties",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/warranties` : null),
    icon: ShieldCheck,
  },
  {
    labelKey: "common:nav.tags",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/tags` : null),
    icon: Tag,
  },
  {
    labelKey: "common:nav.files",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/files` : null),
    icon: FolderOpen,
  },
  {
    labelKey: "common:nav.members",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/members` : null),
    icon: Users,
  },
  {
    labelKey: "common:nav.backup",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/backup` : null),
    icon: HardDriveDownload,
  },
  {
    labelKey: "common:nav.system",
    to: (slug) => (slug ? `/g/${encodeURIComponent(slug)}/system` : null),
    icon: Settings,
  },
  { labelKey: "common:nav.profile", to: () => "/profile", icon: User },
  { labelKey: "common:nav.preferences", to: () => "/settings", icon: SlidersHorizontal },
]

// Translation keys above are full namespace-qualified paths (e.g.
// "common:nav.dashboard") so we can pass them straight to t() as a
// variable and avoid template-literal extraction noise from
// i18next-cli (see the matching note in AppSidebar.tsx).

interface PaletteRowProps {
  entry: PaletteEntry
  target: string | null
  onSelect: (path: string | null) => void
}

// PaletteRow is broken out so `useNavLabel` (a hook) can be called once
// per entry without violating the rules-of-hooks ordering (mapping inside
// the parent component would call the hook in a loop, which is fine in
// React 19 but read-only-disallowed by the rule).
function PaletteRow({ entry, target, onSelect }: PaletteRowProps) {
  const label = useNavLabel(entry.labelKey)
  const Icon = entry.icon
  return (
    <CommandItem value={label} disabled={!target} onSelect={() => onSelect(target)}>
      <Icon className="size-4" />
      <span>{label}</span>
    </CommandItem>
  )
}

// CommandPalette is the Cmd/Ctrl+K quick-nav. It opens whenever the
// platform-appropriate shortcut is pressed AND the user isn't typing in an
// input (cmdk handles the focus check via its `onKeyDown` integration; we
// just bind the global shortcut). Typing a query surfaces a "Search 'X'…"
// entry that hands the query off to /g/{slug}/search?q=… (the global
// search page from #1416).
export function CommandPalette() {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const navigate = useNavigate()
  const params = useParams<{ groupSlug?: string }>()
  const { t } = useTranslation()
  const groupSlug = params.groupSlug ?? null

  // Reset the query when the palette closes so the next open starts
  // empty — otherwise the user's last keyword sits there with no
  // visible cursor and the navigation list is filtered against it.
  useEffect(() => {
    if (!open) setQuery("")
  }, [open])

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      const isShortcut =
        (event.key === "k" || event.key === "K") && (event.metaKey || event.ctrlKey)
      if (!isShortcut) return
      event.preventDefault()
      setOpen((prev) => !prev)
    }
    window.addEventListener("keydown", onKeyDown)
    return () => window.removeEventListener("keydown", onKeyDown)
  }, [])

  function go(path: string | null) {
    if (!path) return
    setOpen(false)
    navigate(path)
  }

  function runSearch() {
    if (!groupSlug) return
    const trimmed = query.trim()
    if (!trimmed) return
    setOpen(false)
    navigate(`/g/${encodeURIComponent(groupSlug)}/search?q=${encodeURIComponent(trimmed)}`)
  }

  const trimmedQuery = query.trim()

  return (
    <CommandDialog open={open} onOpenChange={setOpen} contentTestId="command-palette">
      <CommandInput
        placeholder={t("common:shell.commandPlaceholder")}
        value={query}
        onValueChange={setQuery}
      />
      <CommandList>
        {/* cmdk only renders <CommandEmpty> when there are zero matches AND the
            query is non-empty. The Cmd+K spec asks for an initial hint before
            the user types anything, so we render that ourselves above the
            empty-results message. */}
        {trimmedQuery.length === 0 ? (
          <p className="py-6 text-center text-sm text-muted-foreground">
            {t("common:shell.commandHint")}
          </p>
        ) : null}
        <CommandEmpty>
          {t("common:shell.commandNoResults", { query: trimmedQuery })}
        </CommandEmpty>
        {trimmedQuery && groupSlug ? (
          <>
            <CommandGroup heading={t("common:shell.commandGroupSearch")}>
              <CommandItem
                value={`__search__:${trimmedQuery}`}
                onSelect={runSearch}
                data-testid="palette-search-handoff"
              >
                <Search className="size-4" />
                <span>{t("common:shell.commandSearchHandoff", { query: trimmedQuery })}</span>
              </CommandItem>
            </CommandGroup>
            <CommandSeparator />
          </>
        ) : null}
        <CommandGroup heading={t("common:shell.commandGroupNavigation")}>
          {NAVIGATION.map((entry) => (
            <PaletteRow
              key={entry.labelKey}
              entry={entry}
              target={entry.to(groupSlug)}
              onSelect={go}
            />
          ))}
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  )
}
