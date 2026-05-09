// Hard-coded enum values + display metadata for commodities. The values
// must match models.CommodityType / models.CommodityStatus on the BE
// (kept in sync via openapi codegen — see src/types/api.d.ts). Adding
// a new value here without a matching BE migration would generate
// requests the server rejects.

export const COMMODITY_TYPES = [
  "white_goods",
  "electronics",
  "equipment",
  "furniture",
  "clothes",
  "other",
] as const

export type CommodityTypeValue = (typeof COMMODITY_TYPES)[number]

// Emoji maps from the design mock (`inventario-design/src/data/mock.ts`)
// — used to anchor each row visually. Kept inline (not i18n) because
// they're decorative; localising would just re-emit the same emoji.
export const COMMODITY_TYPE_ICONS: Record<CommodityTypeValue, string> = {
  white_goods: "🏠",
  electronics: "💻",
  equipment: "🔧",
  furniture: "🪑",
  clothes: "👕",
  other: "📦",
}

export const COMMODITY_STATUSES = ["in_use", "sold", "lost", "disposed", "written_off"] as const

export type CommodityStatusValue = (typeof COMMODITY_STATUSES)[number]

// Tailwind classes for each status pill. The status-* tokens live in
// src/index.css; the dark-mode variants are already pre-shifted there
// so a `text-status-active` reads correctly in both themes.
export const COMMODITY_STATUS_TONES: Record<CommodityStatusValue, string> = {
  in_use: "text-status-active border-status-active/30 bg-status-active/10",
  sold: "text-status-expiring border-status-expiring/30 bg-status-expiring/10",
  lost: "text-status-expired border-status-expired/30 bg-status-expired/10",
  disposed: "text-muted-foreground border-border bg-muted",
  written_off: "text-muted-foreground border-border bg-muted",
}

export const COMMODITY_SORT_OPTIONS = [
  "name",
  "registered_date",
  "purchase_date",
  "current_price",
  "original_price",
  "count",
] as const

export type CommoditySortOption = (typeof COMMODITY_SORT_OPTIONS)[number]

// Warranty status the list-page filter dropdown exposes. The values
// mirror the design mock's `WarrantyStatus` union and the
// `--status-{active,expiring,expired,none}` design tokens. The set
// matches `models.WarrantyStatus` on the BE 1:1; the
// `warranty_status=` query param accepts the same tokens.
export const COMMODITY_WARRANTY_STATUSES = ["active", "expiring", "expired", "none"] as const

export type CommodityWarrantyStatus = (typeof COMMODITY_WARRANTY_STATUSES)[number]

// WARRANTY_EXPIRING_DAYS — items inside this window count as "expiring
// soon". 60 days matches `models.WarrantyExpiringWindowDays` so the
// FE pill and the BE filter agree on the boundary.
const WARRANTY_EXPIRING_DAYS = 60

// warrantyStatus derives the warranty bucket from a commodity's
// `warranty_expires_at` column (#1367). The pre-#1535 fallback that
// read the date out of a `warranty:YYYY-MM-DD` tag was removed once
// migration 1779400000 drained those tags into the column.
export function warrantyStatus(input: { warranty_expires_at?: string }): CommodityWarrantyStatus {
  const effective = effectiveWarrantyExpiry(input)
  if (!effective) return "none"
  const ms = parseWarrantyDate(effective)
  return ms ? classifyDays(ms) : "none"
}

// effectiveWarrantyExpiry returns the YYYY-MM-DD string the rest of
// the warranty UI should render, or `undefined` when no warranty is
// tracked. Kept as a thin alias so the call sites keep reading like
// `effectiveWarrantyExpiry(commodity)` instead of a raw field access
// — the helper used to dual-source from a legacy tag (see migration
// 1779400000), and the call sites still want to be coupled to a
// single source of truth so a future move (e.g., a separate warranty
// table) is a one-line change.
export function effectiveWarrantyExpiry(input: {
  warranty_expires_at?: string
}): string | undefined {
  return input.warranty_expires_at || undefined
}

function parseWarrantyDate(s: string | undefined): number | null {
  if (!s) return null
  const t = Date.parse(`${s}T00:00:00Z`)
  return Number.isNaN(t) ? null : t
}

// classifyDays buckets a parsed expiry-date timestamp against today's
// UTC midnight — same anchor as the BE's models.ComputeWarrantyStatus.
// Using `Date.now()` directly here would let the status flip mid-day
// (e.g., "expiring" at 23:00 UTC, "expired" at 00:30 UTC the next day
// purely from the wall-clock hours offset) and disagree with the
// server-side filter, which buckets by whole UTC days.
function classifyDays(expiresAt: number): CommodityWarrantyStatus {
  const now = new Date()
  const todayUTC = Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate())
  const days = (expiresAt - todayUTC) / (1000 * 60 * 60 * 24)
  if (days < 0) return "expired"
  if (days <= WARRANTY_EXPIRING_DAYS) return "expiring"
  return "active"
}

// Tone classes for the warranty pill used to live here as
// `COMMODITY_WARRANTY_TONES`. Per #1529 the canonical rendering surface
// is `<WarrantyBadge>` (frontend/src/components/warranty/) which reads
// `WARRANTY_STATUS_CONFIG`; importing tones from here is no longer
// supported. New code: render `<WarrantyBadge status={...} />` instead.
