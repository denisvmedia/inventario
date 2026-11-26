import { useEffect, useRef, useState } from "react"
import whatsNewMeta from "virtual:whats-new/meta"

import { useOptionalImpersonation } from "@/features/admin/impersonation/ImpersonationContext"
import { useAuth } from "@/features/auth/AuthContext"
import { SETTING_WHATS_NEW_LAST_SEEN_VERSION } from "@/features/settings/api"
import { usePatchSetting, useUserSettings } from "@/features/settings/hooks"

import { announcedRelease, hasUnreadRelease } from "./unread"

function useAnnouncedRelease(): string | null {
  const { user } = useAuth()
  return announcedRelease(whatsNewMeta, user?.is_system_admin === true)
}

// useWhatsNewUnread is true while the build carries a release the user has
// not seen on the What's new page. The last seen version is a per-user
// server setting, so the dot follows the user across devices. It stays off
// until the settings have loaded: a dot that appears and vanishes on every
// load would read as a glitch.
export function useWhatsNewUnread(): boolean {
  const settings = useUserSettings()
  const announced = useAnnouncedRelease()
  if (!settings.data) return false
  return hasUnreadRelease(announced, settings.data.whatsNewLastSeenVersion)
}

// How many times a failed write is tried again while the page stays open.
// Opening the page again starts over.
const MARK_SEEN_RETRIES = 1

// useMarkWhatsNewSeen clears the dot once the page has opened. It stores
// the build's newest release, which is never older than the one the dot is
// about, so a user who later becomes a system administrator is not shown a
// dot for a release they already opened. It never lowers a stored version
// that is newer than the build's, and it stores nothing while a system
// administrator impersonates the user: they read the page, the user did
// not. It writes only once the impersonation probe has answered, so a
// probe that is still loading or has failed blocks the write. A write that
// fails is tried again once; one that succeeds is not repeated.
export function useMarkWhatsNewSeen(): void {
  const settings = useUserSettings()
  const announced = useAnnouncedRelease()
  const impersonation = useOptionalImpersonation()
  // Outside an ImpersonationProvider, as in some tests, nobody can be
  // impersonating.
  const blocked = impersonation ? !impersonation.resolved || impersonation.active : false
  const { mutate } = usePatchSetting()
  // "pending" while a write is in flight, "done" after one succeeded.
  const phase = useRef<"idle" | "pending" | "done">("idle")
  const [failures, setFailures] = useState(0)
  const loaded = settings.data !== undefined
  const lastSeen = settings.data?.whatsNewLastSeenVersion
  useEffect(() => {
    const latest = whatsNewMeta.latestVersion
    if (phase.current !== "idle" || failures > MARK_SEEN_RETRIES) return
    if (blocked || !loaded || !latest || !hasUnreadRelease(announced, lastSeen)) return
    phase.current = "pending"
    mutate(
      { field: SETTING_WHATS_NEW_LAST_SEEN_VERSION, value: latest },
      {
        onSuccess: () => {
          phase.current = "done"
        },
        onError: () => {
          phase.current = "idle"
          setFailures((n) => n + 1)
        },
      }
    )
  }, [announced, blocked, failures, loaded, lastSeen, mutate])
}
