import { useEffect } from "react"

import { setNumberFormatLocaleOverride } from "@/lib/numberFormatLocale"
import { useUserSettings } from "@/features/settings/hooks"

// NumberFormatLocaleSync mirrors `appearance.number_format_locale` from
// the per-user settings endpoint into the in-memory override consumed
// by `frontend/src/lib/intl.ts`. The component renders nothing and is
// mounted once inside the authenticated Shell — the same level
// AppearanceSection's `useUserSettings()` runs at, so the underlying
// query is shared and there's no extra network roundtrip.
//
// Why a Settings → store mirror at all: `currentLocale()` is called
// from non-React code paths (date formatters in test helpers, lib
// utilities that don't subscribe to React context), so the override
// needs to be readable synchronously. The store also persists the
// last-seen value to localStorage, which keeps cold-boot currency
// rendering stable before the settings query resolves.
export function NumberFormatLocaleSync() {
  const settingsQuery = useUserSettings()
  const locale = settingsQuery.data?.appearanceNumberFormatLocale
  // settingsQuery is loading/error tolerant — only mirror once data is
  // present; before that the store keeps whatever the previous session
  // wrote to localStorage (or null, which falls through to navigator).
  useEffect(() => {
    if (!settingsQuery.data) return
    setNumberFormatLocaleOverride(locale ?? null)
  }, [settingsQuery.data, locale])
  return null
}
