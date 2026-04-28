# Data Visualization

The current dashboard (`02-home`) is four stat cards and two empty containers. That's not a dashboard, that's a placeholder. This document defines what should be there and how it looks.

## What the dashboard should answer

User-facing questions, in priority order:

1. **What's in my home / collection right now?** Total inventory count, value (if known), recent additions. *Identity at a glance.*
2. **What needs attention?** Warranties expiring soon, items missing prices for insurance, untagged items, low-document items. *Action triggers.*
3. **What did I do recently?** Activity feed: last 10 events (item added, file uploaded, status changed). *Confidence and memory.*
4. **What's the shape of my collection?** Category distribution, value-by-area, age distribution. *Reflective patterns.*

Not in scope for the personal dashboard:
- Performance metrics (uptime, memory) — those belong on `/system` admin page (where they already live)
- Cohort analytics, conversion funnels — irrelevant for a personal tool
- Real-time updating numbers — overkill, refresh on visit is fine

## Dashboard layout

12-col grid, with widgets sized as follows on desktop:

```
┌─────────────────────────────────────────────────┐
│ Hero: "Welcome back. 93 things, worth 167K."    │  (col-span-12, height: auto)
└─────────────────────────────────────────────────┘
┌──────────────────┐ ┌──────────────────┐ ┌──────┐
│ Needs attention  │ │ Recent activity  │ │ Stats│  (4 + 5 + 3 cols, height: 320px)
│ (callout list)   │ │ (timeline)       │ │ stack│
└──────────────────┘ └──────────────────┘ └──────┘
┌────────────────────────┐ ┌──────────────────────┐
│ Value over time        │ │ Distribution by area │  (7 + 5 cols, height: 280px)
│ (area chart)           │ │ (donut/treemap)      │
└────────────────────────┘ └──────────────────────┘
┌─────────────────────────────────────────────────┐
│ Recently added (gallery — last 6 items)         │  (col-span-12, height: auto)
└─────────────────────────────────────────────────┘
```

Mobile: every widget collapses to full-width, stacked. Recent activity becomes the second widget (highest engagement on mobile).

## Hero widget

Not "Welcome to Inventario". A live, personalized line:

> **Welcome back, Denis.**
> 93 things across 4 locations. Roughly 167,000 CZK on the books.

Below it, three thin secondary lines:
- *3 warranties expire this month.*
- *5 items still missing a price — your insurance form will thank you.*
- *Last updated yesterday.*

Tone: respectful, present-tense, gently nudging where useful. No exclamation marks.

## Chart palette

Charts inherit from the chosen color palette but use **specific data-viz tokens** that maintain visual hierarchy across many series.

Direction A example (5 categorical colors + 1 emphasis):
```css
--chart-1: var(--accent);            /* terracotta */
--chart-2: #5B7C99;                  /* desaturated blue, complementary */
--chart-3: #7A8F60;                  /* muted sage */
--chart-4: #B59169;                  /* warm tan */
--chart-5: #8E6B85;                  /* dusty rose */
--chart-emphasis: var(--ink-primary);
--chart-grid:    var(--border-subtle);
--chart-axis:    var(--ink-muted);
--chart-tooltip: var(--surface-overlay);
```

Direction B and C get analogous 5-step palettes derived from their accents.

**Order of use:** for series 1–5 in alphabetical/categorical order, use chart-1 → chart-5. For continuous data (single series), use accent only with gradient fill.

Avoid: rainbow palettes, generic d3 schemes (`d3.schemeCategory10`), traffic-light reds/greens unless semantic.

## Chart types per data shape

| Data shape | Chart |
| --- | --- |
| Single value over time | **Area chart** with gradient fill, hover tooltip with value+date |
| Comparison across categories | **Horizontal bar** (vertical bars only for time series), value labels on bars when ≤7 categories |
| Part-to-whole, ≤6 parts | **Donut** with center summary label |
| Part-to-whole, 7+ parts | **Treemap** or **stacked bar** — never a many-segment pie |
| Distribution / variance | **Box plot** or **histogram**, depending on count |
| Geographic | **Choropleth or pin map** — out of scope for v1, but reserved |
| Sparkline (in stat cards) | Tiny line chart, no axes, hover only |

## Chart component requirements

Every chart in Inventario must:

