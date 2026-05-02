# Screenshots — PR #1486 (#1400 BE tags first-class)

35 screenshots captured locally from the running React frontend (binary
built with `with_frontend`, memory:// DB, seed data) on 2026-05-02.

This branch is **viewing-only**, not for merge. Hand-rolled per
maintainer request, against the `screenshot-review` skill's default
"local-only" recommendation.

## How they were taken

```
make build-frontend
cd go/cmd/inventario && go build -tags with_frontend -o ../../../bin/inventario .
./bin/inventario run --db-dsn=memory:// --no-auth-rate-limit --no-global-rate-limit &
curl -X POST http://localhost:3333/api/v1/seed
BASE_URL=http://localhost:3333 OUT=/tmp/screenshots-1486 node e2e/screenshots.mjs
```

`40-tags-page.png` and `41-add-dialog-extras.png` came from a one-off
script (since deleted) that drove the sidebar's `/tags` link and tried
to reach the Add Item dialog's Extras step.

## TL;DR — what these screenshots tell us about #1400

#1400 is **backend-only**. There is no new visible FE surface in this
PR. The expected user-visible delta is:

- Tag inputs on commodity/file create+update get normalized server-side
  (e.g. typing `Kitchen` produces a row tagged `kitchen`).
- New `tags` rows are auto-created when a commodity/file write
  references an unknown slug.

Both deltas are invisible until the FE Tags page (#1412) ships. So
these screenshots are a regression check, not a feature showcase.

## From this PR — verdict: no regressions visible

| Surface | Screenshot | Verdict |
| --- | --- | --- |
| Commodity list — tag chips on cards | `15-commodities-list.png` | ✅ Tags render correctly (`outdoor`, `seasonal`, `electronics`, `presentation`, `furniture`, `work`, `clothes`, `entertainment`). All seed slugs are already lowercase so my normalizer wouldn't change them — confirmed nothing got mangled. |
| Commodity preview sheet — TAGS section | `16-commodities-preview-sheet.png` | ✅ Two chips render correctly. |
| Commodity detail — TAGS section | `18-commodity-detail.png` | ✅ Chips render in same shape as before. |
| Commodity print — TAGS line (comma-joined text, not chips) | `19-commodity-print.png` | ✅ Pre-existing comma-joined format preserved. |
| File upload step 2 — metadata | `32-files-upload-step2-metadata.png` | ✅ Dialog now-and-before says "Tags can be added later" — no tag field on this step (existing behavior). |
| File edit form — Tags input | `36-file-edit.png` | ✅ Single comma-separated tag input renders. Auto-create kicks in on save (handler change). |

## From this PR — confirmed wiring

`40-tags-page.png` shows `/g/{slug}/tags` is a placeholder reading
**"Coming soon. Tracked by #1412."** That confirms the existing FE has
been waiting for this BE work; the route + sidebar entry already exist.

`41-add-dialog-extras.png` shows the multi-step Add Item dialog with
`Basics → Purchase → Warranty → Extras → Files`. Tags input lives on
**Extras** (not screenshotted here because the script couldn't bypass
step-1 validation). Auto-create runs in the create handler regardless
of which step the user typed the tag on, so this is a wiring concern
rather than a visual one.

## Pre-existing observations (NOT from this PR — surfaced for context)

- `40-tags-page.png` placeholder is intentional waiting room for #1412;
  this PR doesn't change it.
- File upload step-2 dropdown styled differently from the rest of the
  form (native `<select>` arrow on `Category`) — pre-existing in the
  Files page (#1411). Not related to #1400.

## Surfaces verified visually correct

`01-login` `02-register` `03-forgot-password` `04-not-found`
`10-dashboard` `11-locations` `12-locations-new` `13-location-detail`
`13qa..d` (location quick-attach) `14-area-detail` `15-commodities-list`
`16-commodities-preview-sheet` `17-commodities-add-dialog-step1`
`18-commodity-detail` `18b-commodity-detail-files` `18qa..d` (commodity
quick-attach) `19-commodity-print` `20-profile` `21-settings`
`30-files-list` `31..34-files-upload-flow` `35-file-detail-sheet`
`36-file-edit` `37-files-filtered-invoices` `40-tags-page`
`41-add-dialog-extras`.

## What this set does NOT cover

- Postgres-backed flow (this run was `memory://`).
- The auto-create round-trip end-to-end: typing a new tag in the Add
  Item Extras step, submitting, and seeing the resulting commodity
  detail show the normalized chip + the new tag landing in `/tags`
  (placeholder, so not visible) + autocomplete suggesting it. Drilling
  to Extras step required filling required step-1 fields with valid
  data; the script attempted to advance via Next without filling all
  required selects, so the validation blocker stopped traversal.
- Dark-mode variants. Light theme only.
- Mobile breakpoint (1440×900 viewport).

If any of those gaps are interesting to verify before merge, say
which and I'll script them.
