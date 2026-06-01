import { useMutation } from "@tanstack/react-query"

import { scanCommodityPhotos, type ScanResult } from "./scanApi"

interface ScanVars {
  photos: File[]
  signal?: AbortSignal
  hint?: string
}

// useScanCommodityPhotos wraps the multipart scan request in a
// `useMutation` so the CommodityFormDialog can drive the four-phase
// state machine (offer → scanning → review/error) off `mutate` /
// `isPending` / `error` without hand-rolling abort + cleanup.
//
// No cache invalidation — the scan is read-only; nothing on the BE
// changes, no list queries need to refetch. The `slug` argument is
// part of the hook signature for symmetry with other group-scoped
// hooks (and so a future caller that needs explicit cross-group
// scanning has a place to thread it through), but the shared `http`
// wrapper reads the active group slug from GroupContext today, so the
// slug isn't forwarded to the request body.
//
// `anonymous` (#1988) routes the request to the public
// /public/commodities/scan endpoint with the group rewrite skipped, for
// the unauthenticated landing-page "add your first item" flow. Defaults
// to false so every existing authenticated caller is unaffected.
export function useScanCommodityPhotos(slug: string, anonymous = false) {
  // Reference `slug` so eslint doesn't drop it from the signature —
  // it's documented in the JSDoc above as the stable scope identifier
  // for this mutation. When per-call overrides land, this becomes
  // `scanCommodityPhotos({ slug, ... })`.
  void slug
  return useMutation<ScanResult, Error, ScanVars>({
    mutationFn: ({ photos, signal, hint }) =>
      scanCommodityPhotos({ photos, signal, hint, anonymous }),
  })
}
