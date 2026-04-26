import { computed, ref, watch } from 'vue'

export type ThemePreference = 'light' | 'dark' | 'system'
export type ResolvedTheme = 'light' | 'dark'

const STORAGE_KEY = 'inventario:theme'
const VALID: readonly ThemePreference[] = ['light', 'dark', 'system'] as const

const preference = ref<ThemePreference>(readInitial())

function readInitial(): ThemePreference {
  if (typeof window === 'undefined') return 'system'
  const stored = window.localStorage.getItem(STORAGE_KEY)
  if (stored && (VALID as readonly string[]).includes(stored)) {
    return stored as ThemePreference
  }
  return 'system'
}

function systemPrefersDark(): boolean {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    return false
  }
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function resolve(pref: ThemePreference): ResolvedTheme {
  if (pref === 'light' || pref === 'dark') return pref
  return systemPrefersDark() ? 'dark' : 'light'
}

function applyToDocument(resolved: ResolvedTheme) {
  if (typeof document === 'undefined') return
  const root = document.documentElement
  root.dataset.theme = resolved
  root.classList.toggle('dark', resolved === 'dark')
}

const resolved = ref<ResolvedTheme>(resolve(preference.value))

watch(
  preference,
  (next) => {
    resolved.value = resolve(next)
    applyToDocument(resolved.value)
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(STORAGE_KEY, next)
    }
  },
  { immediate: true },
)

if (typeof window !== 'undefined' && typeof window.matchMedia === 'function') {
  // Tied to document lifetime — no cleanup needed.
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (preference.value === 'system') {
      resolved.value = resolve('system')
      applyToDocument(resolved.value)
    }
  })
}

/**
 * Reactive access to the user's theme preference. Light/dark/system; persisted
 * to localStorage and reapplied to `<html data-theme="..."> + .dark` on boot.
 */
export function useTheme() {
  return {
    preference: computed(() => preference.value),
    resolved: computed(() => resolved.value),
    setTheme(next: ThemePreference) {
      if ((VALID as readonly string[]).includes(next)) {
        preference.value = next
      }
    },
  }
}

export function initThemeOnBoot() {
  applyToDocument(resolved.value)
}
