// Query-key factory for the auth feature slice. Each feature slice owns its
// own keys so other slices invalidate via the typed entry point rather than
// copy-pasting key arrays.
export const authKeys = {
  all: ["auth"] as const,
  currentUser: () => [...authKeys.all, "currentUser"] as const,
  mfaStatus: () => [...authKeys.all, "mfaStatus"] as const,
  // OAuth (#1394): the deployment's enabled providers (public, session-stable)
  // and the caller's linked identities (per-user, invalidated on link/unlink).
  oauthProviders: () => [...authKeys.all, "oauthProviders"] as const,
  oauthIdentities: () => [...authKeys.all, "oauthIdentities"] as const,
}
