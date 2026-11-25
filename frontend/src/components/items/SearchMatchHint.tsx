import { useTranslation } from "react-i18next"
import type { TFunction } from "i18next"

import type { CommoditySearchMatch } from "@/features/commodities/query"
import { cn } from "@/lib/utils"

// The text of a search match: "Serial number: SN-1234". A tag match carries
// the slug, shown through tagLabel as the tag's label.
export function searchMatchText(
  t: TFunction,
  match: CommoditySearchMatch,
  tagLabel: (slug: string) => string
): string {
  const raw = match.value ?? ""
  const value = match.field === "tags" ? tagLabel(raw) : raw
  return t("commodities:searchMatch.text", {
    field: t(`commodities:searchMatch.field.${match.field ?? "name"}`),
    value,
  })
}

interface SearchMatchHintProps {
  match: CommoditySearchMatch | undefined
  tagLabel: (slug: string) => string
  className?: string
  "data-testid"?: string
}

// One muted line under a search result saying where the search text was
// found, for a match the card does not already show (see
// primarySearchMatch). Renders nothing without a match.
export function SearchMatchHint({ match, tagLabel, className, ...rest }: SearchMatchHintProps) {
  const { t } = useTranslation()
  if (!match?.field) return null
  const text = searchMatchText(t, match, tagLabel)
  return (
    <p
      className={cn("truncate text-[11px] text-muted-foreground", className)}
      title={text}
      data-testid={rest["data-testid"] ?? "commodity-search-match"}
      data-match-field={match.field}
    >
      {text}
    </p>
  )
}
