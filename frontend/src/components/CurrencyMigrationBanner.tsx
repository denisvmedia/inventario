import { useTranslation } from "react-i18next"
import { Loader2 } from "lucide-react"

import { useOptionalCurrentGroup } from "@/features/group/GroupContext"

// Persistent top-of-layout banner that surfaces the in-flight currency
// migration on the active group (epic #202). Reads
// `currency_migration_id` from the current group; when null, the banner
// is hidden. There's no inline navigation target (#1553 skips a
// dedicated detail route) so we don't render a link — the migrations
// list on the group settings page is the canonical "where is it now"
// surface.
export function CurrencyMigrationBanner() {
  const { t } = useTranslation()
  const ctx = useOptionalCurrentGroup()
  const group = ctx?.currentGroup
  if (!group?.currency_migration_id) return null
  return (
    <div
      className="flex items-center gap-3 border-b border-amber-500/40 bg-amber-500/10 px-4 py-2.5"
      role="status"
      data-testid="currency-migration-banner"
    >
      <div className="flex size-6 shrink-0 items-center justify-center rounded-full bg-amber-500/20">
        <Loader2 className="size-3.5 animate-spin text-amber-700 dark:text-amber-300" />
      </div>
      <p className="flex-1 text-sm text-foreground">
        {t("groups:migration.banner", { name: group.name ?? "" })}
      </p>
    </div>
  )
}
