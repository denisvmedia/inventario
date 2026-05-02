import { Upload } from "lucide-react"

export interface DropOverlayProps {
  label: string
  hint?: string
}

// Visual cue rendered above the entity-detail page while the user
// drags files over it (#1448 quick-attach). pointer-events-none so the
// overlay never intercepts the drop event itself — the browser
// dispatches drop to the topmost element that handled dragover, which
// is the wrapper around this overlay, not the overlay.
export function DropOverlay({ label, hint }: DropOverlayProps) {
  return (
    <div
      role="presentation"
      data-testid="entity-drop-overlay"
      className="pointer-events-none absolute inset-0 z-30 flex items-center justify-center rounded-md border-2 border-dashed border-primary bg-primary/10 backdrop-blur-sm"
    >
      <div className="flex flex-col items-center gap-2 text-primary">
        <Upload className="size-8" aria-hidden="true" />
        <p className="text-sm font-medium">{label}</p>
        {hint ? <p className="text-xs text-primary/80">{hint}</p> : null}
      </div>
    </div>
  )
}
