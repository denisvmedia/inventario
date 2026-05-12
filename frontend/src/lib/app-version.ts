// Application version surfaced in the UI (Settings → Help → "What's new"
// row badge, and the version footer). Sourced from package.json at
// build time via Vite's `define` injection in `frontend/vite.config.ts`,
// which substitutes the literal identifier `__APP_VERSION__` with the
// package's `version` field. We use a dedicated global (not
// `import.meta.env.VITE_APP_VERSION`) because Vite's `define` only
// replaces EXACT expressions — any optional chaining or destructuring
// silently bypasses the substitution and ships the fallback to prod.
// Unit tests run outside Vite, so the read falls back to the literal
// "0.1.0" baked in below — keep it roughly in sync with package.json so
// test fixtures don't drift.
//
// We intentionally expose only the marketing-friendly `Major.Minor`
// form for the badge (the patch number is mostly noise to users); the
// full string is kept for the footer.
declare const __APP_VERSION__: string | undefined

const PACKAGE_VERSION = typeof __APP_VERSION__ === "string" ? __APP_VERSION__ : "0.1.0"

export const APP_VERSION = PACKAGE_VERSION

// Marketing version: trim trailing patch + prerelease so "0.1.0" → "0.1",
// "1.2.3" → "1.2", "1.2.0-beta.1" → "1.2". Used in the v{{version}}
// badge so it tracks the major.minor of the shipped build without
// churning every patch release.
export function shortAppVersion(version: string = APP_VERSION): string {
  const trimmed = version.replace(/^v/, "").split("-")[0]
  const [major, minor] = trimmed.split(".")
  if (!major) return version
  if (!minor) return major
  return `${major}.${minor}`
}