- Have a **clear title** above the chart, body-sm weight medium
- Have a **subtitle** with date range / scope, body-xs weight regular `--ink-muted`
- Have a **clean axis** — only print labels at meaningful intervals, no jagged tick density
- Use **tabular-nums** for all axis labels and tooltip values
- Have a **hover state** with a tooltip showing exact value (currency-formatted)
- Have an **empty state** ("Not enough data yet — keep adding items and this chart will fill in.")
- Have a **loading state** as a skeleton with chart-shaped pulse
- Be **responsive** — recompute layout on resize, no fixed pixel widths
- Be **keyboard navigable** — tab through data points
- Have a **screen reader description** summarizing the trend ("Inventory value rose from 45,000 to 167,000 over the past 12 months")

## Recommended library

**Apache ECharts** (`vue-echarts`) for major charts. Reasons:
- Best-in-class accessibility and keyboard support
- Highly themeable via tokens
- Performance scales to 10K+ data points
- Solid SSR/hydration story (relevant if Inventario adds SSR)

For sparklines (in stat cards): hand-rolled SVG inside a small Vue component. ~30 lines, no library overhead.

Alternative: **Recharts** is React-only. **ApexCharts** is fine but heavier and fights the design tokens. **Chart.js** is lightweight but accessibility is weaker.

## Sparkline style

Compact line chart embedded in stat cards.

```
Total value
167,432 CZK    ▁▂▂▃▅▇█
↑ 12% this month
```

- 60×24px (or 80×32 in compact mode)
- Stroke `--accent`, 1.5px
- No fill (line-only)
- No axis labels
- Hover: tooltip with exact values per point
- For binary/discrete sparklines, switch to small dot-bar chart

## Number formatting in charts

- **Money:** abbreviate above 10K (`12.5K`, `1.2M`) on axis labels; show full on tooltip (`12,532.00 CZK`)
- **Counts:** raw integer always
- **Percentages:** one decimal place (`12.4%`)
- **Dates:** locale-respecting; relative on tooltips when within 7 days (`3 days ago`)

## Trend indicators

Up/down arrows next to delta values. Use semantic colors but **muted** versions to avoid alarm-fatigue:

```css
.trend-up   { color: var(--success); }
.trend-down { color: var(--ink-secondary); }   /* not red — "down" isn't always bad */
.trend-flat { color: var(--ink-muted); }
```

Use red (`--destructive`) **only** when down is genuinely concerning — e.g., "expired warranties" count going up means red on that specific metric, not on inventory count.

## Activity feed widget

Vertical list, max 10 items, "View all" link to a full activity log page (out of v1 scope; placeholder link is OK).

Each entry:
```
[icon]  You added "Camping Equipment" to Home → Bedroom.
        2 hours ago
```

- Icon: Phosphor matching action type (Plus, Pencil, Trash, Upload, Download, Tag)
- Action sentence: present-tense action verb, item name in `font-medium`, location/path in `text-muted`
- Time: relative ("2 hours ago", "Tuesday at 3pm", "Apr 12") via a single time-formatter

## Distribution widget

Donut for ≤6 categories; treemap (or vertical-stacked bar with labels) for 7+. Click a slice to filter the inventory list to that category.

Donut center label:
```
93
items
```

In `heading-xl` weight medium, `body-xs uppercase tracking-uppercase` for "items".

## "Recently added" gallery

Horizontal scroller of last 6 items as cards. Each card:
- Image thumbnail (or file-type icon if no image)
- Item name (`body` weight medium)
- Location ("Home → Bedroom") (`body-sm` muted)
- Time added ("2 days ago")

Click → goes to commodity detail.

## Backend implications

The current backend likely doesn't expose the aggregated endpoints needed. Sprint 1 includes adding:

- `GET /api/v1/dashboard/summary` → hero stats (count, value, location count)
- `GET /api/v1/dashboard/attention` → list of items needing action (warranty expiring, missing prices, etc.)
- `GET /api/v1/activity` → paginated activity feed
- `GET /api/v1/dashboard/value-history` → time-series for area chart
- `GET /api/v1/dashboard/distribution` → category breakdown

Pre-aggregate via materialized views or dashboard-specific table. Don't compute on every dashboard hit if data grows.

## Decision needed

None on this document — it's all my call. Just confirm the scope when you read it.
