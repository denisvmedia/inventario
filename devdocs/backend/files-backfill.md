# Files backfill runbook

`inventario migrate files-backfill` copies the legacy commodity-scoped
`images`, `invoices`, and `manuals` tables into the unified `files` table
introduced under epic [#1397](https://github.com/denisvmedia/inventario/issues/1397).
Once it has run, the new `GET /files` endpoint serves a complete view of
every commodity attachment, which is what the cutover ([#1421])
expects.

[#1421]: https://github.com/denisvmedia/inventario/issues/1421

## When to run

Run after the FE has switched to `/files` for reads and uploads ([#1411])
and before the cutover PR that drops the legacy tables. This is the
ordering called out in [#1399].

[#1411]: https://github.com/denisvmedia/inventario/issues/1411
[#1399]: https://github.com/denisvmedia/inventario/issues/1399

The command is safe to run multiple times: every legacy row whose `uuid`
already appears in `files` is skipped. Re-running after a partial failure,
or after the FE has been writing directly into `files` for a while, both
produce zero new rows.

## How to run

```bash
# Preview — runs the same INSERTs but rolls the transaction back at the
# end. Use this in production to confirm row counts before committing.
inventario db migrate files-backfill --dry-run --db-dsn "$DB_DSN"

# Live — commits.
inventario db migrate files-backfill --db-dsn "$DB_DSN"
```

The DSN must connect as the migrator role (the same one used for
`inventario migrate up`). RLS policies use `USING (true)` for that role,
which is what lets the cross-tenant INSERT work.

## What the output looks like

```
=== FILES BACKFILL ===
Database: postgres://inventario:***@db.internal:5432/inventario?sslmode=disable
Mode: LIVE

Source    Total  Migrated  Pending  Inserted
------    -----  --------  -------  --------
images    3142   0         3142     3142
invoices  892    0         892      892
manuals   411    0         411      411

🎉 Backfill complete — 4445 rows inserted.
```

| Column     | Meaning                                                                                       |
| ---------- | --------------------------------------------------------------------------------------------- |
| `Total`    | Rows currently in the legacy table.                                                           |
| `Migrated` | Legacy rows whose `uuid` already exists in `files` (already-backfilled or directly-uploaded). |
| `Pending`  | Legacy rows still to copy. `Total − Migrated`.                                                |
| `Inserted` | Rows the command actually wrote. Always `0` on `--dry-run`.                                   |

After a successful live run, `Pending` should be zero on every subsequent
invocation.

## Mapping

| Source table                | `files.category` | `files.type`     | `linked_entity_type` | `linked_entity_meta` |
| --------------------------- | ---------------- | ---------------- | -------------------- | -------------------- |
| `images` (commodity-scoped) | `photos`         | derived from MIME | `commodity`          | `images`             |
| `invoices`                  | `invoices`       | derived from MIME | `commodity`          | `invoices`           |
| `manuals`                   | `documents`      | derived from MIME | `commodity`          | `manuals`            |

Location-scoped uploads already write straight into `files` via the new
upload handlers, so they don't need a backfill.

The `type` column mirrors `models.FileTypeFromMIME` — kept as inline SQL
in `services/files_backfill/backfill.go` so the migration logic stays in
lock-step with the Go function. Any future change to `FileTypeFromMIME`
must be reflected there.

## Monitoring

The whole run executes inside a single transaction. For a 1 M-row legacy
dataset the command should finish in well under 10 minutes; the
`Pending → 0` transition is the success signal. If it doesn't land,
re-running picks up where the previous attempt stopped (idempotency by
`uuid`).

Watch for:

- Postgres error logs — the migrator role bypassing RLS means a stray
  `tenant_id` or `group_id` violation would surface as a NOT NULL or FK
  failure at INSERT time.
- `files` table size growth approximately equal to `images` + `invoices` +
  `manuals` row counts. Larger growth indicates the FE is still uploading
  through the legacy path; smaller growth indicates idempotency was hit
  for an unexpected reason (worth inspecting).

## Rollback

The backfill never touches the legacy tables, so rollback is just
"discard the new `files` rows":

```sql
DELETE FROM files
WHERE uuid IN (SELECT uuid FROM images)
   OR uuid IN (SELECT uuid FROM invoices)
   OR uuid IN (SELECT uuid FROM manuals);
```

Run as the migrator role (RLS bypass).

This is destructive only for rows the backfill itself wrote — the legacy
tables remain canonical until the cutover ([#1421]) drops them.
