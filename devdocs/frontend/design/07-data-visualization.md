# Data Visualization

Charts, sparklines, stat tiles, progress meters. **Recommendation:**
lean on shadcn's chart wrapper (`src/components/ui/chart.tsx` if
copied in) over Recharts; reach for the `--chart-1` … `--chart-5`
tokens. Keep charts quiet — they support a stat, they're not a stat.

## When to chart

| Surface | Default visualization |
| --- | --- |
| Dashboard | Stat tiles + recent items list. **No** large charts. |
| Reports / future | Bar / line / sparkline / donut, in that order of preference. |
| Item detail | Status history as a timeline list, **not** a chart. |
| Tag detail (future) | Stat tiles + maybe sparkline of usage over time. |
| Insurance report | Tabular density. Charts only when they replace 5+ rows of text. |

A chart that needs an axis label and a legend usually wants to be a
table instead. Inventario users are reading "what do I have?"; a
table answers it directly.

## Chart palette

The five chart-token slots (`--chart-1` through `--chart-5`) are the
**only** allowed chart colors. Defined in `frontend/src/index.css`:

| Token | Light | Use |
| --- | --- | --- |
| `--chart-1` | `oklch(0.7 0.16 75)` | Amber — primary series |
| `--chart-2` | `oklch(0.65 0.14 145)` | Green — secondary |
| `--chart-3` | `oklch(0.6 0.14 220)` | Blue — tertiary |
| `--chart-4` | `oklch(0.75 0.18 55)` | Warm yellow — quaternary |
| `--chart-5` | `oklch(0.62 0.18 25)` | Red — quinary / "other / decline" |

Order is meaningful: the most important series gets `--chart-1`;
"decline" / "negative" connotation goes to `--chart-5`. Don't pick a
chart slot to convey domain status — use the `--status-*` tokens
([08-interaction-states.md](08-interaction-states.md), [01-palette.md](01-palette.md)).

## Chart-type-per-data-shape

A short heuristic, in descending order of preference:

| Data shape | Default | Why |
| --- | --- | --- |
| One number, in context | Stat tile | The biggest insight from the smallest pixel cost. |
| One number over time | Sparkline | Compact, no axes, fits a stat tile. |
| Few categories at one moment | Horizontal bars | Easier than donuts to read; ranks naturally. |
| Many categories at one moment | Vertical bars | Higher density. |
| One series over time, with magnitude | Line chart | The default time-series shape. |
| Two-series comparison over time | Line, two series | Bar grouping invites mistakes; lines are clearer. |
| Composition of a whole | Stacked horizontal bar | A donut hides the smallest slices. **Avoid pie / donut.** |
| Distribution | Histogram | Bucketed bars, not a smoothed density curve. |

## Axes and labels

- Y-axis labels at sm size (`text-xs text-muted-foreground`),
  tick-aligned.
- X-axis labels at sm size, rotated only if absolutely necessary
  (rotation is a smell — rephrase the labels first).
- No grid lines. Use `stroke-muted/30` for a single horizontal axis
  line if a baseline is needed.
- No 3D effects. No drop shadows on bars. No gradient fills.

## Sparklines

A sparkline goes inside a stat tile. Inline, no axes, single color,
the same height as the value text:

```tsx
<div className="flex items-end gap-3">
  <div>
    <p className="text-xs text-muted-foreground">{label}</p>
    <p className="text-2xl font-bold tracking-tight">{value}</p>
  </div>
  <Sparkline data={recent} className="h-8 w-24 text-chart-1" />
</div>
```

Sparkline color = `--chart-1` unless the metric has a domain
connotation that maps to a status (`--status-*`).

## Tooltips on charts

Always interactive — the user hovers a bar / point and sees:

```
┌──────────────────┐
│ April 18, 2026   │
│ Items added: 12  │
└──────────────────┘
```

- Use the same Radix `<Tooltip>` primitive as elsewhere.
- Format dates / numbers via `src/lib/intl.ts`'s helpers (see
  [13-formatting-and-i18n.md](13-formatting-and-i18n.md)).
- Never put a tooltip on every visual — the whole-chart hover layer
  handles it.

## Empty / loading / error

Charts have the same three states every other data surface has (see
[08-interaction-states.md](08-interaction-states.md)):

- **Empty**: a single line of muted copy ("No data yet") +
  the same chart frame at minimum size. Don't render an empty axis-only
  skeleton.
- **Loading**: skeleton-shaped placeholder bar / line at `bg-muted`
  with `animate-pulse`.
- **Error**: an inline note, not a toast. ("Couldn't load. Retry.")

## Ranges and aggregation

When the chart is over time, default range = "last 30 days". User
can pick from `7d / 30d / 90d / 12m / All` with a segmented control
(`<Tabs>` works fine). Persist the choice per-page in `searchParams`
(not localStorage) so a deep link encodes the view.

## Hard rules

1. **Five colors max.** `--chart-1` … `--chart-5`. If you need a
   sixth, you have too many series — re-bucket.
2. **No domain status as a chart color.** Status uses `--status-*`.
3. **No pies / donuts.** A stacked horizontal bar is always clearer.
4. **No gradients in chart fills.** Solid token colors only.
5. **Tooltips and labels via `src/lib/intl.ts`.** No `Number(value).toFixed(2)`
   sprayed at render time.

## Anti-patterns

- A 6-series line chart ("the whole inventory broken down by tag").
  Filter to top 5 + "Other".
- A pie chart with 3 slices. Use 3 stat tiles.
- A "comparison" chart that overlays bars on lines. The eye can't
  parse it; pick one shape.
- Chart titles in `text-2xl`. Charts are dependents — `text-base
  font-semibold` is the cap.
- A chart full of `text-amber-500` because "the bars should be amber".
  Tokens (`text-chart-1`).

## Cross-refs

- Tokens: [01-palette.md](01-palette.md).
- Stat tiles: [03-space-and-layout.md](03-space-and-layout.md) ("Stat card row").
- Formatting: [13-formatting-and-i18n.md](13-formatting-and-i18n.md).
- States (empty / loading / error): [08-interaction-states.md](08-interaction-states.md).
- Recharts via shadcn chart wrapper: <https://ui.shadcn.com/charts>.
