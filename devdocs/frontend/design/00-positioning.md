# Inventario — Product Positioning

The product anchor every other doc cites. **Committed.** When a taste
call is contested, this doc is the tiebreaker — the rest of the design
brief is downstream of "what is Inventario, and who is it for?".

## What it is

Inventario is a **personal inventory ledger**. A user catalogs the
things they own — appliances, electronics, tools, furniture, vehicles —
across locations and areas, attaches receipts and warranty
documentation, and gets a single source of truth for "what do I have,
where is it, what's covered, what's worth what?".

It's not:

- A point-of-sale or retail system. (No SKUs, no margin tracking.)
- A small-business asset manager. (No depreciation schedules, no IRS
  Section 179 forms — yet.)
- A consumer wishlist or "things I want". (Tracking what you don't
  own is out of scope; we track what you do.)

It might one day be: a shared household inventory (multi-user groups
already exist), a tenant-isolated business surface (multi-tenant is
already wired), an insurance-exchange surface (the export → restore →
"insurance report" path is the on-ramp).

## Audience

| Primary | Secondary | Aspirational |
| --- | --- | --- |
| The careful homeowner who *catalogs* (not just hoards). Probably has spreadsheets they're tired of. Wants warranty awareness and receipt archiving more than they want clever automation. | A household sharing one inventory across two adults. Both write, both read, both want a coherent group. | A small landlord or property manager who's outgrown a spreadsheet and a Dropbox folder. |
| Cares about: data ownership, not losing receipts, knowing what's still under warranty, seeing what got bought when. | Cares about: not stepping on each other, who-changed-what, both phones working. | Cares about: per-property scoping, exporting in a hurry for insurance, audit trails. |

The product voice is calibrated to the primary audience: thoughtful,
low-pressure, explicit. The secondary audience is the multi-tenant /
group story; the aspirational audience justifies the existence of the
backup/restore/files surface but doesn't yet drive UX choices.

## Voice traits

| Trait | What it sounds like | What it doesn't sound like |
| --- | --- | --- |
| Considered | "Add an item — we'll fill in the details later." | "Build your inventory in seconds!" |
| Honest | "This export is a snapshot. Restoring overwrites the current state." | "Magic restore — never lose data!" |
| Quiet | Status colors only when they carry meaning. Borders over shadows. | Marketing gradients, exclamation marks, full-page modals for trivia. |
| Specific | "12 items expiring in 30 days." | "Check your dashboard for updates!" |
| Forgiving | Confirm destructive actions; show a way back. | Warning banners on every page. |

A representative microcopy line, in voice: *"You don't have any items
in this area yet. When you do, they'll show up here."* See
[12-tone-of-voice-and-copy.md](12-tone-of-voice-and-copy.md) for the full microcopy contract.

## Visual anchor

Three references, in descending priority:

1. **The current shipping UI** and the internal design mock. Warm
   off-white surfaces, near-black text, amber accents, borders for
   elevation. **This is the canonical look.**
2. [Things 3](https://culturedcode.com/things/) (Cultured Code) — for
   the rhythm of dense list rows, the restraint with color, the "one
   thing on screen at a time" feel.
3. [Linear](https://linear.app/) (early) — for keyboard navigation,
   the command palette, the "tool not toy" tone of voice.

Anti-references — what Inventario deliberately is **not**:

- [Notion](https://www.notion.so/)'s "everything-is-a-block" maximalism.
- Apple's frosted-glass / hairline-shadow / heavy blur aesthetic.
- [Material 3](https://m3.material.io/)'s expressive color motion.

## Surface tone per page

| Surface | Tone target |
| --- | --- |
| Auth pages | Calm, copy-first, single column. No marketing. |
| Dashboard | Glanceable. Stat row + recent items + warranty alerts. No celebratory hero. |
| List (commodities, locations, files, tags, exports) | Dense, scannable, sortable. Filter chips quiet; bulk actions hidden until a row is selected. |
| Detail | One concept per page. Sidesheet for previews; full page when the user committed. |
| Form / dialog | Multi-step is the rule for >4 fields; single step for ≤3. |
| Settings | Card-on-page with `divide-y` rows. No toggles in flight without immediate save. |
| Empty state | Single icon + sentence + a primary CTA, never a wall of marketing copy. |
| Error / 404 / 500 | Quiet, apologetic, with a way back. No 8-bit graphics, no humor. |

## What this doc commits to

- Inventario is **boring on purpose** at the surface, so the user's
  data can be vivid. Restraint is a feature.
- Status colors carry semantic meaning, never decoration. (See
  [01-palette.md](01-palette.md), [08-interaction-states.md](08-interaction-states.md).)
- Confirmation precedes destruction. Optimism precedes server roundtrip.
  (See [15-form-and-data-ux.md](15-form-and-data-ux.md).)
- Microcopy is short, declarative, and honest. (See
  [12-tone-of-voice-and-copy.md](12-tone-of-voice-and-copy.md).)
- One audience, one tone. We don't ship a "playful mode" or a
  "professional mode" — the same voice serves both the homeowner and
  the landlord.

## What this doc doesn't decide

- Whether the product gets a marketing site (out of scope; the auth
  pages are the public surface).
- Whether mobile gets a native app (deferred; responsive web is the
  current answer).
- Whether the product gets a paid tier (deferred; pricing is a
  product-business decision, not a design one).
