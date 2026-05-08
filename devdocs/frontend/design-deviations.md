# Frontend design deviations

Tracks every place where Inventario's React frontend (`frontend/`) intentionally diverges from the canonical visual contract in [`design-mocks/`](../../design-mocks/).

`design-mocks/` is a read-only mirror of `github.com/denisvmedia/inventario-design` (see [AGENTS.md](../../AGENTS.md)). Default fidelity is 1:1; this file is the trail of every decision *not* to be 1:1.

## When to add an entry

- Default is 1:1 fidelity. Any divergence is a deliberate decision.
- If the agent suggests a deviation, it must be **explicitly approved by the user** before merge.
- If the user requests a deviation, the agent must **explain the consequences** (visual drift, maintenance friction, future review cost) and confirm understanding before implementing.
- Backend/data realities that don't fit the mock count as deviations and must be logged.
- Pages or surfaces absent from `design-mocks/` should fall back to [`design-mocks/src/views/UIShowcaseView.tsx`](../../design-mocks/src/views/UIShowcaseView.tsx) and the gap should be logged here (`Why: not present in mock`).

A change that diverges without a corresponding entry here is **not finished**.

## Entry format

Append entries to the matching section. Use this template:

```markdown
### YYYY-MM-DD — Surface name

- **Issue/PR**: #NNNN / PR #NNNN
- **Mock**: <what the design mock shows — view file, key visual element>
- **Reality**: <what we ship, and why it differs visually or behaviourally>
- **Why**: <reason — backend constraint, missing data, UX decision, mock-omission, etc.>
- **Approved by**: user (explicit) | agent-suggested-then-user-confirmed
- **Reversion plan**: <how/when this might be reconciled, or "permanent">
```

Do not edit prior entries except to fix factual errors (typos, wrong issue number). When a deviation is reverted (the code is brought back in line with the mock), keep the entry but append a final line `- **Resolved**: YYYY-MM-DD, PR #NNNN — back to 1:1`.

## Sections

### Items / Commodities

_None yet._

### Locations & Areas

_None yet._

### Files & Attachments

_None yet._

### Forms & Validation

_None yet._

### Auth & Profile

_None yet._

### Settings & Preferences

_None yet._

### Navigation & App shell

_None yet._

### i18n & Formatting

_None yet._

### Tables & Lists

_None yet._

### Empty / Error / Loading states

_None yet._

### Cross-cutting (theme, density, a11y, performance)

_None yet._

### Other

_None yet._
