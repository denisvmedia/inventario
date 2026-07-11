//go:build !legacy_xml_backup

package export

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	_ "github.com/denisvmedia/inventario/internal/fileblob"
	"github.com/denisvmedia/inventario/models"
)

// closedBucket returns a bucket whose every operation fails with a
// FailedPrecondition — the stand-in for a transient storage failure (network
// blip, throttling, a 5xx), i.e. a probe that cannot determine whether the blob
// exists. Distinct from a bucket that simply doesn't hold the key, which answers
// NotFound / (false, nil).
func closedBucket(c *qt.C) *blob.Bucket {
	b := must.Must(blob.OpenBucket(context.Background(), "file://inb-probe?memfs=1&create_dir=1"))
	c.Assert(b.Close(), qt.IsNil)
	return b
}

func probeFile(sizeHint int64) *models.FileEntity {
	f := &models.FileEntity{
		Title: "probe",
		File:  &models.File{Path: "probe", Ext: ".jpg", MIMEType: "image/jpeg", SizeBytes: sizeHint},
	}
	f.OriginalPath = "t/tenant-a/files/probe.jpg"
	return f
}

// TestResolveFileSize_TransientProbeFailureFailsExport pins the difference between
// "the blob is gone" and "I could not find out". Only the former is an orphan the
// exporter may silently drop; treating the latter as an orphan would hand the user
// a backup that reports success while missing live files — the exact silent
// data-loss class #2235 exists to eliminate. Both probe branches are covered: the
// Exists path (taken when the row carries a SizeBytes hint) and the Attributes
// path (taken when it does not).
func TestResolveFileSize_TransientProbeFailureFailsExport(t *testing.T) {
	tests := []struct {
		name     string
		sizeHint int64 // >0 → Exists probe; 0 → Attributes probe
	}{
		{name: "exists probe (size hint present)", sizeHint: 512},
		{name: "attributes probe (no size hint)", sizeHint: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			b := &inbBuilder{ctx: context.Background(), bucket: closedBucket(c)}

			size, present, err := b.resolveFileSize(probeFile(tt.sizeHint))

			c.Assert(err, qt.IsNotNil)
			c.Assert(present, qt.IsFalse)
			c.Assert(size, qt.Equals, int64(0))
		})
	}
}
