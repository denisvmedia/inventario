import { useOptionalCurrentGroup } from "@/features/group/GroupContext"

interface MigrationLock {
  // True when a currency migration for the active group is pending or
  // running. Disable commodity write CTAs (Add/Edit/Delete) and the
  // export restore start CTA when this is true.
  locked: boolean
  // The currency_migrations.id when locked, otherwise undefined.
  migrationId?: string
}

// Lightweight selector built on the existing GroupContext — the BE
// exposes `currency_migration_id` on LocationGroup as a read-only
// attribute (issue #202 / PR #1578) and that's the only signal we need
// to decide whether a write is currently locked. Returns `locked: false`
// outside of <GroupProvider> so the banner / disabled CTAs degrade
// silently on chrome surfaces that render before the provider mounts.
export function useGroupMigrationLock(): MigrationLock {
  const ctx = useOptionalCurrentGroup()
  const id = ctx?.currentGroup?.currency_migration_id
  if (id) {
    return { locked: true, migrationId: id }
  }
  return { locked: false }
}
