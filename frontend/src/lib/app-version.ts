// Application version surfaced in the UI: the Settings → Help → "What's new"
// row badge, and the diagnostics a feedback report carries. The build
// substitutes the literal identifier `__APP_VERSION__` with the version it
// was given (see `frontend/vite.config.ts`): the release tag for a release
// image, `pr-123` or `edge` for CI images, `dev` for a local build. We use a
// dedicated global (not `import.meta.env.X`) because Vite's `define` only
// replaces EXACT expressions — any optional chaining or destructuring
// silently bypasses the substitution and ships the fallback to prod.
// Unit tests run outside Vite, so the read falls back to "dev", the same
// default as the build and the Go binary; a test that needs a release
// version sets the global with `vi.stubGlobal`, which is why this is read on
// each call and not once at import.
declare const __APP_VERSION__: string | undefined

export function appVersion(): string {
  return typeof __APP_VERSION__ === "string" ? __APP_VERSION__ : "dev"
}

// A release version, with or without the "v": "1.2.3", "v1.2.3",
// "v1.2.3-rc.1", or the `git describe` form "v1.2.3-4-gabc1234-dirty".
const RELEASE_VERSION = /^v?(\d+)\.(\d+)(?:\.\d+)?(?:[-+].*)?$/

// Marketing version for the v{{version}} badge: "Major.Minor" of a release
// version ("v1.2.3" → "1.2", "1.2.0-beta.1" → "1.2"), so the badge tracks
// the shipped build without churning every patch release. Returns null for a
// build with no release number ("dev", "edge", "pr-123"), which shows no
// badge.
export function shortAppVersion(version: string = appVersion()): string | null {
  const match = RELEASE_VERSION.exec(version.trim())
  return match ? `${match[1]}.${match[2]}` : null
}
