package inb_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/internal/inb"
)

// gzipTar returns gzip(tar) of the given members, in order.
func gzipTar(c *qt.C, members ...[2]string) []byte {
	c.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, m := range members {
		c.Assert(tw.WriteHeader(&tar.Header{Name: m[0], Mode: 0o600, Size: int64(len(m[1])), Typeflag: tar.TypeReg}), qt.IsNil)
		_, err := tw.Write([]byte(m[1]))
		c.Assert(err, qt.IsNil)
	}
	c.Assert(tw.Close(), qt.IsNil)
	c.Assert(gz.Close(), qt.IsNil)
	return buf.Bytes()
}

func TestIsPersonalDataExport(t *testing.T) {
	personal := [2]string{"manifest.json", `{"kind":"personal_data_export","formatVersion":"1.0"}`}
	tests := []struct {
		name string
		head func(c *qt.C) []byte
		want bool
	}{
		{name: "personal data export", head: func(c *qt.C) []byte {
			return gzipTar(c, personal, [2]string{"profile.json", "{}"})
		}, want: true},
		{name: "backup payload manifest", head: func(c *qt.C) []byte {
			return gzipTar(c, [2]string{"manifest.json", `{"formatVersion":"2.0"}`})
		}, want: false},
		{name: "manifest not first", head: func(c *qt.C) []byte {
			return gzipTar(c, [2]string{"profile.json", "{}"}, personal)
		}, want: false},
		{name: "not gzip", head: func(*qt.C) []byte { return []byte("plain text") }, want: false},
		{name: "empty", head: func(*qt.C) []byte { return nil }, want: false},
		{name: "cut short", head: func(c *qt.C) []byte { return gzipTar(c, personal)[:20] }, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(inb.IsPersonalDataExport(tt.head(c)), qt.Equals, tt.want)
		})
	}
}

func TestReadContainer_RefusesPersonalDataExport(t *testing.T) {
	c := qt.New(t)
	archive := gzipTar(c, [2]string{"manifest.json", `{"kind":"personal_data_export"}`})

	_, _, err := inb.ReadContainer(bytes.NewReader(archive), inb.DefaultLimits())
	c.Assert(err, qt.ErrorIs, inb.ErrPersonalDataExport)
}

func TestReadContainer_StillReadsABackupAfterSniffing(t *testing.T) {
	c := qt.New(t)
	payload := bytes.Repeat([]byte("p"), 3*inb.SniffHeadSize)
	var buf bytes.Buffer
	c.Assert(inb.WriteContainer(&buf, []byte("sig"), bytes.NewReader(payload), int64(len(payload))), qt.IsNil)

	sig, r, err := inb.ReadContainer(&buf, inb.DefaultLimits())
	c.Assert(err, qt.IsNil)
	c.Assert(string(sig), qt.Equals, "sig")
	var got bytes.Buffer
	_, err = got.ReadFrom(r)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Bytes(), qt.DeepEquals, payload)
}
