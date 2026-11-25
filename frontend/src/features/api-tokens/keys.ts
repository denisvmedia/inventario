// Query keys for the personal API token slice (#2808). The endpoint reads
// the user from the session, so the key carries no id.
export const apiTokensKeys = {
  all: ["api-tokens"] as const,
  list: () => [...apiTokensKeys.all, "list"] as const,
}
