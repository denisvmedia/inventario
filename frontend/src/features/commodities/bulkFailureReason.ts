import type { TFunction } from "i18next"

import type { CommodityBulkFailure } from "./api"

// bulkFailureReason says in the user's language why a bulk action could not
// change an item. A validation failure (code invalid) comes with the field
// the server refused and the validator's code; the codes this knows get a
// translated sentence, and any other keeps the server's English message, so
// a new rule on the server still says something useful.
export function bulkFailureReason(t: TFunction, f: CommodityBulkFailure): string {
  switch (f.code) {
    case "not_found":
      return t("commodities:bulk.result.notFound")
    case "not_sent":
      return t("commodities:bulk.result.notSent")
    case "invalid":
      return invalidReason(t, f.field ?? "", f.reason ?? "") ?? (f.error || failed(t))
    default:
      return failed(t)
  }
}

function failed(t: TFunction): string {
  return t("commodities:bulk.result.failed")
}

function invalidReason(t: TFunction, field: string, reason: string): string | undefined {
  // Rules with a sentence of their own, whatever field carries them.
  switch (reason) {
    case "status_date_required_on_transition":
      return t("commodities:detail.statusTransitionDialog.errors.dateRequired")
    case "sale_price_required_for_sold":
      return t("commodities:detail.statusTransitionDialog.errors.salePriceRequired")
    case "quantity_forbids_warranty":
    case "quantity_forbids_warranty_notes":
      return t("commodities:validation.quantityForbidsWarranty")
    case "validation_no_price_in_group_currency":
      return t("commodities:validation.noPriceInGroupCurrency")
    case "status_forbids_sale_price":
      return t("commodities:bulk.result.invalid.statusForbidsSalePrice")
    case "status_forbids_status_date":
      return t("commodities:bulk.result.invalid.statusForbidsStatusDate")
  }

  const label = fieldLabel(t, field)
  if (label === undefined) return undefined
  if (isMinRule(reason) && isPrice(field)) return t("commodities:validation.priceNegative")
  if (isMinRule(reason) && field === "count") return t("commodities:validation.countMin")
  switch (reason) {
    case "validation_required":
    case "validation_is_empty":
    case "validation_nil_or_not_empty_required":
    case "validation_not_nil_required":
      return t("commodities:bulk.result.invalid.required", { field: label })
    case "validation_length_too_long":
    case "validation_length_out_of_range":
      return t("commodities:bulk.result.invalid.tooLong", { field: label })
    case "validation_in_invalid":
    case "invalid_type":
    case "invalid_status":
      return t("commodities:bulk.result.invalid.notAllowed", { field: label })
    case "validation_match_invalid":
    case "validation_date_invalid":
      return t("commodities:bulk.result.invalid.format", { field: label })
    default:
      return undefined
  }
}

function isMinRule(reason: string): boolean {
  return (
    reason === "validation_min_greater_equal_than_required" ||
    reason === "validation_min_greater_than_required"
  )
}

function isPrice(field: string): boolean {
  return (
    field === "original_price" ||
    field === "converted_original_price" ||
    field === "current_price" ||
    field === "sale_price"
  )
}

// fieldLabel names an item field the server reports, in the words of the
// item form. Each key is spelled out so the i18n extractor sees it.
function fieldLabel(t: TFunction, field: string): string | undefined {
  switch (field) {
    case "name":
      return t("commodities:fields.name")
    case "short_name":
      return t("commodities:fields.shortName")
    case "brand":
      return t("commodities:fields.brand")
    case "model":
      return t("commodities:fields.model")
    case "type":
      return t("commodities:fields.type")
    case "status":
      return t("commodities:fields.status")
    case "count":
      return t("commodities:fields.count")
    case "purchase_date":
      return t("commodities:fields.purchaseDate")
    case "original_price":
      return t("commodities:fields.originalPrice")
    case "original_price_currency":
      return t("commodities:bulk.result.invalid.currencyField")
    case "current_price":
      return t("commodities:fields.currentPrice")
    case "warranty_expires_at":
      return t("commodities:fields.warrantyExpiresAt")
    case "warranty_notes":
      return t("commodities:fields.warrantyNotes")
    case "urls":
      return t("commodities:fields.urls")
    case "tags":
      return t("commodities:fields.tags")
    case "status_date":
      return t("commodities:detail.statusTransitionDialog.dateLabel")
    case "sale_price":
      return t("commodities:detail.statusTransitionDialog.salePriceLabel")
    default:
      return undefined
  }
}
