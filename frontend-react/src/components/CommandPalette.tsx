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
// just bind the global shortcut). Search-result entries (items, files,
// tags) are deferred to per-feature issues — the AC explicitly calls them
// out as stubbed for now, so we render only the Navigation group.
export function CommandPalette() {
  const [open, setOpen] = useState(false)
  const navigate = useNavigate()
  const params = useParams<{ groupSlug?: string }>()
  const { t } = useTranslation()
  const groupSlug = params.groupSlug ?? null

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

  return (
    <CommandDialog open={open} onOpenChange={setOpen}>
      <CommandInput placeholder={t("common:shell.commandPlaceholder")} />
      <CommandList>
        <CommandEmpty>{t("common:shell.commandNoResults")}</CommandEmpty>
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
        <CommandSeparator />
        <CommandGroup heading={t("common:shell.commandGroupSearch")}>
          <CommandItem disabled>
            <Search className="size-4" />
            <span>{t("common:shell.commandSearchStub")}</span>
          </CommandItem>
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  )
}
