// Draft persistence + payload helpers for the commodity create/edit form.
//
// Extracted from CommodityFormDialog.tsx (#1988) so the anonymous
// "add your first item before login" flow can drive the SAME draft
// machinery the dialog uses: the landing wrapper stashes validated
// values via writeDraft, and the post-login FirstItemResolver replays
// them through readDraft → toRequest → createCommodity. Keeping these
// in a leaf module (no React, no GroupProvider dependency) lets the
// resolver import them without dragging the whole dialog in.
//
// Behaviour is identical to the in-dialog originals — this is a pure
// move. The dialog re-imports every symbol below.
import { uploadFile, updateFile } from "@/features/files/api"
import { categoryFromMime } from "@/features/files/constants"
import type { CommodityStatusValue, CommodityTypeValue } from "@/features/commodities/constants"
import type { CommodityFormInput } from "@/features/commodities/schemas"
import type {
  Commodity,
  CreateCommodityRequest,
  UpdateCommodityRequest,
} from "@/features/commodities/api"

// PendingFile mirrors the Files-step model in CommodityFormDialog. Kept
// here (rather than imported from the dialog) so the upload helper has
// no dependency back on the React component — the resolver replays
// pending files loaded from IndexedDB (`loadPendingFiles`) which return
// this same shape.
export interface PendingFile {
  id: string
  file: File
  tags: string[]
}

// ---- Draft persistence helpers ------------------------------------------

// readDraft pulls the previously-saved form values for `key` (per
// #1383). Returns undefined when nothing is stored or the JSON has
// rotted; callers fall back to defaults in either case.
export function readDraft(key: string): Partial<CommodityFormInput> | undefined {
  if (typeof window === "undefined") return undefined
  try {
    const raw = window.localStorage.getItem(key)
    if (!raw) return undefined
    const parsed = JSON.parse(raw) as Partial<CommodityFormInput>
    return parsed
  } catch {
    return undefined
  }
}

export function writeDraft(key: string, values: Partial<CommodityFormInput>): void {
  if (typeof window === "undefined") return
  try {
    window.localStorage.setItem(key, JSON.stringify(values))
  } catch {
    // Quota / private mode / disabled storage — drop silently. Drafts
    // are an enhancement, not a guarantee.
  }
}

export function clearDraft(key: string): void {
  if (typeof window === "undefined") return
  try {
    window.localStorage.removeItem(key)
  } catch {
    // see writeDraft
  }
}

// buildDefaults populates the form with safe initial values. For edit
// mode it carries the existing record; for create mode the only
// pre-populated bits are the count (1), status (in_use), draft, and the
// group currency. Everything else stays empty so the user fills it in.
//
// `defaultDraft` seeds the `draft` toggle for a brand-new item (it has
// no effect in edit mode, where the record's own `draft` wins). The
// anonymous landing flow (#1988) passes `true` so a first-time visitor
// only has to enter name/short_name/type — price/date become optional —
// keeping the "add your first item" path as short as possible.
export function buildDefaults(
  initial: Commodity | undefined,
  currency: string,
  defaultDraft = false
): CommodityFormInput {
  // `urls` is typed as `string` by openapi-typescript because the BE
  // model uses `swaggertype:"string"` on a JSONB column; at runtime it's
  // an array of strings. Coerce safely.
  const urls = Array.isArray(initial?.urls) ? (initial.urls as unknown as string[]) : []
  // Numeric defaults are kept as strings here (and throughout the
  // form) so the schema's input type stays string — see schemas.ts
  // for the rationale. They convert to JS numbers at submit time
  // inside toRequest.
  const numStr = (n: number | undefined): string => (n === undefined ? "" : String(n))
  return {
    name: initial?.name ?? "",
    short_name: initial?.short_name ?? "",
    type: (initial?.type as string) ?? "",
    area_id: initial?.area_id ?? "",
    status: (initial?.status as string) ?? "in_use",
    count: initial?.count !== undefined ? String(initial.count) : "1",
    original_price: numStr(initial?.original_price),
    original_price_currency: (initial?.original_price_currency as string) ?? currency,
    converted_original_price: numStr(initial?.converted_original_price),
    current_price: numStr(initial?.current_price),
    serial_number: initial?.serial_number ?? "",
    extra_serial_numbers: initial?.extra_serial_numbers ?? [],
    part_numbers: initial?.part_numbers ?? [],
    tags: initial?.tags ?? [],
    purchase_date: (initial?.purchase_date as string) ?? "",
    urls,
    comments: initial?.comments ?? "",
    draft: initial?.draft ?? defaultDraft,
    warranty_expires_at: (initial?.warranty_expires_at as string) ?? "",
    warranty_notes: initial?.warranty_notes ?? "",
  }
}

