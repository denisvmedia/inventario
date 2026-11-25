import type { DragEvent, KeyboardEvent, ReactNode } from "react"

// PortalEventBoundary wraps a dialog that a form field opens over its form.
// The dialog renders in a portal, outside the form in the page, but React
// still bubbles its events through the components around the field. A submit
// would reach the item form and save the item, Enter would reach the
// step-by-step dialog and move it to the next step, and a file dropped on the
// dialog would reach the item page, which takes it as an attachment.
export function PortalEventBoundary({ children }: { children: ReactNode }) {
  return (
    <div
      role="presentation"
      className="contents"
      onSubmit={stopEvent}
      onKeyDown={stopEnter}
      onDragEnter={refuseFileDrag}
      onDragOver={refuseFileDrag}
      onDragLeave={refuseFileDrag}
      onDrop={refuseFileDrag}
    >
      {children}
    </div>
  )
}

function stopEvent(event: { stopPropagation: () => void }) {
  event.stopPropagation()
}

function stopEnter(event: KeyboardEvent) {
  if (event.key === "Enter") event.stopPropagation()
}

// The default is prevented too, or the browser would open the dropped file
// in place of the page and drop the form with it.
function refuseFileDrag(event: DragEvent) {
  if (!Array.from(event.dataTransfer?.types ?? []).includes("Files")) return
  event.stopPropagation()
  event.preventDefault()
  event.dataTransfer.dropEffect = "none"
}
