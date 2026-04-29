// Tiny feature-flag reader. Each flag is a Vite env var like
// `VITE_FEATURE_OAUTH_LOGIN=1` that maps to a boolean. Default is off.
//
// Real flag delivery (per-user, remote toggle) is out of scope for the new
// frontend's first slice — for now this just exposes a single chokepoint so
// auth-stub call sites (#1380 2FA, #1394 OAuth) don't sprinkle
// `import.meta.env.VITE_…` directly through the component tree.

// Single source of truth for the supported flag names — keep the union in
// sync if you add a new VITE_FEATURE_<NAME>.
export type FeatureFlag = "OAUTH_LOGIN" | "TWO_FACTOR_AUTH" | "AUTH_STATS_TEASER"

function readFlag(name: FeatureFlag): boolean {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const env = (import.meta as any).env ?? {}
  const raw = env[`VITE_FEATURE_${name}`]
  if (typeof raw === "string") {
    return raw === "1" || raw.toLowerCase() === "true"
  }
  return !!raw
}

export function isFeatureEnabled(name: FeatureFlag): boolean {
  return readFlag(name)
}
