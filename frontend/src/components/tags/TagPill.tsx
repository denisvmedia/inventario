import { Hash, X } from "lucide-react"
import { Link } from "react-router-dom"

import { cn } from "@/lib/utils"

import type { TagColor } from "@/features/tags/api"

// Tone per tag color, in the mock's proportions (`bg-*/15`, `border-*/30`,
// full-strength text). Written out so Tailwind's scanner sees every class.
const TONE: Record<TagColor, string> = {
  amber: "text-tag-amber border-tag-amber/30 bg-tag-amber/15",
  green: "text-tag-green border-tag-green/30 bg-tag-green/15",
  blue: "text-tag-blue border-tag-blue/30 bg-tag-blue/15",
  orange: "text-tag-orange border-tag-orange/30 bg-tag-orange/15",
  red: "text-tag-red border-tag-red/30 bg-tag-red/15",
  muted: "text-tag-muted border-tag-muted/30 bg-tag-muted/15",
}

export interface TagPillProps {
  label: string
  color: TagColor
  size?: "sm" | "xs"
  // When set, the pill is a link, e.g. to the items list filtered by
  // the tag.
  href?: string
  onRemove?: () => void
  removeLabel?: string
  className?: string
  "data-testid"?: string
}

// A tag as the mock draws it (design-mocks/src/components/TagPill.tsx):
// a rounded pill in the tag's color with a `#` glyph and the label.
export function TagPill({
  label,
  color,
  size = "sm",
  href,
  onRemove,
  removeLabel,
  className,
  ...rest
}: TagPillProps) {
  const classes = cn(
    "inline-flex items-center gap-1 rounded-full border font-medium select-none",
    size === "xs" ? "h-4 px-1.5 text-[10px]" : "h-5 px-2 text-xs",
    TONE[color] ?? TONE.muted,
    href &&
      "transition-opacity hover:opacity-80 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-ring/50",
    className
  )
  const content = (
    <>
      <Hash aria-hidden="true" className={cn("shrink-0", size === "xs" ? "size-2.5" : "size-3")} />
      {label}
      {onRemove ? (
        <button
          type="button"
          onClick={(e) => {
            e.preventDefault()
            e.stopPropagation()
            onRemove()
          }}
          className="ml-0.5 opacity-60 transition-opacity hover:opacity-100"
          aria-label={removeLabel ?? label}
        >
          <X aria-hidden="true" className={size === "xs" ? "size-2.5" : "size-3"} />
        </button>
      ) : null}
    </>
  )
  if (href) {
    return (
      <Link to={href} className={classes} data-testid={rest["data-testid"]}>
        {content}
      </Link>
    )
  }
  return (
    <span className={classes} data-testid={rest["data-testid"]}>
      {content}
    </span>
  )
}
