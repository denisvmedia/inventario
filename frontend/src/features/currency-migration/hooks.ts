import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { useCurrentGroup } from "@/features/group/GroupContext"
import { groupKeys } from "@/features/group/keys"

import {
  getMigration,
  isCurrencyMigrationTerminal,
  listMigrations,
  previewMigration,
  startMigration,
  type Migration,
  type MigrationPreview,
  type PreviewRequest,
  type StartRequest,
} from "./api"
import { currencyMigrationKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

// Polling cadence for in-flight migrations. The issue specifies 5s; a single
// migration is one transaction and "running" is short — this is enough for
// the user to see the pending → running → completed flip without flooding
// the BE. TanStack Query backs off when the tab is hidden so this is safe
// to leave on every list mount.
const POLL_INTERVAL_MS = 5_000

export function useCurrencyMigrations({ enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<{ migrations: Migration[] }>({
    queryKey: currencyMigrationKeys.list(slug),
    queryFn: ({ signal }) => listMigrations(signal),
    enabled: enabled && !!slug,
    placeholderData: (prev) => prev,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data) return false
      const anyInFlight = data.migrations.some((m) => !isCurrencyMigrationTerminal(m.status))
      return anyInFlight ? POLL_INTERVAL_MS : false
    },
  })
}

export function useCurrencyMigration(
  id: string | undefined,
  { enabled = true }: QueryOptions = {}
) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<Migration>({
    queryKey: currencyMigrationKeys.detail(slug, id ?? ""),
    queryFn: ({ signal }) => {
      if (!id) throw new Error("useCurrencyMigration called without an id")
      return getMigration(id, signal)
    },
    enabled: enabled && !!id && !!slug,
    refetchInterval: (query) => {
      const data = query.state.data
      if (!data) return false
      return isCurrencyMigrationTerminal(data.status) ? false : POLL_INTERVAL_MS
    },
  })
}

// Preview is a mutation rather than a query because (a) it has side
// effects on the BE (rate-limits, audit hint) and (b) the call site
// fires it once per "Continue" click in the wizard, not on focus.
//
// The preview body carries a 10-min `preview_token` plus rendering data
// (totals, top-N diffs). The wizard stores the result in component state
// and treats it as the source of truth until the user lands on the
// confirm step.
//
// Not invalidating any caches — preview neither creates nor mutates a
// migration row.
export function usePreviewMigration() {
  return useMutation<MigrationPreview, Error, PreviewRequest>({
    mutationFn: (req) => previewMigration(req),
    // Suppress the global 5xx toast — the wizard renders its own inline
    // error block. 5xx is rare (BE bug); we still want the wizard to
    // surface a domain-shaped message rather than a generic toast.
    meta: { suppressGlobalErrorToast: true },
  })
}

// Start invalidates the migrations list (so the new pending row shows up)
// and the group detail (so the LocationGroup.currency_migration_id field
// is fetched again — the lock UX gates off it).
export function useStartMigration() {
  const queryClient = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useMutation<Migration, Error, StartRequest>({
    mutationFn: (req) => startMigration(req),
    meta: { suppressGlobalErrorToast: true },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: currencyMigrationKeys.list(slug) })
      queryClient.invalidateQueries({ queryKey: groupKeys.all })
    },
  })
}
