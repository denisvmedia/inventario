package services

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/memblob" // register memblob driver

	"go.5x5.cz/inventario/models"
)

type stubBackfillRegistry struct {
	updates int
}

func (s *stubBackfillRegistry) Update(_ context.Context, f models.FileEntity) (*models.FileEntity, error) {
	s.updates++
	return &f, nil
}

func fileAt(path string) *models.FileEntity {
	return &models.FileEntity{File: &models.File{OriginalPath: path}}
}

// TestBackfillBatch_EmptyBlobDoesNotCountAsProgress is the regression test for
// the hot loop in #2129. ListPendingSizeBackfill selects size_bytes = 0, so a
// genuinely empty blob is rewritten to the same 0 and stays selected. Counting
// that write as progress kept the caller's loop alive for the life of the
// process, with no sleep between rounds.
func TestBackfillBatch_EmptyBlobDoesNotCountAsProgress(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	bucket, err := blob.OpenBucket(ctx, "mem://")
	c.Assert(err, qt.IsNil)
	defer bucket.Close()

	c.Assert(bucket.WriteAll(ctx, "empty", []byte{}, nil), qt.IsNil)
	c.Assert(bucket.WriteAll(ctx, "sized", []byte("0123456789"), nil), qt.IsNil)

	reg := &stubBackfillRegistry{}
	updated, advanced, failed, cancelled := backfillBatch(ctx, reg, bucket,
		[]*models.FileEntity{fileAt("empty"), fileAt("sized")})

	c.Assert(cancelled, qt.IsFalse)
	c.Assert(failed, qt.Equals, 0)
	c.Assert(updated, qt.Equals, 2, qt.Commentf("both rows were written"))
	c.Assert(advanced, qt.Equals, 1,
		qt.Commentf("only the sized blob leaves the pending set; the empty one stays selected"))
}

func TestBackfillBatch_AllEmptyReportsNoProgress(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	bucket, err := blob.OpenBucket(ctx, "mem://")
	c.Assert(err, qt.IsNil)
	defer bucket.Close()
	c.Assert(bucket.WriteAll(ctx, "a", []byte{}, nil), qt.IsNil)
	c.Assert(bucket.WriteAll(ctx, "b", []byte{}, nil), qt.IsNil)

	_, advanced, _, _ := backfillBatch(ctx, &stubBackfillRegistry{}, bucket,
		[]*models.FileEntity{fileAt("a"), fileAt("b")})

	// The caller breaks on this, which is what stops the spin.
	c.Assert(advanced, qt.Equals, 0)
}

func TestBackfillBatch_RespectsCancellation(t *testing.T) {
	c := qt.New(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	bucket, err := blob.OpenBucket(context.Background(), "mem://")
	c.Assert(err, qt.IsNil)
	defer bucket.Close()

	_, _, _, cancelled := backfillBatch(ctx, &stubBackfillRegistry{}, bucket,
		[]*models.FileEntity{fileAt("a")})

	c.Assert(cancelled, qt.IsTrue)
}
