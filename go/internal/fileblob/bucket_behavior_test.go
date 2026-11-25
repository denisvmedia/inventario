package fileblob_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/spf13/afero"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/internal/fileblob"
)

// These tests cover the behavior ported from gocloud.dev v0.49's fileblob.

func TestBucket_RejectsKeysThatEscapeTheBucketRoot(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	rootDir := t.TempDir()
	bucketDir := filepath.Join(rootDir, "bucket")
	c.Assert(os.MkdirAll(bucketDir, 0o755), qt.IsNil)

	bucket, err := fileblob.OpenBucket(bucketDir, nil)
	c.Assert(err, qt.IsNil)
	defer bucket.Close()

	// ".." points at the parent and "a/.." at the bucket itself. A ".."
	// followed by "/" never gets this far: escapeKey escapes that slash.
	for _, key := range []string{"..", "a/.."} {
		c.Run(key, func(c *qt.C) {
			err := bucket.WriteAll(ctx, key, []byte("x"), nil)
			c.Assert(err, qt.ErrorMatches, `.*escapes bucket root.*`)

			_, err = bucket.ReadAll(ctx, key)
			c.Assert(err, qt.ErrorMatches, `.*escapes bucket root.*`)

			err = bucket.Delete(ctx, key)
			c.Assert(err, qt.ErrorMatches, `.*escapes bucket root.*`)
		})
	}

	// Nothing was written next to the bucket.
	entries, err := os.ReadDir(rootDir)
	c.Assert(err, qt.IsNil)
	c.Assert(entries, qt.HasLen, 1)
	c.Assert(entries[0].Name(), qt.Equals, "bucket")
}

func TestBucket_RejectsListPrefixThatEscapesTheBucketRoot(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	bucketDir := filepath.Join(t.TempDir(), "bucket")
	c.Assert(os.MkdirAll(bucketDir, 0o755), qt.IsNil)

	bucket, err := fileblob.OpenBucket(bucketDir, nil)
	c.Assert(err, qt.IsNil)
	defer bucket.Close()

	_, _, err = bucket.ListPage(ctx, blob.FirstPageToken, 10, &blob.ListOptions{Prefix: "../"})
	c.Assert(err, qt.ErrorMatches, `.*escapes bucket root.*`)
}

func TestBucket_ListReturnsTheMD5OfWrittenBlobs(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	bucket, err := fileblob.OpenBucket(t.TempDir(), nil)
	c.Assert(err, qt.IsNil)
	defer bucket.Close()

	c.Assert(bucket.WriteAll(ctx, "dir/key", []byte("hello"), nil), qt.IsNil)

	objs, _, err := bucket.ListPage(ctx, blob.FirstPageToken, 10, &blob.ListOptions{Prefix: "dir/"})
	c.Assert(err, qt.IsNil)
	c.Assert(objs, qt.HasLen, 1)
	// md5("hello")
	c.Assert(objs[0].MD5, qt.DeepEquals, []byte{
		0x5d, 0x41, 0x40, 0x2a, 0xbc, 0x4b, 0x2a, 0x76, 0xb9, 0x71, 0x9d, 0x91, 0x10, 0x17, 0xc5, 0x92,
	})
}

func TestBucket_ListPagesStayInKeyOrderWithADelimiter(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	bucket, err := fileblob.OpenBucket(t.TempDir(), nil)
	c.Assert(err, qt.IsNil)
	defer bucket.Close()

	// The walk visits "a/b" before "a-b", but with the delimiter "/" the
	// "directory" a/ sorts after the file a-b.
	for _, key := range []string{"a/b", "a-b", "b"} {
		c.Assert(bucket.WriteAll(ctx, key, []byte("x"), nil), qt.IsNil)
	}

	var got []string
	token := blob.FirstPageToken
	for {
		objs, next, err := bucket.ListPage(ctx, token, 1, &blob.ListOptions{Delimiter: "/"})
		c.Assert(err, qt.IsNil)
		for _, o := range objs {
			got = append(got, o.Key)
		}
		if next == nil {
			break
		}
		token = next
	}
	c.Assert(got, qt.DeepEquals, []string{"a-b", "a/", "b"})
}

func TestBucket_DeleteCallsBeforeDeleteOnce(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	bucket, err := fileblob.OpenBucket(t.TempDir(), nil)
	c.Assert(err, qt.IsNil)
	defer bucket.Close()

	c.Assert(bucket.WriteAll(ctx, "key", []byte("x"), nil), qt.IsNil)

	refused := errors.New("refused")
	calls := 0
	err = bucket.DeleteWithOptions(ctx, "key", &blob.DeleteOptions{
		BeforeDelete: func(func(any) bool) error {
			calls++
			return refused
		},
	})
	c.Assert(err, qt.ErrorIs, refused)
	c.Assert(calls, qt.Equals, 1)

	exists, err := bucket.Exists(ctx, "key")
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue)

	calls = 0
	err = bucket.DeleteWithOptions(ctx, "key", &blob.DeleteOptions{
		BeforeDelete: func(func(any) bool) error {
			calls++
			return nil
		},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(calls, qt.Equals, 1)

	exists, err = bucket.Exists(ctx, "key")
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)
}

func TestBucket_BeforeWriteErrorRemovesTheTempFile(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := afero.NewMemMapFs()
	bucketDir := "/bucket"
	c.Assert(fs.MkdirAll(bucketDir, 0o755), qt.IsNil)

	bucket, err := fileblob.OpenBucket(bucketDir, &fileblob.Options{FS: fs})
	c.Assert(err, qt.IsNil)
	defer bucket.Close()

	refused := errors.New("before write failed")
	var tempName string
	_, err = bucket.NewWriter(ctx, "key", &blob.WriterOptions{
		ContentType: "text/plain",
		BeforeWrite: func(as func(any) bool) error {
			var f afero.File
			c.Assert(as(&f), qt.IsTrue)
			tempName = f.Name()
			return refused
		},
	})
	c.Assert(err, qt.ErrorIs, refused)
	c.Assert(tempName, qt.Not(qt.Equals), "")

	_, err = fs.Stat(tempName)
	c.Assert(os.IsNotExist(err), qt.IsTrue, qt.Commentf("temp file %s still exists: %v", tempName, err))
}
