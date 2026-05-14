// Query keys for the per-user active-sessions slice (#1378).
// Single user-scoped namespace — the sessions endpoint reads the
// auth'd user from the JWT, so the key doesn't carry an id.
export const sessionsKeys = {
  all: ["sessions"] as const,
  list: () => [...sessionsKeys.all, "list"] as const,
}
