import { compareVersions, normalizeVersion, type WhatsNewMeta } from "@/lib/changelog"

// hasUnreadRelease decides the What's new dot (#1386): the build carries a
// release newer than the last one the user saw. A user who never opened
// the page (no stored version) sees it once for the build's newest
// release. A stored version newer than the build, as after a server
// rollback, shows no dot.
export function hasUnreadRelease(
  latest: string | null,
  lastSeen: string | null | undefined
): boolean {
  if (!latest) return false
  if (!lastSeen || !normalizeVersion(lastSeen)) return true
  return compareVersions(latest, lastSeen) > 0
}

// announcedRelease is the release the dot is about for this user. A system
// administrator reads every section, so the build's newest release counts.
// Anyone else does not see Operators sections, so a release that has
// nothing else is skipped: its dot would open to an empty card.
export function announcedRelease(meta: WhatsNewMeta, isSystemAdmin: boolean): string | null {
  return isSystemAdmin ? meta.latestVersion : meta.latestUserVersion
}
