import { Hash, X } from "lucide-react"
import { cn } from "@/lib/utils"
import type { Tag } from "@/data/mock"

interface TagPillProps {
  tag: Tag
  size?: "sm" | "xs"
  onRemove?: () => void
  className?: string
}

export function TagPill({ tag, size = "sm", onRemove, className }: TagPillProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1 rounded-full border font-medium select-none",
        size === "xs"
          ? "h-4 px-1.5 text-[10px]"
          : "h-5 px-2 text-xs",
        tag.bg,
        tag.color,
        tag.border,
        className
      )}
    >
      <Hash className={cn("shrink-0", size === "xs" ? "size-2.5" : "size-3")} />
      {tag.label}
      {onRemove && (
        <button
          type="button"
          onClick={(e) => { e.stopPropagation(); onRemove() }}
          className="ml-0.5 opacity-60 hover:opacity-100 transition-opacity"
          aria-label={`Remove tag ${tag.label}`}
        >
          <X className={size === "xs" ? "size-2.5" : "size-3"} />
        </button>
      )}
    </span>
  )
}
