export const groupKeys = {
  all: ["group"] as const,
  list: () => [...groupKeys.all, "list"] as const,
  detail: (groupId: string) => [...groupKeys.all, "detail", groupId] as const,
  members: (groupId: string) => [...groupKeys.all, "members", groupId] as const,
  invites: (groupId: string) => [...groupKeys.all, "invites", groupId] as const,
}
