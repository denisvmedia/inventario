import { http, HttpResponse } from "msw"

import { FEATURE_FLAGS_FALLBACK, type FeatureFlags } from "@/features/feature-flags/api"

import { apiUrl } from "."

// defaults answers GET /feature-flags with every flag off, the state a
// failed request falls back to. src/test/server.ts installs it for every
// test: GroupProvider reads the plans flag (#1389), so nearly every page
// test asks for the flags, and an unanswered request would reach the
// network. A test that needs a flag on overrides it with `set`.
export function defaults() {
  return [http.get(apiUrl("/feature-flags"), () => HttpResponse.json(FEATURE_FLAGS_FALLBACK))]
}

// set answers GET /feature-flags with the given flags on top of the
// all-off defaults.
export function set(flags: Partial<FeatureFlags>) {
  return [
    http.get(apiUrl("/feature-flags"), () =>
      HttpResponse.json({ ...FEATURE_FLAGS_FALLBACK, ...flags })
    ),
  ]
}
