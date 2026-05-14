// Query keys for the login-history slice (#1379). The endpoint is
// user-scoped via the JWT, so no id is part of the key — and a
// trailing limit segment so two simultaneous queries with different
// caps don't share a cache entry.
export const loginHistoryKeys = {
  all: ["login-history"] as const,
  list: (limit: number) => [...loginHistoryKeys.all, "list", limit] as const,
}
