//go:build !legacy_xml_backup

package importpkg

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/internal/backupsign"
	"github.com/denisvmedia/inventario/internal/inb"
	"github.com/denisvmedia/inventario/models"
)

// importFileMeta returns the FileEntity stamping for an imported `.inb` backup
// artifact: `.inb` / application/x-inventario-backup / "inb-2.0" (#534).
func importFileMeta() importFileMetaFields {
	return importFileMetaFields{
		Ext:              ".inb",
		MIMEType:         "application/x-inventario-backup",
		LinkedEntityMeta: "inb-2.0",
		Tags:             models.StringSlice{"export", "inb", "imported"},
	}
}

// manifestDoc is the subset of the `.inb` manifest the import path reads.
type manifestDoc struct {
	Statistics struct {
		LocationCount  int   `json:"locationCount"`
		AreaCount      int   `json:"areaCount"`
		CommodityCount int   `json:"commodityCount"`
		ImageCount     int   `json:"imageCount"`
		InvoiceCount   int   `json:"invoiceCount"`
		ManualCount    int   `json:"manualCount"`
		FileCount      int   `json:"fileCount"`
		TotalFileSize  int64 `json:"totalFileSize"`
	} `json:"statistics"`
}

// parseImportMetadata verifies the uploaded `.inb` archive's signature against
// the server's own key, then reads manifest.json for the statistics used to
// stamp the import's export record (#534). A bad/missing signature or a
// non-`.inb` upload (e.g. legacy XML) fails hard — there is no bypass.
func (s *ImportService) parseImportMetadata(_ context.Context, reader io.Reader) (importStats, error) {
	if s.signer == nil {
		return importStats{}, errx.NewSentinel("backup signer is required to import an .inb archive")
	}

	sig, payload, err := inb.ReadContainer(reader, inb.DefaultLimits())
	if err != nil {
		return importStats{}, errxtrace.Wrap("not a valid signed .inb backup archive", err)
	}

	// Spool the payload to a temp file while digesting so verification never
	// buffers the whole payload.
	tmp, err := os.CreateTemp("", "inventario-inb-import-*.payload")
	if err != nil {
		return importStats{}, errxtrace.Wrap("failed to create temp payload file", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	digest := backupsign.NewDigest()
	if _, err := io.Copy(io.MultiWriter(tmp, digest), payload); err != nil {
		return importStats{}, errxtrace.Wrap("failed to spool backup payload", err)
	}

	// Verify BEFORE inflate.
	if err := s.signer.VerifyDigest(digest.Sum(nil), sig); err != nil {
		return importStats{}, errxtrace.Wrap("backup signature verification failed; refusing to import", err)
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return importStats{}, errxtrace.Wrap("failed to rewind temp payload", err)
	}

	manifest, err := readManifest(tmp)
	if err != nil {
		return importStats{}, err
	}

	return importStats{
		LocationCount:  manifest.Statistics.LocationCount,
		AreaCount:      manifest.Statistics.AreaCount,
		CommodityCount: manifest.Statistics.CommodityCount,
		ImageCount:     manifest.Statistics.ImageCount,
		InvoiceCount:   manifest.Statistics.InvoiceCount,
		ManualCount:    manifest.Statistics.ManualCount,
		FileCount:      manifest.Statistics.FileCount,
		BinaryDataSize: manifest.Statistics.TotalFileSize,
	}, nil
}

// maxManifestBytes caps the inflated size of manifest.json. The manifest is a
// small statistics document; a hostile archive declaring a multi-GiB manifest
// would otherwise let io.ReadAll exhaust memory even though the signature
// verified (the attacker controls the payload after re-signing with a leaked
// key, or simply by being a legitimate-but-malicious tenant).
const maxManifestBytes int64 = 4 << 20 // 4 MiB

// ErrManifestTooLarge is returned when manifest.json's declared size exceeds
// maxManifestBytes (or is negative). Capping before io.ReadAll prevents an
// out-of-memory DoS from an oversized manifest member.
var ErrManifestTooLarge = errx.NewSentinel("backup manifest.json exceeds the maximum allowed size")

// readManifest inflates the verified payload and reads manifest.json. It only
// reads the manifest member — file bytes are never inflated during import.
func readManifest(payload io.Reader) (*manifestDoc, error) {
	gzr, err := gzip.NewReader(payload)
	if err != nil {
		return nil, errxtrace.Wrap("failed to open gzip reader", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		hdr, nextErr := tr.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, errxtrace.Wrap("failed to read inner tar", nextErr)
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if hdr.Name != "manifest.json" {
			// Skip non-manifest members (including large `files/...` bodies) —
			// the next tr.Next() advances past the unread bytes, so file bytes
			// are never inflated during import.
			continue
		}
		if hdr.Size < 0 || hdr.Size > maxManifestBytes {
			return nil, errx.Classify(ErrManifestTooLarge, errx.Attrs("size", hdr.Size, "max", maxManifestBytes))
		}
		var manifest manifestDoc
		data, err := io.ReadAll(io.LimitReader(tr, hdr.Size))
		if err != nil {
			return nil, errxtrace.Wrap("failed to read manifest", err)
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, errxtrace.Wrap("failed to decode manifest", err)
		}
		return &manifest, nil
	}
	return nil, errx.NewSentinel("backup archive is missing manifest.json")
}
