// Asset IDs are the per-group running numbers the backend assigns to every
// item on creation. They are stored as a plain integer (`asset_number`)
// and displayed as `A-0042`, padded to four digits and growing past that.
// Keep this in step with models.FormatAssetID in go/models/commodity.go:
// search parses the same shape back into the number.
const ASSET_ID_PREFIX = "A-"
const ASSET_ID_DIGITS = 4

export function formatAssetId(assetNumber: number | null | undefined): string {
  if (assetNumber == null || !Number.isInteger(assetNumber) || assetNumber <= 0) return ""
  return ASSET_ID_PREFIX + String(assetNumber).padStart(ASSET_ID_DIGITS, "0")
}
