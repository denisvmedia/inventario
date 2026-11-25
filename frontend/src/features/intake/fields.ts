import { useTranslation } from "react-i18next"

import type { IntakeReviewField } from "./api"

// fieldLabelKey maps a review field onto the `commodities:fields.*` labels
// the item form uses, so a field reads the same in the review and in the
// draft it becomes.
function fieldLabelKey(field: Exclude<IntakeReviewField, "original_price_currency">): string {
  switch (field) {
    case "short_name":
      return "shortName"
    case "serial_number":
      return "serialNumber"
    case "purchase_date":
      return "purchaseDate"
    case "original_price":
      return "originalPrice"
    case "warranty_expires_at":
      return "warrantyExpiresAt"
    default:
      return field
  }
}

export function useIntakeFieldLabel(): (field: IntakeReviewField) => string {
  const { t } = useTranslation()
  return (field) =>
    field === "original_price_currency"
      ? t("commodities:intake.currency")
      : t(`commodities:fields.${fieldLabelKey(field)}`)
}
