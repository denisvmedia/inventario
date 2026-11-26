import { compareVersions, normalizeVersion } from "@/lib/changelog"

// hasUnreadRelease decides the What's new dot (#1386): the build carries a
// release newer than the last one the user saw. A user who never opened
// the page (no stored version) sees it once for the build's newest
// release. A stored version newer than the build, as after a server
// rollback, shows no dot.
export function hasUnreadRelease(latest: string | null, lastSeen: string | null | undefined): boolean {
  if (!latest) return false
  if (!lastSeen || !normalizeVersion(lastSeen)) return true
  return compareVersions(latest, lastSeen) > 0
}
