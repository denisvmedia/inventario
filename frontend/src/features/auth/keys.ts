// Query-key factory for the auth feature slice. Each feature slice owns its
// own keys so other slices invalidate via the typed entry point rather than
// copy-pasting key arrays.
export const authKeys = {
  all: ["auth"] as const,
  currentUser: () => [...authKeys.all, "currentUser"] as const,
}
