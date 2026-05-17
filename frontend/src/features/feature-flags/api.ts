// Data layer for the deployment feature-flag slice (#1616). Single
// public endpoint, no auth, no per-user variation — flipping a flag
// requires a re-deploy so the response is effectively immutable for
// the lifetime of a session. The FE consumes this at boot to hide
// entry points for features whose backend is gated off (e.g. the
// currency-migration wizard).
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type FeatureFlags = Schema<"apiserver.FeatureFlags">

// Defaults match an "everything off" deployment so that a fetch
// failure can never silently enable a feature whose backend is
// disabled. When the request fails (offline, race during boot) we
// degrade to hiding the gated UI rather than showing it — the
// alternative would let the user click through to a request that
// then 404s without context, which is the exact bug #1616 fixes.
export const FEATURE_FLAGS_FALLBACK: FeatureFlags = {
  currency_migration: false,
}

export async function getFeatureFlags(signal?: AbortSignal): Promise<FeatureFlags> {
  // skipGroupRewrite because feature-flags is a tenant/deployment-level
  // surface — it lives at /api/v1/feature-flags directly, no /g/{slug}/
  // prefix. The rewrite is a no-op for un-prefixed paths today, but the
  // explicit opt-out makes the contract obvious and survives future
  // changes to GROUP_SCOPED_PREFIXES.
  return http.get<FeatureFlags>("/feature-flags", { signal, skipGroupRewrite: true })
}
