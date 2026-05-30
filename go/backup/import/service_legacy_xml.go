//go:build legacy_xml_backup

// DEPRECATED — LEGACY XML BACKUP CODE.
// Compiled ONLY under the `legacy_xml_backup` build tag; NOT in the default build.
// Implements the obsolete XML backup format that #534 replaced with the signed
// JSON `.inb` archive. Retained solely to be extracted into a separate repo as an
// XML-streaming proof-of-concept. Do not extend; do not couple new code to it.

package importpkg

import (
	"context"
	"io"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/backup/export/parser"
	"github.com/denisvmedia/inventario/models"
)

// importFileMeta returns the FileEntity stamping for a legacy imported XML
// backup: `.xml` / application/xml / "xml-1.0".
func importFileMeta() importFileMetaFields {
	return importFileMetaFields{
		Ext:              ".xml",
		MIMEType:         "application/xml",
		LinkedEntityMeta: "xml-1.0",
		Tags:             models.StringSlice{"export", "xml", "imported"},
	}
}

// parseImportMetadata parses an uploaded XML backup, extracting the statistics
// used to stamp the import's export record.
func (s *ImportService) parseImportMetadata(ctx context.Context, reader io.Reader) (importStats, error) {
	stats, _, err := parser.ParseXMLMetadata(ctx, reader)
	if err != nil {
		return importStats{}, errxtrace.Wrap("failed to parse XML metadata", err)
	}
	return importStats{
		LocationCount:  stats.LocationCount,
		AreaCount:      stats.AreaCount,
		CommodityCount: stats.CommodityCount,
		ImageCount:     stats.ImageCount,
		InvoiceCount:   stats.InvoiceCount,
		ManualCount:    stats.ManualCount,
		FileCount:      stats.FileCount,
		BinaryDataSize: stats.BinaryDataSize,
	}, nil
}
