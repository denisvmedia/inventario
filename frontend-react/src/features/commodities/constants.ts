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

export const COMMODITY_STATUSES = [
  "in_use",
  "sold",
  "lost",
  "disposed",
  "written_off",
] as const

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
