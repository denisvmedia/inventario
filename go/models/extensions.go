package models

// PostgreSQL extensions are deliberately not declared to Ptah.
//
// Declaring one makes Ptah manage it, and management includes removal: an extension
// present in the database that the schema does not declare is planned as DROP EXTENSION.
// A real database carries extensions this schema has no opinion about, and "ensure it
// exists" has no safe reverse — at rollback nothing can tell whether the forward step
// created the extension or found it, and dropping pg_trgm cascades into the trigram
// indexes that depend on it.
//
// Declaring and also ignoring is not a way out: ignoring removes the extension from both
// sides of the diff, so it would never be created either. Ptah can express "this
// description does not describe extensions" (ptah:not-described), but only from an HCL,
// SQL or YAML document — from Go annotations, silence about extensions is removal intent.
//
// Extensions are instead created by `inventario db bootstrap`, kept out of the diff by the
// ignored-extension list in schema/migrations/generator, and checked before migrations run
// by the migrator's preflight.
type DatabaseExtensions struct{}
