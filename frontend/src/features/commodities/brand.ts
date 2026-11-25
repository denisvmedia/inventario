// brandModelText is the "brand · model" line of an item card and of the
// item page header (design-mocks ItemsPanel.tsx, ItemDetail.tsx). A part
// the item does not have is left out with its separator; an item with
// neither gives "".
export function brandModelText(item: { brand?: string; model?: string }): string {
  return [item.brand, item.model]
    .map((part) => part?.trim() ?? "")
    .filter((part) => part !== "")
    .join(" · ")
}

// brandedName puts the brand before the item's name ("Bosch Dishwasher"), as
// the insurance report titles an item (design-mocks InsuranceReportView.tsx).
// A brand the name already starts with is not repeated.
export function brandedName(item: { brand?: string; name?: string }): string {
  const brand = item.brand?.trim() ?? ""
  const name = item.name?.trim() ?? ""
  if (brand === "" || name.toLowerCase().startsWith(brand.toLowerCase())) return name
  return name === "" ? brand : `${brand} ${name}`
}
