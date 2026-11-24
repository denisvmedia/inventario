// Initials for a user who has no profile photo: the first letter of each of the
// first two words of their name ("Alex Johnson" → "AJ"), else the first letter
// of their email ("alex@example.com" → "A"), else "?" so the tile is never
// blank. Letters are taken by code point, so a name that starts with a letter
// outside the Basic Multilingual Plane is not cut in half.
export function userInitials(name?: string | null, email?: string | null): string {
  const words = (name ?? "").trim().split(/\s+/).filter(Boolean)
  if (words.length > 0) {
    return words
      .slice(0, 2)
      .map((word) => Array.from(word)[0] ?? "")
      .join("")
      .toUpperCase()
  }
  const first = Array.from((email ?? "").trim())[0]
  return first ? first.toUpperCase() : "?"
}
