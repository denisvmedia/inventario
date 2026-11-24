// Links into and out of the label print page.

export type LabelTargetKind = "commodities" | "areas"

// The URL a label's QR code encodes: the absolute address of the item or
// area page. Opening it while signed out goes through the login page and
// comes back here (ProtectedRoute keeps the path in `?redirect=`).
export function labelTargetUrl(
  origin: string,
  groupSlug: string,
  kind: LabelTargetKind,
  id: string
): string {
  return `${origin}/g/${encodeURIComponent(groupSlug)}/${kind}/${encodeURIComponent(id)}`
}

export interface LabelSelection {
  items?: string[]
  areas?: string[]
  // Also print a label for every item in this area.
  areaItems?: string
}

export function labelsPageHref(groupSlug: string, selection: LabelSelection): string {
  const params = new URLSearchParams()
  for (const id of selection.areas ?? []) params.append("area", id)
  if (selection.areaItems) params.set("area_items", selection.areaItems)
  for (const id of selection.items ?? []) params.append("item", id)
  const qs = params.toString()
  return `/g/${encodeURIComponent(groupSlug)}/labels${qs ? `?${qs}` : ""}`
}
