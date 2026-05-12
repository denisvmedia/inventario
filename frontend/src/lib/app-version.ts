// Application version surfaced in the UI (Settings → Help → "What's new"
// row badge, and the version footer). Sourced from package.json at
// build time via Vite's `define` injection in `frontend/vite.config.ts`
// (replaces every `import.meta.env.VITE_APP_VERSION` literal with the
// package's `version` field at compile time). Unit tests run outside
// Vite, so the read falls back to the literal "0.1.0" baked into the
// module — keep that bump roughly in sync with package.json so test
// fixtures don't drift.
//
// We intentionally expose only the marketing-friendly `Major.Minor`
// form for the badge (the patch number is mostly noise to users); the
// full string is kept for the footer.
const PACKAGE_VERSION = (import.meta.env?.VITE_APP_VERSION as string | undefined) ?? "0.1.0"

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
