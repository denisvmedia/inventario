import { useEffect, useRef } from "react"
import { useQueryClient } from "@tanstack/react-query"
import whatsNewMeta from "virtual:whats-new/meta"

import { SETTING_WHATS_NEW_LAST_SEEN_VERSION } from "@/features/settings/api"
import { usePatchSetting, useUserSettings } from "@/features/settings/hooks"
import { settingsKeys } from "@/features/settings/keys"

import { hasUnreadRelease } from "./unread"

// useWhatsNewUnread is true while the build carries a release the user has
// not seen on the What's new page. The last seen version is a per-user
// server setting, so the dot follows the user across devices. It stays off
// until the settings have loaded: a dot that appears and vanishes on every
// load would read as a glitch.
export function useWhatsNewUnread(): boolean {
  const settings = useUserSettings()
  if (!settings.data) return false
  return hasUnreadRelease(whatsNewMeta.latestVersion, settings.data.whatsNewLastSeenVersion)
}

// useMarkWhatsNewSeen stores the build's newest release as seen once the
// page has opened, which clears the dot. It never lowers a stored version
// that is newer than the build's.
export function useMarkWhatsNewSeen(): void {
  const qc = useQueryClient()
  const settings = useUserSettings()
  const { mutate } = usePatchSetting()
  const sent = useRef(false)
  const loaded = settings.data !== undefined
  const lastSeen = settings.data?.whatsNewLastSeenVersion
  useEffect(() => {
    const latest = whatsNewMeta.latestVersion
    if (sent.current || !loaded || !latest || !hasUnreadRelease(latest, lastSeen)) return
    sent.current = true
    mutate(
      { field: SETTING_WHATS_NEW_LAST_SEEN_VERSION, value: latest },
      {
        // The setting belongs to the user, but the settings are cached per
        // group; refetch the other groups' copies so switching groups does
        // not bring the dot back.
        onSuccess: () => void qc.invalidateQueries({ queryKey: settingsKeys.all }),
      }
    )
  }, [loaded, lastSeen, mutate, qc])
}
