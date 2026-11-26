import { useRef, useState, type DragEvent } from "react"

function carriesFiles(e: DragEvent): boolean {
  return Array.from(e.dataTransfer?.types ?? []).includes("Files")
}

export interface FileDropTarget {
  // True while files are dragged over the target.
  dragging: boolean
  handlers: {
    onDragEnter: (e: DragEvent) => void
    onDragOver: (e: DragEvent) => void
    onDragLeave: (e: DragEvent) => void
    onDrop: (e: DragEvent) => void
  }
}

// useFileDrop makes an element a drop target for files from the desktop.
// Drags of anything else (text, links, rows) pass through untouched. The
// enter/leave counter keeps `dragging` steady while the pointer crosses the
// element's children.
export function useFileDrop(enabled: boolean, onFiles: (files: File[]) => void): FileDropTarget {
  const [dragging, setDragging] = useState(false)
  const depth = useRef(0)

  function reset() {
    depth.current = 0
    setDragging(false)
  }

  return {
    dragging: enabled && dragging,
    handlers: {
      onDragEnter: (e) => {
        if (!enabled || !carriesFiles(e)) return
        e.preventDefault()
        depth.current += 1
        setDragging(true)
      },
      onDragOver: (e) => {
        if (!enabled || !carriesFiles(e)) return
        e.preventDefault()
        e.dataTransfer.dropEffect = "copy"
      },
      onDragLeave: (e) => {
        if (!enabled || !carriesFiles(e)) return
        depth.current = Math.max(0, depth.current - 1)
        if (depth.current === 0) setDragging(false)
      },
      onDrop: (e) => {
        if (!enabled || !carriesFiles(e)) return
        e.preventDefault()
        reset()
        const files = Array.from(e.dataTransfer.files ?? [])
        if (files.length > 0) onFiles(files)
      },
    },
  }
}
