import { useTranslation } from "react-i18next"

import { formatBytes } from "@/lib/intl"

import type { StorageBreakdown } from "./api"

// StoragePie renders a donut chart of the per-group storage breakdown
// (#1388). Each filled bucket gets a fixed hex color (mapped 1:1 with
// the legend swatch); the unused remainder of the quota becomes a
// neutral "free" segment so the user sees how much room they have
// left at a glance — not just where the spent bytes went.
//
// No charting library: the donut is drawn as N <circle>s whose
// `stroke-dasharray` carries one segment each. Order is fixed so
// the legend rows always line up with the matching slice. Colors
// are hex literals — `var(--color-chart-*)` resolves the first
// stroke fine but headless Chromium (and some browsers) silently
// drop later strokes that share the same lookup, which made the
// chart render as a single tiny slice during screenshot capture.

interface PieSegment {
  key: keyof StorageBreakdown | "free"
  bytes: number
  // Hex color for the slice + matching legend swatch. We avoid the
  // shadcn `--color-chart-*` tokens here because resolving them
  // through `var()` inside an SVG `stroke` reliably renders the
  // first slice but silently drops later ones in headless Chromium —
  // hex bypasses the variable-resolution path entirely.
  color: string
  testid: string
}

const FILLED_KEYS: ReadonlyArray<{
  key: keyof StorageBreakdown
  color: string
  testid: string
}> = [
  { key: "images", color: "#e0a93e", testid: "storage-pie-images" }, // amber
  { key: "documents", color: "#5c9d6c", testid: "storage-pie-documents" }, // green
  { key: "invoices", color: "#e88f3c", testid: "storage-pie-invoices" }, // orange
  { key: "exports", color: "#5587b8", testid: "storage-pie-exports" }, // blue
  { key: "other", color: "#c66565", testid: "storage-pie-other" }, // red
]
const FREE_COLOR = "#d6d3d1" // neutral-300, visible against the card bg

interface StoragePieProps {
  breakdown: StorageBreakdown
  usedBytes: number
  quotaBytes: number | null
}

export function StoragePie({ breakdown, usedBytes, quotaBytes }: StoragePieProps) {
  const { t } = useTranslation()

  // Build the segment list. Free space only appears when a quota is
  // set; without one the donut covers the used breakdown alone (the
  // "unlimited" case).
  const filled: PieSegment[] = FILLED_KEYS.map(({ key, color, testid }) => ({
    key,
    bytes: breakdown[key],
    color,
    testid,
  }))
  const free = quotaBytes != null ? Math.max(0, quotaBytes - usedBytes) : 0
  const segments: PieSegment[] = [
    ...filled,
    ...(quotaBytes != null
      ? [
          {
            key: "free" as const,
            bytes: free,
            color: FREE_COLOR,
            testid: "storage-pie-free",
          },
        ]
      : []),
  ]
  const total = segments.reduce((acc, s) => acc + s.bytes, 0)

  // Donut geometry. r=42 gives ~16px stroke-width room inside a 100x100
  // viewBox; circumference is what stroke-dasharray drives.
  const r = 42
  const C = 2 * Math.PI * r
  const strokeWidth = 14

  // Empty state — no usage AND no quota. Render a flat ring so the
  // layout doesn't jump compared to the populated case.
  if (total <= 0) {
    return (
      <div className="flex items-center gap-4" data-testid="storage-pie" data-empty="true">
        <svg viewBox="0 0 100 100" width={128} height={128} className="shrink-0" aria-hidden="true">
          <circle
            cx={50}
            cy={50}
            r={r}
            fill="transparent"
            stroke={FREE_COLOR}
            strokeWidth={strokeWidth}
          />
        </svg>
      </div>
    )
  }

  // Compute cumulative arc offsets. Each segment is a full-circle
  // <circle> with strokeDasharray="<len> <gap>"; rotating by
  // -90° aligns the start of segment 1 with 12 o'clock.
  let cumulative = 0
  const rendered = segments
    .filter((s) => s.bytes > 0)
    .map((s) => {
      const arcLen = (s.bytes / total) * C
      const node = (
        <circle
          key={s.key}
          cx={50}
          cy={50}
          r={r}
          fill="transparent"
          stroke={s.color}
          strokeWidth={strokeWidth}
          strokeDasharray={`${arcLen} ${C - arcLen}`}
          strokeDashoffset={-cumulative}
          data-testid={s.testid}
        >
          <title>
            {`${
              s.key === "free"
                ? t("settings:storage.free")
                : t(`settings:storage.breakdown.${s.key}`)
            } — ${formatBytes(s.bytes)}`}
          </title>
        </circle>
      )
      cumulative += arcLen
      return node
    })

  return (
    <div className="flex items-start gap-4" data-testid="storage-pie">
      <svg
        viewBox="0 0 100 100"
        width={128}
        height={128}
        className="shrink-0"
        role="img"
        aria-label={t("settings:storage.title")}
      >
        <g transform="rotate(-90 50 50)">{rendered}</g>
      </svg>
      <ul className="flex-1 space-y-1 text-xs" data-testid="storage-pie-legend">
        {segments.map((s) => {
          const label =
            s.key === "free" ? t("settings:storage.free") : t(`settings:storage.breakdown.${s.key}`)
          const pct = total > 0 ? Math.round((s.bytes / total) * 100) : 0
          return (
            <li key={s.key} className="flex items-center gap-2">
              <span
                aria-hidden="true"
                className="inline-block size-2.5 shrink-0 rounded-sm"
                style={{ backgroundColor: s.color }}
              />
              <span className="flex-1 text-muted-foreground">{label}</span>
              <span className="tabular-nums font-medium">{formatBytes(s.bytes)}</span>
              <span className="tabular-nums text-muted-foreground w-9 text-right">{pct}%</span>
            </li>
          )
        })}
      </ul>
    </div>
  )
}
