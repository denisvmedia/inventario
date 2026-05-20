import { useEffect, useState } from "react"
import {
  LayoutDashboard,
  Package,
  ShieldCheck,
  Tag,
  FolderOpen,
  MapPin,
  Users,
  Settings,
  HardDriveDownload,
  Zap,
  User,
  ArrowRight,
  Box,
  FileText,
  Image,
} from "lucide-react"
import {
  CommandDialog,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from "@/components/ui/command"
import { Badge } from "@/components/ui/badge"
import { MOCK_ITEMS, MOCK_LOCATIONS, MOCK_FILES } from "@/data/mock"

const NAV_ITEMS = [
  { id: "dashboard", label: "Dashboard", icon: LayoutDashboard, description: "Overview & stats" },
  { id: "items", label: "All Items", icon: Package, description: "Browse inventory" },
  { id: "warranties", label: "Warranties", icon: ShieldCheck, description: "Track warranties" },
  { id: "tags", label: "Tags", icon: Tag, description: "Manage labels" },
  { id: "files", label: "Files", icon: FolderOpen, description: "Attachments & documents" },
  { id: "locations", label: "Locations", icon: MapPin, description: "Rooms & areas" },
  { id: "members", label: "Members", icon: Users, description: "Shared access" },
  { id: "backup", label: "Backup & Restore", icon: HardDriveDownload, description: "Export your data" },
  { id: "plans", label: "Plans & Pricing", icon: Zap, description: "Upgrade your plan" },
  { id: "settings", label: "Preferences", icon: Settings, description: "App settings" },
  { id: "profile", label: "Profile", icon: User, description: "Your account" },
] as const

const FILE_ICON_MAP: Record<string, React.ElementType> = {
  "application/pdf": FileText,
}

function getFileIcon(mimeType: string): React.ElementType {
  if (mimeType.startsWith("image/")) return Image
  return FILE_ICON_MAP[mimeType] ?? FileText
}

interface CommandPaletteProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  onNavigate: (view: string) => void
  onItemClick: (id: string) => void
}

export function CommandPalette({ open, onOpenChange, onNavigate, onItemClick }: CommandPaletteProps) {
  const recentItems = MOCK_ITEMS.slice(0, 5)
  const recentFiles = (MOCK_FILES ?? []).slice(0, 4)
  const locations = MOCK_LOCATIONS.slice(0, 6)

  function select(fn: () => void) {
    onOpenChange(false)
    // slight delay so the dialog closes before state update
    setTimeout(fn, 50)
  }

  return (
    <CommandDialog open={open} onOpenChange={onOpenChange} title="Command palette" description="Navigate, search items, files, and locations">
      <CommandInput placeholder="Search or jump to…" />
      <CommandList>
        <CommandEmpty>No results found.</CommandEmpty>

        {/* Navigate */}
        <CommandGroup heading="Navigate">
          {NAV_ITEMS.map((item) => (
            <CommandItem
              key={item.id}
              value={`navigate ${item.label} ${item.description}`}
              onSelect={() => select(() => onNavigate(item.id))}
              className="gap-3"
            >
              <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted">
                <item.icon className="size-3.5 text-muted-foreground" />
              </div>
              <div className="flex-1 min-w-0">
                <span className="font-medium text-sm">{item.label}</span>
                <span className="ml-2 text-xs text-muted-foreground">{item.description}</span>
              </div>
              <ArrowRight className="size-3 text-muted-foreground/40 shrink-0" />
            </CommandItem>
          ))}
        </CommandGroup>

        <CommandSeparator />

        {/* Recent items */}
        <CommandGroup heading="Recent Items">
          {recentItems.map((item) => (
            <CommandItem
              key={item.id}
              value={`item ${item.name} ${item.brand} ${item.model} ${item.category}`}
              onSelect={() => select(() => onItemClick(item.id))}
              className="gap-3"
            >
              <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted">
                <Box className="size-3.5 text-muted-foreground" />
              </div>
              <div className="flex-1 min-w-0">
                <span className="font-medium text-sm truncate">{item.name}</span>
                {item.brand && (
                  <span className="ml-2 text-xs text-muted-foreground">{item.brand}</span>
                )}
              </div>
              <Badge variant="outline" className="text-[10px] h-4 px-1.5 shrink-0 capitalize">
                {item.category}
              </Badge>
            </CommandItem>
          ))}
        </CommandGroup>

        <CommandSeparator />

        {/* Recent files */}
        {recentFiles.length > 0 && (
          <>
            <CommandGroup heading="Recent Files">
              {recentFiles.map((file) => {
                const FileIcon = getFileIcon(file.mimeType)
                return (
                  <CommandItem
                    key={file.id}
                    value={`file ${file.name} ${file.category}`}
                    onSelect={() => select(() => onNavigate("files"))}
                    className="gap-3"
                  >
                    <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted">
                      <FileIcon className="size-3.5 text-muted-foreground" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <span className="font-medium text-sm truncate">{file.name}</span>
                      <span className="ml-2 text-xs text-muted-foreground">{file.size}</span>
                    </div>
                    <Badge variant="outline" className="text-[10px] h-4 px-1.5 shrink-0 capitalize">
                      {file.category}
                    </Badge>
                  </CommandItem>
                )
              })}
            </CommandGroup>
            <CommandSeparator />
          </>
        )}

        {/* Locations */}
        <CommandGroup heading="Locations">
          {locations.map((loc) => (
            <CommandItem
              key={loc.id}
              value={`location ${loc.name}`}
              onSelect={() => select(() => onNavigate("locations"))}
              className="gap-3"
            >
              <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted text-sm leading-none">
                {loc.icon}
              </div>
              <span className="font-medium text-sm">{loc.name}</span>
            </CommandItem>
          ))}
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  )
}

// Hook to wire the Cmd/Ctrl+K shortcut
export function useCommandPalette() {
  const [open, setOpen] = useState(false)

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "k" && (e.metaKey || e.ctrlKey)) {
        e.preventDefault()
        setOpen((o) => !o)
      }
    }
    document.addEventListener("keydown", handleKeyDown)
    return () => document.removeEventListener("keydown", handleKeyDown)
  }, [])

  return { open, setOpen }
}
