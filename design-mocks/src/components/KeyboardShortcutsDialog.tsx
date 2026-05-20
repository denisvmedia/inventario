import { Command } from "lucide-react"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog"
import { Kbd, KbdGroup } from "@/components/ui/kbd"
import { Separator } from "@/components/ui/separator"
import { ScrollArea } from "@/components/ui/scroll-area"

interface ShortcutRowProps {
  label: string
  keys: React.ReactNode
}

function ShortcutRow({ label, keys }: ShortcutRowProps) {
  return (
    <div className="flex items-center justify-between py-2">
      <span className="text-sm text-foreground">{label}</span>
      <div className="shrink-0">{keys}</div>
    </div>
  )
}

function ShortcutSection({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="space-y-0.5">
      <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground mb-1 pt-1">
        {title}
      </p>
      <div className="divide-y divide-border">{children}</div>
    </div>
  )
}

// Platform-aware modifier key label
const isMac = typeof navigator !== "undefined" && /mac/i.test(navigator.platform)
const Mod = () =>
  isMac ? (
    <Kbd>
      <Command className="size-3" />
    </Kbd>
  ) : (
    <Kbd>Ctrl</Kbd>
  )

interface KeyboardShortcutsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function KeyboardShortcutsDialog({ open, onOpenChange }: KeyboardShortcutsDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <div className="flex size-7 items-center justify-center rounded-lg bg-primary/10">
              <Command className="size-4 text-primary" />
            </div>
            Keyboard Shortcuts
          </DialogTitle>
          <DialogDescription>
            All available shortcuts across Inventario.
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className="max-h-[60vh] pr-1">
          <div className="space-y-4 pb-2">

            <ShortcutSection title="Global">
              <ShortcutRow
                label="Open command palette"
                keys={
                  <KbdGroup>
                    <Mod />
                    <Kbd>K</Kbd>
                  </KbdGroup>
                }
              />
              <ShortcutRow
                label="Keyboard shortcuts"
                keys={
                  <KbdGroup>
                    <Mod />
                    <Kbd>/</Kbd>
                  </KbdGroup>
                }
              />
              <ShortcutRow
                label="Toggle sidebar"
                keys={
                  <KbdGroup>
                    <Mod />
                    <Kbd>B</Kbd>
                  </KbdGroup>
                }
              />
              <ShortcutRow
                label="Add new item"
                keys={
                  <KbdGroup>
                    <Mod />
                    <Kbd>N</Kbd>
                  </KbdGroup>
                }
              />
              <ShortcutRow
                label="Close dialog / panel"
                keys={<Kbd>Esc</Kbd>}
              />
            </ShortcutSection>

            <Separator />

            <ShortcutSection title="Navigation">
              <ShortcutRow
                label="Go to Dashboard"
                keys={
                  <KbdGroup>
                    <Kbd>G</Kbd>
                    <Kbd>D</Kbd>
                  </KbdGroup>
                }
              />
              <ShortcutRow
                label="Go to All Items"
                keys={
                  <KbdGroup>
                    <Kbd>G</Kbd>
                    <Kbd>I</Kbd>
                  </KbdGroup>
                }
              />
              <ShortcutRow
                label="Go to Warranties"
                keys={
                  <KbdGroup>
                    <Kbd>G</Kbd>
                    <Kbd>W</Kbd>
                  </KbdGroup>
                }
              />
              <ShortcutRow
                label="Go to Files"
                keys={
                  <KbdGroup>
                    <Kbd>G</Kbd>
                    <Kbd>F</Kbd>
                  </KbdGroup>
                }
              />
              <ShortcutRow
                label="Go to Locations"
                keys={
                  <KbdGroup>
                    <Kbd>G</Kbd>
                    <Kbd>L</Kbd>
                  </KbdGroup>
                }
              />
              <ShortcutRow
                label="Go to Settings"
                keys={
                  <KbdGroup>
                    <Kbd>G</Kbd>
                    <Kbd>S</Kbd>
                  </KbdGroup>
                }
              />
            </ShortcutSection>

            <Separator />

            <ShortcutSection title="Items list">
              <ShortcutRow
                label="Search / filter"
                keys={<Kbd>/</Kbd>}
              />
              <ShortcutRow
                label="Select item"
                keys={<Kbd>Enter</Kbd>}
              />
              <ShortcutRow
                label="Move focus up"
                keys={<Kbd>↑</Kbd>}
              />
              <ShortcutRow
                label="Move focus down"
                keys={<Kbd>↓</Kbd>}
              />
            </ShortcutSection>

            <Separator />

            <ShortcutSection title="Item detail panel">
              <ShortcutRow
                label="Close panel"
                keys={<Kbd>Esc</Kbd>}
              />
              <ShortcutRow
                label="Edit item"
                keys={
                  <KbdGroup>
                    <Mod />
                    <Kbd>E</Kbd>
                  </KbdGroup>
                }
              />
              <ShortcutRow
                label="Generate insurance report"
                keys={
                  <KbdGroup>
                    <Mod />
                    <Kbd>R</Kbd>
                  </KbdGroup>
                }
              />
            </ShortcutSection>

          </div>
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}
