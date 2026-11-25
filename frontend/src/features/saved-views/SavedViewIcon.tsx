import { Bookmark } from "lucide-react"

import { SAVED_VIEW_ICONS } from "./view"

interface SavedViewIconProps {
  // The icon name the server stores; an unknown name shows a bookmark.
  name: string
  className?: string
}

export function SavedViewIcon({ name, className }: SavedViewIconProps) {
  const Icon = SAVED_VIEW_ICONS[name] ?? Bookmark
  return <Icon className={className} aria-hidden="true" />
}
