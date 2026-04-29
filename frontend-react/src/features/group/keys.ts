export const groupKeys = {
  all: ["group"] as const,
  list: () => [...groupKeys.all, "list"] as const,
}
