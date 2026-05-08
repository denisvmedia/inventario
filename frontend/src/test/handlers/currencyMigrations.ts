import { http, HttpResponse } from "msw"

import type {
  CurrencyMigration,
  CurrencyMigrationPreviewBody,
} from "@/features/currency-migration/api"

import { apiUrl } from "."

type MigrationFixture = Partial<CurrencyMigration> & { id: string }

function migrationEnvelope(fix: MigrationFixture) {
  const { id, ...attributes } = fix
  return { id, type: "currency-migrations", attributes }
}

export function list(slug: string, items: MigrationFixture[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/currency-migrations`), () =>
      HttpResponse.json({ data: items.map(migrationEnvelope) })
    ),
  ]
}

export function detail(slug: string, item: MigrationFixture) {
  return [
    http.get(
      apiUrl(`/g/${encodeURIComponent(slug)}/currency-migrations/${encodeURIComponent(item.id)}`),
      () => HttpResponse.json({ data: migrationEnvelope(item) })
    ),
  ]
}

// preview returns a successful preview body. Defaults are a 1-commodity
// group with a clean before→after swap; tests override what they need.
export function preview(slug: string, body: Partial<CurrencyMigrationPreviewBody> = {}) {
  const expiresAt = body.preview_expires_at ?? new Date(Date.now() + 600_000).toISOString()
  const previewBody: CurrencyMigrationPreviewBody = {
    from_currency: "USD",
    to_currency: "EUR",
    exchange_rate: 0.9,
    commodity_count: 1,
    total_current_before: 100,
    total_current_after: 90,
    preview_token: "mock-token",
    preview_expires_at: expiresAt,
    preview_expires_in_seconds: 600,
    diffs: [
      {
        commodity_id: "c1",
        commodity_name: "Demo widget",
        current_price_before: 100,
        current_price_after: 90,
        original_price_before: 100,
        original_price_after: 90,
      },
    ],
    ...body,
  }
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/currency-migrations/preview`), () =>
      HttpResponse.json({
        data: { type: "currency-migration-previews", attributes: previewBody },
      })
    ),
  ]
}

export function start(slug: string, item: MigrationFixture) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/currency-migrations`), () =>
      HttpResponse.json({ data: migrationEnvelope(item) }, { status: 201 })
    ),
  ]
}

// startError returns a coded JSON:API error envelope. The `code` matches
// the BE's stable strings (e.g. "currency_migration.daily_cap_reached")
// so handlers covering 409/422/429 paths exercise the same branching as
// production.
export function startError(
  slug: string,
  status: number,
  code: string,
  meta?: Record<string, string>
) {
  return [
    http.post(apiUrl(`/g/${encodeURIComponent(slug)}/currency-migrations`), () =>
      HttpResponse.json({ errors: [{ code, detail: code, ...(meta ? { meta } : {}) }] }, { status })
    ),
  ]
}
