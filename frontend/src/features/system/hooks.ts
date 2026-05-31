import { useQuery } from "@tanstack/react-query"

import { getSystemInfo } from "./api"

export const systemKeys = {
  all: ["system"] as const,
  info: () => [...systemKeys.all, "info"] as const,
}

// Build/system info is immutable for the lifetime of a deploy, so we fetch it
// once and never revalidate — no focus refetch, no retry churn. Currently
// only the CommitBadge consumes it (#1972).
export function useSystemInfo() {
  return useQuery({
    queryKey: systemKeys.info(),
    queryFn: ({ signal }) => getSystemInfo(signal),
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    retry: false,
  })
}
