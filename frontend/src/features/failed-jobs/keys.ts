import type { FailedJobsParams } from "./api"

// TanStack Query keys for a group's failed jobs. Keyed by the group slug so
// switching groups never shows another group's list.
export const failedJobKeys = {
  all: ["failed-jobs"] as const,
  group: (slug: string) => [...failedJobKeys.all, "group", slug] as const,
  groupList: (slug: string, params: FailedJobsParams) =>
    [...failedJobKeys.group(slug), "list", params] as const,
}
