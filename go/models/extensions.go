package models

// PostgreSQL extensions are deliberately NOT declared to Ptah.
//
// Declaring them would make Ptah manage them, and management includes removal: an
// extension present in the database that the schema does not declare is planned as
// DROP EXTENSION (classified Destructive, and DS107 at error severity in lint). A real
// database carries extensions this schema has no opinion about, and the reverse direction
// of "ensure it exists" has no safe form -- at rollback time nothing can tell whether the
// forward step created the extension or found it already there, and dropping pg_trgm would
// cascade into the trigram indexes that depend on it.
//
// Ptah does have a way to say "the description does not describe extensions"
// (ptah:not-described), but only an HCL, SQL or YAML document can carry it; a Go
// annotation source cannot, so here silence about extensions IS removal intent.
//
// The extensions are therefore created by `inventario db bootstrap` (roles and pg_trgm),
// kept out of the diff by the ignored-extension list in schema/migrations/generator, and
// asserted before migrations run by the migrator's preflight. See #2423.
//
// Historical note: these carried live //migrator:schema:extension annotations until
// PR #477 (2025-08-20) disabled them by prefixing them with an x, with no recorded reason.
// They are removed rather than left dead so nobody re-enables them without reading the
// above.
type DatabaseExtensions struct{}
