// Query-key factory for the back-office auth slice (#1785 Phase 6). Mirrors
// `features/auth/keys.ts` but lives in its own namespace — the tenant
// `useCurrentUser()` cache and the back-office `useBackofficeMe()` cache
// MUST never collide (a browser may be signed into both planes at once
// and the two `me` responses are different types).
export const backofficeAuthKeys = {
  all: ["backoffice", "auth"] as const,
  me: () => [...backofficeAuthKeys.all, "me"] as const,
}
