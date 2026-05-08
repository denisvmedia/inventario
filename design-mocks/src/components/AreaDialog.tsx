import { useState, useEffect } from "react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
import { cn } from "@/lib/utils"
import type { Area } from "@/data/mock"

const AREA_ICONS = [
  "🍳", "🛋️", "💼", "🧺", "🪣", "🛏️", "🚿", "🎮", "📚", "🔧",
  "🪴", "🍷", "🎨", "🏋️", "🧹", "📦", "🚗", "🌿", "🔑", "⚡",
]

interface AreaDialogProps {
  open: boolean
  onClose: () => void
  locationName?: string
  /** If provided, dialog is in edit mode */
  area?: Area | null
  onSave: (data: { name: string; icon: string }) => void
}

export function AreaDialog({ open, onClose, locationName, area, onSave }: AreaDialogProps) {
  const isEdit = !!area

  const [name, setName] = useState("")
  const [icon, setIcon] = useState("🍳")

  useEffect(() => {
    if (open) {
      setName(area?.name ?? "")
      setIcon(area?.icon ?? "🍳")
    }
  }, [open, area])

  function handleSave() {
    if (!name.trim()) return
    onSave({ name: name.trim(), icon })
  }

  return (
    <Dialog open={open} onOpenChange={(v) => !v && onClose()}>
      <DialogContent className="sm:max-w-sm">
        <DialogHeader>
          <DialogTitle>{isEdit ? "Edit area" : "Add area"}</DialogTitle>
          {locationName && (
            <DialogDescription>
              {isEdit ? `Update area in` : `New area in`}{" "}
              <span className="font-medium text-foreground">{locationName}</span>
            </DialogDescription>
          )}
        </DialogHeader>

        <div className="flex flex-col gap-5 py-1">
          {/* Icon picker */}
          <div className="flex flex-col gap-2">
            <Label>Icon</Label>
            <div className="flex flex-wrap gap-1.5">
              {AREA_ICONS.map((ic) => (
                <button
                  key={ic}
                  type="button"
                  onClick={() => setIcon(ic)}
                  className={cn(
                    "flex size-9 items-center justify-center rounded-lg text-xl transition-all border",
                    icon === ic
                      ? "border-primary bg-primary/10 scale-110"
                      : "border-border bg-muted hover:border-primary/40"
                  )}
                >
                  {ic}
                </button>
              ))}
            </div>
          </div>

          {/* Name */}
          <div className="flex flex-col gap-2">
            <Label htmlFor="area-name">Name <span className="text-destructive">*</span></Label>
            <Input
              id="area-name"
              placeholder="e.g. Kitchen, Living Room, Workshop…"
              value={name}
              onChange={(e) => setName(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && handleSave()}
              autoFocus
            />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={handleSave} disabled={!name.trim()}>
            {isEdit ? "Save changes" : "Add area"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
