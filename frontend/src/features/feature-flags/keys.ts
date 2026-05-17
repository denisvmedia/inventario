// Single, deployment-scoped key. No slug or user dimension — the
// flags are returned identically to every caller.
export const featureFlagsKeys = {
  all: ["feature-flags"] as const,
}
