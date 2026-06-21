import { useEffect } from "react"

import { i18next, SUPPORTED_LANGUAGES, type SupportedLanguage } from "@/i18n"
import { useUserSettings } from "@/features/settings/hooks"

// LanguageSync makes the per-user `appearance.language` setting the
// cross-device source of truth for the UI language. When the authenticated
// settings query resolves with a stored language, it applies it via
// i18next (which re-caches it to localStorage for the next cold boot).
//
// Pre-auth / first paint still uses whatever the i18next language detector
// read from localStorage at init; this only reconciles once the settings
// load, so a user who switched language on another device sees it applied
// here too. It also keeps the language in lockstep with what the backend
// localizes transactional emails in (#2090).
//
// Renders nothing; mounted once inside the authenticated Shell alongside
// NumberFormatLocaleSync so the underlying settings query is shared.
export function LanguageSync() {
  const settingsQuery = useUserSettings()
  const stored = settingsQuery.data?.appearanceLanguage
  useEffect(() => {
    if (!stored) return
    if (!(SUPPORTED_LANGUAGES as readonly string[]).includes(stored)) return
    if (i18next.resolvedLanguage === stored) return
    void i18next.changeLanguage(stored as SupportedLanguage)
  }, [stored])
  return null
}
