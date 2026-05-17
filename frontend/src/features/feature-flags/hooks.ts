import { useQuery } from "@tanstack/react-query"

import { FEATURE_FLAGS_FALLBACK, getFeatureFlags, type FeatureFlags } from "./api"
import { featureFlagsKeys } from "./keys"

// Feature flags don't change at runtime — flipping a flag requires a
// re-deploy. So we set staleTime: Infinity, gcTime: Infinity and let
// the result live for the lifetime of the SPA. No retries: a stable
// "everything off" fallback is better than a flicker between "shown
// because nothing loaded" and "hidden because the fetch eventually
// failed".
export function useFeatureFlags() {
  return useQuery<FeatureFlags>({
    queryKey: featureFlagsKeys.all,
    queryFn: ({ signal }) => getFeatureFlags(signal),
    staleTime: Infinity,
    gcTime: Infinity,
    retry: false,
  })
}

// Convenience selector for the common case: "is feature X on?". Returns
// the FALLBACK value while the query is loading or in an error state so
// gated entry points stay hidden by default — fail-closed rather than
// fail-open (matches the rationale in FEATURE_FLAGS_FALLBACK).
export function useFeatureFlag<K extends keyof FeatureFlags>(name: K): FeatureFlags[K] {
  const { data } = useFeatureFlags()
  if (!data) return FEATURE_FLAGS_FALLBACK[name]
  return data[name]
}
