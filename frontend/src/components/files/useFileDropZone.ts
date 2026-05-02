import { useCallback, useRef, useState } from "react"

export interface UseFileDropZoneOptions {
  onFiles: (files: File[]) => void
  disabled?: boolean
}

export interface FileDropZoneBindProps {
  onDragEnter: (e: React.DragEvent) => void
  onDragOver: (e: React.DragEvent) => void
  onDragLeave: (e: React.DragEvent) => void
  onDrop: (e: React.DragEvent) => void
}

export interface UseFileDropZoneResult {
  isDragging: boolean
  bindProps: FileDropZoneBindProps
}

// Page-level file-drop zone for entity-detail pages (#1448 quick-attach).
// Only triggers for actual file drags — text/url payloads are ignored
// so dragging a hyperlink onto the page doesn't open the upload modal.
//
// The dragenter/dragleave counter pattern keeps `isDragging` true
// while the cursor crosses nested children of the drop area instead
// of flickering off on every internal element boundary.
export function useFileDropZone({
  onFiles,
  disabled,
}: UseFileDropZoneOptions): UseFileDropZoneResult {
  const [isDragging, setIsDragging] = useState(false)
  const counterRef = useRef(0)

  const hasFiles = (e: React.DragEvent): boolean => {
    const types = e.dataTransfer?.types
    if (!types) return false
    for (let i = 0; i < types.length; i++) {
      if (types[i] === "Files") return true
    }
    return false
  }

  const onDragEnter = useCallback(
    (e: React.DragEvent) => {
      if (disabled) return
      if (!hasFiles(e)) return
      e.preventDefault()
      counterRef.current += 1
      setIsDragging(true)
    },
    [disabled]
  )

  const onDragOver = useCallback(
    (e: React.DragEvent) => {
      if (disabled) return
      if (!hasFiles(e)) return
      e.preventDefault()
      // Without dropEffect="copy" the browser falls back to the
      // no-drop cursor on some platforms even when the wrapper has a
      // valid drop handler.
      e.dataTransfer.dropEffect = "copy"
    },
    [disabled]
  )

  const onDragLeave = useCallback(
    (e: React.DragEvent) => {
      if (disabled) return
      if (!hasFiles(e)) return
      counterRef.current = Math.max(0, counterRef.current - 1)
      if (counterRef.current === 0) setIsDragging(false)
    },
    [disabled]
  )

  const onDrop = useCallback(
    (e: React.DragEvent) => {
      if (disabled) return
      if (!hasFiles(e)) return
      e.preventDefault()
      counterRef.current = 0
      setIsDragging(false)
      const files = Array.from(e.dataTransfer.files ?? [])
      if (files.length > 0) onFiles(files)
    },
    [disabled, onFiles]
  )

  return { isDragging, bindProps: { onDragEnter, onDragOver, onDragLeave, onDrop } }
}
