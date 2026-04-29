// Query-key factory for the invite feature slice.
export const inviteKeys = {
  all: ["invite"] as const,
  info: (token: string) => [...inviteKeys.all, "info", token] as const,
}
