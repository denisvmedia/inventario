import { useState } from "react"

// Grid-or-list presentation mode shared by every file-listing surface
// (the global Files page, the location/area EntityFilesPanel, and the
// commodity detail Files tab). Centralising it here keeps the three
// surfaces rendering files the same way (#1966).
export type FilesViewMode = "list" | "grid"

function readStored(storageKey: string, fallback: FilesViewMode): FilesViewMode {
  if (typeof window === "undefined") return fallback
  const raw = window.localStorage.getItem(storageKey)
  return raw === "grid" || raw === "list" ? raw : fallback
}

// localStorage-backed view-mode state. Router-free on purpose so it's
// usable in any surface (and in tests) without a Router context — the
// Files page layers its shareable `?view=` URL override on top of this.
//
// Two distinct keys are in use across the app:
//   - "files:viewMode"        — the global Files page (defaults to list).
//   - "files:entityViewMode"  — entity-detail file surfaces, i.e. the
//     commodity Files tab + the location/area panel (defaults to grid,
//     since those surfaces are photo-first).
export function useFilesViewMode(
  storageKey: string,
  defaultMode: FilesViewMode = "list"
): [FilesViewMode, (mode: FilesViewMode) => void] {
  const [mode, setMode] = useState<FilesViewMode>(() => readStored(storageKey, defaultMode))
  const update = (next: FilesViewMode) => {
    setMode(next)
    if (typeof window !== "undefined") {
      window.localStorage.setItem(storageKey, next)
    }
  }
  return [mode, update]
}
