import { cn } from "@/lib/utils"

import type { TagColor } from "@/features/tags/api"

// Tone classes are intentionally written out, not built via template
// strings, so Tailwind's content scanner picks each variant up at build
// time. Each token resolves to `var(--tag-{color})` (defined in
// index.css) and the same name works in both light and dark mode — only
// the underlying OKLCH lightness shifts.
const TONE: Record<TagColor, string> = {
  amber: "text-tag-amber border-tag-amber/40 bg-tag-amber/10",
  green: "text-tag-green border-tag-green/40 bg-tag-green/10",
  blue: "text-tag-blue border-tag-blue/40 bg-tag-blue/10",
  orange: "text-tag-orange border-tag-orange/40 bg-tag-orange/10",
  red: "text-tag-red border-tag-red/40 bg-tag-red/10",
  muted: "text-tag-muted border-tag-muted/40 bg-tag-muted/10",
}

export interface TagBadgeProps {
  label: string
  color: TagColor
  size?: "sm" | "md"
  className?: string
  testId?: string
}

export function TagBadge({ label, color, size = "md", className, testId }: TagBadgeProps) {
  return (
    <span
      data-testid={testId}
      className={cn(
        "inline-flex items-center rounded-full border font-medium",
        TONE[color],
        size === "sm" ? "h-5 px-2 text-[11px]" : "h-6 px-2.5 text-xs",
        className
      )}
    >
      {label}
    </span>
  )
}
