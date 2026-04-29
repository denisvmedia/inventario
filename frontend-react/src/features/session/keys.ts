// Query-key factory for the session feature slice. Each feature slice owns
// its own keys so other slices invalidate via the typed entry point rather
// than copy-pasting key arrays.
export const sessionKeys = {
  all: ["session"] as const,
  currentUser: () => [...sessionKeys.all, "currentUser"] as const,
}
