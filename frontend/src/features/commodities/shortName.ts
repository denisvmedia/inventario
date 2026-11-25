// The short name the item dialog suggests from the name (#2812), so quick
// capture needs one name, not two. The suggestion is the name itself when it
// fits, otherwise its leading whole words.

// Short names are meant for compact lists and labels; the design keeps
// them to 20 characters even though the server accepts 40.
export const SUGGESTED_SHORT_NAME_MAX = 20

// The limit counts code points, as the server's rune count does, so a
// character outside the Basic Multilingual Plane (an emoji, say) counts once
// and is never split in half.
export function suggestShortName(name: string): string {
  const clean = name.trim().replace(/\s+/g, " ")
  const chars = Array.from(clean)
  if (chars.length <= SUGGESTED_SHORT_NAME_MAX) return clean
  const cut = chars.slice(0, SUGGESTED_SHORT_NAME_MAX + 1)
  const lastSpace = cut.lastIndexOf(" ")
  const words = lastSpace > 0 ? cut.slice(0, lastSpace) : cut.slice(0, SUGGESTED_SHORT_NAME_MAX)
  return words.join("").replace(/[\s,.;:–—-]+$/u, "")
}