// toRequest maps the validated form input into the BE-shaped envelope's
// attributes. Numbers come out of the form as strings (see schemas.ts);
// we convert here. `urls` flows through as string[] even though
// openapi-typescript types it as a single string (see buildDefaults).
export function toRequest(
  values: CommodityFormInput,
  groupCurrency: string
): CreateCommodityRequest & UpdateCommodityRequest {
  const num = (v: string): number | undefined => (v === "" ? undefined : Number(v))
  // Date fields are PDate (pointer-to-Date) on the BE — `Date.UnmarshalJSON`
  // rejects empty strings as "cannot parse \"\" as \"2006\"". Omit the
  // field entirely when the input is blank so the BE sees a missing
  // value (decoded as nil pointer) rather than an invalid date string.
  const date = (v: string): string | undefined => {
    const trimmed = v.trim()
    return trimmed === "" ? undefined : trimmed
  }
  // BE rule (PriceRule.ErrConvertedPriceNotZero in
  // go/models/rules/price.go): when the purchase currency matches the
  // group's currency, `converted_original_price` MUST be 0 — the
  // original price is already expressed in group currency, so a
  // non-zero converted amount would conflict. The mock hides the
  // converted-price field entirely in this case (AddItemDialog L1198
  // isForeignCurrency = false branch); we mirror that visually, and
  // force the value to 0 here so the BE's same-currency invariant is
  // satisfied.
  const original = num(values.original_price)
  const convertedFromForm = num(values.converted_original_price)
  const currentFromForm = num(values.current_price)
  const sameCurrency =
    !!groupCurrency &&
    values.original_price_currency.trim().toUpperCase() === groupCurrency.trim().toUpperCase()
  // Foreign-currency: ConvertedOriginalPrice still carries
  // `validation.Required` at the BE for non-draft commodities, so an
  // omitted value JSON-decodes to the zero struct and is rejected.
  // The schema's cross-field rule lets the user satisfy "at-least-one"
  // by filling only the current-value field; mirror that into
  // converted so the BE invariant survives. Explicit 0s are preserved
  // (`?? `, not `||`) so an edit-mode row that genuinely has
  // converted=0 / current>0 round-trips unchanged.
  // CurrentPrice carries no field-level Required anymore (#1625), so
  // we never need to mirror in the other direction — passing through
  // whatever the user typed is enough.
  let converted: number | undefined
  if (sameCurrency) {
    converted = 0
  } else {
    converted = convertedFromForm ?? currentFromForm
  }
  const current = currentFromForm
  return {
    name: values.name.trim(),
    short_name: values.short_name.trim(),
    type: values.type as CommodityTypeValue,
    // Omit when empty so the BE creates/leaves the item unassigned
    // (#1986) rather than rejecting "" as a missing area FK. An explicit
    // clear in edit mode (area_id = "") therefore un-assigns the item.
    area_id: values.area_id || undefined,
    status: values.status as CommodityStatusValue,
    count: Number(values.count),
    original_price: original,
    original_price_currency: values.original_price_currency,
    converted_original_price: converted,
    current_price: current,
    serial_number: values.serial_number.trim(),
    extra_serial_numbers: values.extra_serial_numbers,
    part_numbers: values.part_numbers,
    tags: values.tags,
    purchase_date: date(values.purchase_date),
    // Drop blank rows the user added but never filled — sending `[""]`
    // would trip the BE's per-URL Host/Scheme validation.
    urls: values.urls.map((u) => u.trim()).filter((u) => u !== "") as unknown as string,
    comments: values.comments,
    draft: values.draft,
    warranty_expires_at: date(values.warranty_expires_at),
    warranty_notes: values.warranty_notes,
  }
}

// uploadPendingFiles uploads each staged file then links it to the
// freshly-created commodity (two-step BE flow: POST /uploads/file then
// PUT /files/:id with the entity link + category + tags). Per-file
// failures are reported via `onError`; the commodity itself is already
// persisted, so a failed attach is non-fatal (the user can retry from
// the detail page).
export async function uploadPendingFiles(
  pending: PendingFile[],
  commodityId: string,
  onError: (entry: PendingFile, err: unknown) => void
): Promise<void> {
  const work = pending.map(async (entry) => {
    try {
      const result = await uploadFile(entry.file)
      const category = categoryFromMime(entry.file.type)
      await updateFile(result.file.id, {
        linked_entity_type: "commodity",
        linked_entity_id: commodityId,
        category,
        tags: entry.tags.length > 0 ? entry.tags : undefined,
      })
    } catch (err) {
      onError(entry, err)
    }
  })
  await Promise.all(work)
}
