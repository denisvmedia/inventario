package types

// ExportStats tracks statistics during export generation
type ExportStats struct {
	LocationCount  int
	AreaCount      int
	CommodityCount int
	// ImageCount/InvoiceCount/ManualCount are the legacy commodity-scoped
	// attachment counts. Their SQL columns and model fields are preserved
	// for historical row data per #1421, but new exports leave them at 0
	// — file attachments now feed the unified FileCount instead.
	ImageCount   int
	InvoiceCount int
	ManualCount  int
	// FileCount is the number of rows from the unified `files` table that
	// were emitted in the `<files>` section of the export XML. Includes
	// commodity-, location-, area-linked and standalone files; excludes
	// the export's own backup-bundle FileEntity (linked_entity_type="export").
	FileCount      int
	BinaryDataSize int64
}
