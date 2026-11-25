import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"

import { Badge } from "@/components/ui/badge"
import type { CustomField } from "@/features/customFields/api"
import { commodityHref } from "@/features/search/results"
import { formatCurrency, formatDate } from "@/lib/intl"

export interface CustomFieldValueProps {
  field: CustomField
  // The stored value in its API form (see CustomFieldValues).
  raw: unknown
  // Names of the items link fields point at (meta.linked_items).
  linkedItems?: Record<string, { name?: string; trashed?: boolean }>
  groupSlug?: string
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v)
}

// CustomFieldValue shows one stored custom field value (#2803) on the item
// page and the printed report, formatted for its type.
export function CustomFieldValue({ field, raw, linkedItems, groupSlug }: CustomFieldValueProps) {
  const { t } = useTranslation()
  switch (field.type) {
    case "boolean":
      return <>{raw === true ? t("customFields:detail.yes") : t("customFields:detail.no")}</>
    case "date":
      return <>{typeof raw === "string" ? formatDate(raw, { style: "short" }) : String(raw)}</>
    case "money": {
      if (!isRecord(raw)) return null
      const amount = Number(raw.amount)
      const currency = typeof raw.currency === "string" ? raw.currency : ""
      if (!Number.isFinite(amount) || !currency) return <>{String(raw.amount ?? "")}</>
      return <>{formatCurrency(amount, currency)}</>
    }
    case "select": {
      const option = field.options.find((o) => o.id === raw)
      return option ? (
        <>{option.label}</>
      ) : (
        <span className="text-muted-foreground">{t("customFields:detail.unknownOption")}</span>
      )
    }
    case "url": {
      const href = String(raw)
      return (
        <a
          href={href}
          target="_blank"
          rel="noreferrer noopener"
          className="break-all text-primary hover:underline"
        >
          {href}
        </a>
      )
    }
    case "item_link": {
      const id = String(raw)
      const item = linkedItems?.[id]
      const name = item?.name || t("customFields:detail.unknownItem")
      return (
        <span className="inline-flex flex-wrap items-center gap-1.5">
          {/* An item in Trash has no page to open. */}
          {groupSlug && item && !item.trashed ? (
            <Link
              to={commodityHref(groupSlug, id)}
              className="hover:underline"
              data-testid={`custom-field-link-${field.id}`}
            >
              {name}
            </Link>
          ) : (
            name
          )}
          {item?.trashed ? (
            <Badge variant="outline" className="text-[10px]">
              {t("customFields:detail.trashed")}
            </Badge>
          ) : null}
        </span>
      )
    }
    case "long_text":
      return <span className="whitespace-pre-wrap">{String(raw)}</span>
    default:
      return <>{String(raw)}</>
  }
}
