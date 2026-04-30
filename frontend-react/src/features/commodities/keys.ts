export const commodityKeys = {
  all: ["commodity"] as const,
  list: () => [...commodityKeys.all, "list"] as const,
  values: () => [...commodityKeys.all, "values"] as const,
}
