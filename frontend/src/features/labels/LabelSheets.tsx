import type { CSSProperties } from "react"
import { QRCodeSVG } from "qrcode.react"

import { cn } from "@/lib/utils"

import { len, type LabelFormat } from "./formats"

export interface LabelData {
  key: string
  // The address the QR code encodes.
  url: string
  // Accessible name of the QR code.
  qrTitle: string
  // Asset ID of an item. Areas have none, and their name takes its place.
  code?: string
  title: string
  // One small line under the title, such as the location of an area.
  detail?: string
}

interface LabelSheetsProps {
  sheets: Array<Array<LabelData | null>>
  format: LabelFormat
}

// Physical sizes reach the Tailwind utilities as CSS variables: they come
// from the chosen format at run time, so no static class can carry them.
function formatVariables(format: LabelFormat): CSSProperties {
  const labelHeight = len(format, format.label.height)
  const padding = len(format, format.padding)
  return {
    "--sheet-w": len(format, format.page.width),
    // A sheet exactly as tall as the page can spill a blank page after
    // it through rounding, so it stops half a millimeter short. Nothing
    // is printed in the bottom margin.
    "--sheet-h": `calc(${len(format, format.page.height)} - 0.5mm)`,
    "--sheet-top": len(format, format.margin.top),
    "--sheet-left": len(format, format.margin.left),
    "--sheet-cols": `repeat(${format.columns}, ${len(format, format.label.width)})`,
    "--label-w": len(format, format.label.width),
    "--label-h": labelHeight,
    "--label-gap-x": len(format, format.gap.column),
    "--label-gap-y": len(format, format.gap.row),
    "--label-pad": padding,
    "--label-qr": `calc(${labelHeight} - 2 * ${padding})`,
    "--label-id-size": `${format.type.idPt}pt`,
    "--label-name-size": `${format.type.namePt}pt`,
    "--label-detail-size": `${format.type.detailPt}pt`,
    "--label-name-lines": String(format.type.nameLines),
  } as CSSProperties
}

// LabelSheets lays labels out on pages at their real size. The same
// markup is the on-screen preview and the printed copy: the page's
// screen-only rules outline every label position (.label-slot) so the
// layout can be checked against the sheet, and the outlines and shadow
// stay off paper.
export function LabelSheets({ sheets, format }: LabelSheetsProps) {
  return (
    <div className="label-sheets contents" style={formatVariables(format)}>
      {sheets.map((cells, sheetIndex) => (
        <div
          key={sheetIndex}
          className="label-sheet label-paper grid h-(--sheet-h) w-(--sheet-w) shrink-0 grid-cols-(--sheet-cols) auto-rows-(--label-h) content-start gap-x-(--label-gap-x) gap-y-(--label-gap-y) overflow-hidden pt-(--sheet-top) pl-(--sheet-left) shadow-sm print:shadow-none"
          data-testid="label-sheet"
        >
          {cells.map((label, cellIndex) =>
            label ? (
              <Label key={label.key} label={label} />
            ) : (
              <div key={`empty-${cellIndex}`} className="label-slot-empty" aria-hidden="true" />
            )
          )}
        </div>
      ))}
    </div>
  )
}

function Label({ label }: { label: LabelData }) {
  return (
    <div
      className="label-slot flex h-(--label-h) w-(--label-w) items-center gap-(--label-pad) overflow-hidden p-(--label-pad)"
      data-testid="label"
      data-label-url={label.url}
    >
      <QRCodeSVG
        value={label.url}
        level="M"
        marginSize={0}
        title={label.qrTitle}
        className="size-(--label-qr) shrink-0"
      />
      <div className="min-w-0 flex-1">
        {label.code ? (
          <p
            className="font-mono text-(length:--label-id-size) leading-tight font-bold"
            data-testid="label-code"
          >
            {label.code}
          </p>
        ) : null}
        <p
          className={cn(
            "line-clamp-(--label-name-lines) text-(length:--label-name-size) leading-snug break-words",
            !label.code && "font-semibold"
          )}
        >
          {label.title}
        </p>
        {label.detail ? (
          <p className="truncate text-(length:--label-detail-size) leading-snug">{label.detail}</p>
        ) : null}
      </div>
    </div>
  )
}
