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
// `--status-{active,expiring,expired,none}` design tokens. The set is
// derived client-side from a tag convention until first-class
// warranties ship (#1367) — see `warrantyStatus()` below.
export const COMMODITY_WARRANTY_STATUSES = ["active", "expiring", "expired", "none"] as const

export type CommodityWarrantyStatus = (typeof COMMODITY_WARRANTY_STATUSES)[number]

// EXPIRING_DAYS — items inside this window count as "expiring soon".
// 60 days mirrors the legacy frontend's threshold.
const WARRANTY_EXPIRING_DAYS = 60

// warrantyStatus derives a warranty bucket from the commodity's tags.
// Convention until #1367 lands: a tag of shape `warranty:YYYY-MM-DD`
// records the expiry; anything else surfaces as "none". The list-page
// filter applies the result client-side because the BE has no warranty
// column today.
export function warrantyStatus(tags: readonly string[] | undefined): CommodityWarrantyStatus {
  if (!tags || tags.length === 0) return "none"
  const expiry = tags.map((t) => /^warranty:(\d{4}-\d{2}-\d{2})$/.exec(t)).find((m) => m !== null)
  if (!expiry) return "none"
  const expiresAt = new Date(`${expiry[1]}T00:00:00Z`)
  if (Number.isNaN(expiresAt.getTime())) return "none"
  const today = new Date()
  const days = (expiresAt.getTime() - today.getTime()) / (1000 * 60 * 60 * 24)
  if (days < 0) return "expired"
  if (days <= WARRANTY_EXPIRING_DAYS) return "expiring"
  return "active"
}

// Tone classes for the warranty pill — same pattern as the status
// pills (text/border/bg derived from the design tokens). Used by
// future detail-page warranty surfaces; the filter dropdown itself
// shows plain labels.
export const COMMODITY_WARRANTY_TONES: Record<CommodityWarrantyStatus, string> = {
  active: "text-status-active border-status-active/30 bg-status-active/10",
  expiring: "text-status-expiring border-status-expiring/30 bg-status-expiring/10",
  expired: "text-status-expired border-status-expired/30 bg-status-expired/10",
  none: "text-muted-foreground border-border bg-muted",
}
