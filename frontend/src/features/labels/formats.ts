// Label layouts for the label print page. Every size is the physical
// size on paper, in the unit the sheet is sold in, so the browser prints
// it at real size when the print dialog's scale is 100%.
//
// The two sheets are the common 3-column address-label sheets: Avery
// L7160 (and compatibles) on A4, Avery 5160 (and compatibles) on US
// Letter. "single" prints one label per page, with the page the size of
// one A4-sheet label, for label printers and for cutting out by hand.
export type LabelFormatId = "single" | "a4-3x7" | "letter-3x10"

export interface LabelFormat {
  id: LabelFormatId
  unit: "mm" | "in"
  page: { width: number; height: number }
  label: { width: number; height: number }
  columns: number
  rows: number
  // Offset of the top-left label from the top-left corner of the page.
  margin: { top: number; left: number }
  gap: { column: number; row: number }
  // Inner padding of a label. The QR code fills the label height minus
  // this padding on both sides, which also gives it its quiet zone.
  padding: number
  type: { idPt: number; namePt: number; nameLines: number; detailPt: number }
}

export const LABEL_FORMATS: Record<LabelFormatId, LabelFormat> = {
  single: {
    id: "single",
    unit: "mm",
    page: { width: 63.5, height: 38.1 },
    label: { width: 63.5, height: 38.1 },
    columns: 1,
    rows: 1,
    margin: { top: 0, left: 0 },
    gap: { column: 0, row: 0 },
    padding: 3,
    type: { idPt: 13, namePt: 8.5, nameLines: 3, detailPt: 7 },
  },
  "a4-3x7": {
    id: "a4-3x7",
    unit: "mm",
    page: { width: 210, height: 297 },
    label: { width: 63.5, height: 38.1 },
    columns: 3,
    rows: 7,
    margin: { top: 15.15, left: 7.21 },
    gap: { column: 2.54, row: 0 },
    padding: 3,
    type: { idPt: 13, namePt: 8.5, nameLines: 3, detailPt: 7 },
  },
  "letter-3x10": {
    id: "letter-3x10",
    unit: "in",
    page: { width: 8.5, height: 11 },
    label: { width: 2.625, height: 1 },
    columns: 3,
    rows: 10,
    margin: { top: 0.5, left: 0.1875 },
    gap: { column: 0.125, row: 0 },
    padding: 0.08,
    type: { idPt: 10, namePt: 7, nameLines: 2, detailPt: 6 },
  },
}

export const LABEL_FORMAT_IDS = Object.keys(LABEL_FORMATS) as LabelFormatId[]

export function isLabelFormatId(value: string | null | undefined): value is LabelFormatId {
  return !!value && value in LABEL_FORMATS
}

export function labelsPerSheet(format: LabelFormat): number {
  return format.columns * format.rows
}

// Letter is the paper size in the US and Canada; everywhere else A4.
const LETTER_REGIONS = new Set(["US", "CA"])

export function defaultSheetFormat(locale: string | undefined): LabelFormatId {
  const region = locale?.split(/[-_]/)[1]?.toUpperCase()
  return region && LETTER_REGIONS.has(region) ? "letter-3x10" : "a4-3x7"
}

// Splits the labels into pages. `skip` leaves that many positions empty
// at the start of the first page, so a partly used sheet can go through
// the printer again. A cell is null where no label is printed.
export function paginateLabels<T>(
  labels: T[],
  format: LabelFormat,
  skip = 0
): Array<Array<T | null>> {
  if (labels.length === 0) return []
  const perSheet = labelsPerSheet(format)
  const offset = Math.min(Math.max(0, Math.floor(skip)), perSheet - 1)
  const cells: Array<T | null> = [...Array<null>(offset).fill(null), ...labels]
  const sheets: Array<Array<T | null>> = []
  for (let i = 0; i < cells.length; i += perSheet) {
    sheets.push(cells.slice(i, i + perSheet))
  }
  return sheets
}

// CSS length in the format's unit.
export function len(format: LabelFormat, value: number): string {
  return `${value}${format.unit}`
}
