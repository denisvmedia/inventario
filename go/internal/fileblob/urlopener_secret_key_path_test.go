package fileblob_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"

	_ "github.com/denisvmedia/inventario/internal/fileblob"
)

func TestOpenBucketURL_AllowsSecretKeyPathWithinBucketRoot(t *testing.T) {
	c := qt.New(t)
	dir := t.TempDir()
	dirURLPath := toFileURLPath(dir)
	secretKeyPath := filepath.Join(dir, "secret.key")

	err := os.WriteFile(filepath.Join(dir, "myfile.txt"), []byte("hello world"), 0o600)
	c.Assert(err, qt.IsNil)
	err = os.WriteFile(secretKeyPath, []byte("secret key"), 0o600)
	c.Assert(err, qt.IsNil)

	tests := []struct {
		name          string
		secretKeyPath string
	}{
		{name: "relative", secretKeyPath: "secret.key"},
		{name: "absolute within bucket", secretKeyPath: filepath.ToSlash(secretKeyPath)},
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)

			bucket, err := blob.OpenBucket(ctx, fmt.Sprintf("file://%s?base_url=/show&secret_key_path=%s", dirURLPath, url.QueryEscape(test.secretKeyPath)))
			c.Assert(err, qt.IsNil)
			defer bucket.Close()

			contents, err := bucket.ReadAll(ctx, "myfile.txt")
			c.Assert(err, qt.IsNil)
			c.Assert(string(contents), qt.Equals, "hello world")
		})
	}
}

func TestOpenBucketURL_RejectsSecretKeyPathTraversal(t *testing.T) {
	c := qt.New(t)
	rootDir := t.TempDir()
	bucketDir := filepath.Join(rootDir, "bucket")

	err := os.MkdirAll(bucketDir, 0o755)
	c.Assert(err, qt.IsNil)
	err = os.WriteFile(filepath.Join(bucketDir, "myfile.txt"), []byte("hello world"), 0o600)
	c.Assert(err, qt.IsNil)
	err = os.WriteFile(filepath.Join(rootDir, "secret.key"), []byte("secret key"), 0o600)
	c.Assert(err, qt.IsNil)

	_, err = blob.OpenBucket(context.Background(), fmt.Sprintf("file://%s?base_url=/show&secret_key_path=%s", toFileURLPath(bucketDir), url.QueryEscape("../secret.key")))
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "invalid secret_key_path")
}

func toFileURLPath(dir string) string {
	dirURLPath := filepath.ToSlash(dir)
	if os.PathSeparator != '/' && !strings.HasPrefix(dirURLPath, "/") {
		dirURLPath = "/" + dirURLPath
	}
	return dirURLPath
}
