// reportSubtitle is the line under an item's title in the insurance report:
// the model, as in the mock (design-mocks InsuranceReportView.tsx), or the
// short name for an item without a model when it differs from the name.
export function reportSubtitle(item: {
  model?: string
  short_name?: string
  name?: string
}): string {
  const model = item.model?.trim() ?? ""
  if (model !== "") return model
  const short = item.short_name?.trim() ?? ""
  return short !== "" && short !== item.name ? short : ""
}
