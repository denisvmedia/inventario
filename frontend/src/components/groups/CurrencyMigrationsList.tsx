import { useTranslation } from "react-i18next"

import { CurrencyMigrationStatusBadge } from "@/components/groups/CurrencyMigrationStatusBadge"
import { Skeleton } from "@/components/ui/skeleton"
import type { Migration } from "@/features/currency-migration/api"
import { formatDateTime } from "@/lib/intl"

export interface CurrencyMigrationsListProps {
  loading: boolean
  migrations: Migration[]
}

// Lists currency migrations for the current group, latest 10. There is
// no "view detail" navigation yet — the BE's GET /currency-migrations/{id}
// endpoint exists but a dedicated detail page is out of scope for #1553;
// the row carries enough context (rate, status, who/when) for an
// admin to triage at a glance.
export function CurrencyMigrationsList({ loading, migrations }: CurrencyMigrationsListProps) {
  const { t } = useTranslation()
  if (loading) {
    return (
      <div className="flex flex-col gap-2" data-testid="migrations-loading">
        {Array.from({ length: 3 }).map((_, idx) => (
          <Skeleton key={idx} className="h-12 w-full" />
        ))}
      </div>
    )
  }
  if (migrations.length === 0) {
    return (
      <p
        className="rounded-md border bg-muted/30 px-4 py-6 text-center text-sm text-muted-foreground"
        data-testid="migrations-empty"
      >
        {t("groups:settings.migrationsEmpty")}
      </p>
    )
  }
  return (
    <ul className="flex flex-col gap-2" data-testid="migrations-list">
      {migrations.slice(0, 10).map((m) => (
        <li
          key={m.id}
          className="flex flex-wrap items-center justify-between gap-3 rounded-md border bg-card px-4 py-3"
          data-testid={`migration-row-${m.id}`}
        >
          <div className="flex min-w-0 flex-col gap-1">
            <div className="flex flex-wrap items-center gap-2 text-sm font-medium font-mono">
              <span>{m.from_currency ?? "—"}</span>
              <span aria-hidden="true">→</span>
              <span>{m.to_currency ?? "—"}</span>
              {m.exchange_rate !== undefined ? (
                <span className="text-xs font-normal text-muted-foreground">
                  {t("groups:migration.rateLabel", {
                    from: m.from_currency,
                    to: m.to_currency,
                    rate: String(m.exchange_rate),
                  })}
                </span>
              ) : null}
            </div>
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              {m.created_at ? <span>{formatDateTime(m.created_at)}</span> : <span>—</span>}
              {m.commodity_count !== undefined ? (
                <>
                  <span aria-hidden="true">·</span>
                  <span>{m.commodity_count}</span>
                </>
              ) : null}
            </div>
          </div>
          {m.status ? <CurrencyMigrationStatusBadge status={m.status} /> : null}
        </li>
      ))}
    </ul>
  )
}
