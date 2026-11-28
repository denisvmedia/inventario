// Query keys for the personal data export slice (#2177). The endpoint reads
// the user from the session, so the key carries no id.
export const dataExportsKeys = {
  all: ["data-exports"] as const,
  list: () => [...dataExportsKeys.all, "list"] as const,
}
