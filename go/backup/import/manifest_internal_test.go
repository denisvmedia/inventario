//go:build !legacy_xml_backup

package importpkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/internal/backupsign"
	"go.5x5.cz/inventario/internal/inb"
)

// tarMember is one entry to write into the test archive.
//
// The header's Size is always the body's real length: archive/tar's Writer
// refuses to close when a header overstates its body ("missed writing N
// bytes"), so a header that lies is not constructible here. The oversize
// case below therefore writes a genuinely oversized member.
type tarMember struct {
	name     string
	body     string
	typeflag byte
}

func gzippedTar(c *qt.C, members ...tarMember) io.Reader {
	c.Helper()

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	for _, m := range members {
		typeflag := m.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		size := int64(len(m.body))
		c.Assert(tw.WriteHeader(&tar.Header{
			Name:     m.name,
			Typeflag: typeflag,
			Size:     size,
			Mode:     0o600,
		}), qt.IsNil)
		_, err := tw.Write([]byte(m.body))
		c.Assert(err, qt.IsNil)
	}
	c.Assert(tw.Close(), qt.IsNil)
	c.Assert(gzw.Close(), qt.IsNil)
	return bytes.NewReader(buf.Bytes())
}

func manifestJSON(locations, areas, commodities, files int, totalFileSize int64) string {
	doc := map[string]any{
		"statistics": map[string]any{
			"locationCount":  locations,
			"areaCount":      areas,
			"commodityCount": commodities,
			"fileCount":      files,
			"totalFileSize":  totalFileSize,
		},
	}
	out, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return string(out)
}

func TestReadManifest_ReadsTheStatistics(t *testing.T) {
	c := qt.New(t)

	archive := gzippedTar(c,
		tarMember{name: "manifest.json", body: manifestJSON(3, 7, 11, 5, 4096)},
	)

	manifest, err := readManifest(archive)
	c.Assert(err, qt.IsNil)
	c.Check(manifest.Statistics.LocationCount, qt.Equals, 3)
	c.Check(manifest.Statistics.AreaCount, qt.Equals, 7)
	c.Check(manifest.Statistics.CommodityCount, qt.Equals, 11)
	c.Check(manifest.Statistics.FileCount, qt.Equals, 5)
	c.Check(manifest.Statistics.TotalFileSize, qt.Equals, int64(4096))
}

// The manifest is found wherever it sits, and the members around it are
// stepped over rather than inflated — that is the property that keeps import
// from expanding a multi-gigabyte archive to read a statistics document.
func TestReadManifest_SkipsEveryOtherMember(t *testing.T) {
	c := qt.New(t)

	archive := gzippedTar(c,
		tarMember{name: "files/", typeflag: tar.TypeDir},
		tarMember{name: "files/photo.jpg", body: strings.Repeat("A", 64<<10)},
		tarMember{name: "exports/report.csv", body: strings.Repeat("B", 8<<10)},
		tarMember{name: "manifest.json", body: manifestJSON(1, 2, 3, 4, 10)},
		tarMember{name: "files/invoice.pdf", body: strings.Repeat("C", 32<<10)},
	)

	manifest, err := readManifest(archive)
	c.Assert(err, qt.IsNil)
	c.Check(manifest.Statistics.CommodityCount, qt.Equals, 3)
}

// The cap is read off the tar header, before any of the member is read. A
// header that declares more than the cap has to be refused on the declaration
// alone — checking after io.ReadAll would be the out-of-memory this guards.
func TestReadManifest_RefusesAnOversizedDeclaration(t *testing.T) {
	c := qt.New(t)

	base := manifestJSON(1, 1, 1, 1, 1)
	body := base + strings.Repeat(" ", int(maxManifestBytes)+1-len(base))
	c.Assert(int64(len(body)), qt.Equals, maxManifestBytes+1)

	// Valid JSON, one byte over the cap: the refusal is on the declared size
	// and not on the content, so the document never reaches the decoder.
	_, err := readManifest(gzippedTar(c, tarMember{name: "manifest.json", body: body}))
	c.Assert(err, qt.ErrorIs, ErrManifestTooLarge)
}

// A manifest exactly at the cap is legal: the boundary is inclusive, so a
// document that fits is not rejected for fitting exactly.
func TestReadManifest_AcceptsTheCapExactly(t *testing.T) {
	c := qt.New(t)

	base := manifestJSON(1, 2, 3, 4, 10)
	// Pad with JSON whitespace so the member is exactly maxManifestBytes and
	// still decodes.
	padding := int(maxManifestBytes) - len(base)
	c.Assert(padding > 0, qt.IsTrue)
	body := base + strings.Repeat(" ", padding)
	c.Assert(int64(len(body)), qt.Equals, maxManifestBytes)

	manifest, err := readManifest(gzippedTar(c, tarMember{name: "manifest.json", body: body}))
	c.Assert(err, qt.IsNil)
	c.Check(manifest.Statistics.CommodityCount, qt.Equals, 3)
}

// rawTarHeader builds a 512-byte ustar header by hand. archive/tar's Writer
// refuses to emit a header that overstates its body, so this is the only way
// to produce the archive a hostile client would send: a member declaring
// gigabytes with almost nothing behind it.
func rawTarHeader(name string, size int64) []byte {
	var h [512]byte
	copy(h[0:100], name)
	copy(h[100:108], "0000600\x00")
	copy(h[108:116], "0000000\x00")
	copy(h[116:124], "0000000\x00")
	copy(h[124:136], fmt.Sprintf("%011o\x00", size))
	copy(h[136:148], "00000000000\x00")
	h[156] = tar.TypeReg
	copy(h[257:263], "ustar\x00")
	copy(h[263:265], "00")

	// The checksum is computed with its own field read as spaces.
	for i := 148; i < 156; i++ {
		h[i] = ' '
	}
	var sum int
	for _, b := range h {
		sum += int(b)
	}
	copy(h[148:156], fmt.Sprintf("%06o\x00 ", sum))
	return h[:]
}

// gzippedRaw wraps hand-built tar bytes so readManifest sees the same
// container shape it does in production.
func gzippedRaw(c *qt.C, blocks ...[]byte) io.Reader {
	c.Helper()

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	for _, b := range blocks {
		_, err := gzw.Write(b)
		c.Assert(err, qt.IsNil)
	}
	c.Assert(gzw.Close(), qt.IsNil)
	return bytes.NewReader(buf.Bytes())
}

// The cap is read off the tar header, before any of the member is read. This
// archive declares 8 GiB and carries one block, so a reader that allocated
// first would try for 8 GiB; the sentinel proves it never got that far.
func TestReadManifest_RefusesTheDeclarationWithoutReadingIt(t *testing.T) {
	c := qt.New(t)

	const declared = int64(8) << 30
	body := make([]byte, 512)
	copy(body, manifestJSON(1, 1, 1, 1, 1))

	_, err := readManifest(gzippedRaw(c, rawTarHeader("manifest.json", declared), body))
	c.Assert(err, qt.ErrorIs, ErrManifestTooLarge)
}

func TestReadManifest_Rejects(t *testing.T) {
	c := qt.New(t)

	c.Run("no manifest member", func(c *qt.C) {
		archive := gzippedTar(c, tarMember{name: "files/photo.jpg", body: "not a manifest"})
		_, err := readManifest(archive)
		c.Assert(err, qt.IsNotNil)
		c.Check(err.Error(), qt.Contains, "missing manifest.json")
	})

	c.Run("a directory named manifest.json", func(c *qt.C) {
		// Only a regular file counts. A directory entry with the right name
		// must not be mistaken for the document.
		archive := gzippedTar(c, tarMember{name: "manifest.json", typeflag: tar.TypeDir})
		_, err := readManifest(archive)
		c.Assert(err, qt.IsNotNil)
		c.Check(err.Error(), qt.Contains, "missing manifest.json")
	})

	c.Run("not gzip", func(c *qt.C) {
		_, err := readManifest(strings.NewReader("plain bytes, no gzip header"))
		c.Assert(err, qt.IsNotNil)
		c.Check(err.Error(), qt.Contains, "failed to open gzip reader")
	})

	c.Run("gzip but not tar", func(c *qt.C) {
		var buf bytes.Buffer
		gzw := gzip.NewWriter(&buf)
		_, err := gzw.Write([]byte("this is not a tar stream"))
		c.Assert(err, qt.IsNil)
		c.Assert(gzw.Close(), qt.IsNil)

		_, err = readManifest(bytes.NewReader(buf.Bytes()))
		c.Assert(err, qt.IsNotNil)
	})

	c.Run("manifest is not JSON", func(c *qt.C) {
		archive := gzippedTar(c, tarMember{name: "manifest.json", body: "{not json"})
		_, err := readManifest(archive)
		c.Assert(err, qt.IsNotNil)
		c.Check(err.Error(), qt.Contains, "failed to decode manifest")
	})
}

// signedArchive wraps payload in the `.inb` container, signed by signer.
func signedArchive(c *qt.C, signer *backupsign.Signer, payload []byte) io.Reader {
	c.Helper()

	digest := backupsign.NewDigest()
	_, err := digest.Write(payload)
	c.Assert(err, qt.IsNil)

	var buf bytes.Buffer
	c.Assert(inb.WriteContainer(&buf, signer.SignDigest(digest.Sum(nil)),
		bytes.NewReader(payload), int64(len(payload))), qt.IsNil)
	return bytes.NewReader(buf.Bytes())
}

func gzippedTarBytes(c *qt.C, members ...tarMember) []byte {
	c.Helper()

	r := gzippedTar(c, members...)
	out, err := io.ReadAll(r)
	c.Assert(err, qt.IsNil)
	return out
}

func seedOf(b byte) []byte {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = b
	}
	return seed
}

func TestParseImportMetadata_ReadsTheStatisticsFromASignedArchive(t *testing.T) {
	c := qt.New(t)

	signer, err := backupsign.NewSigner(seedOf(0x11))
	c.Assert(err, qt.IsNil)

	payload := gzippedTarBytes(c,
		tarMember{name: "files/photo.jpg", body: strings.Repeat("A", 32<<10)},
		tarMember{name: "manifest.json", body: manifestJSON(2, 4, 6, 8, 12345)},
	)

	svc := NewImportService(nil, "", signer)
	stats, err := svc.parseImportMetadata(context.Background(), signedArchive(c, signer, payload))
	c.Assert(err, qt.IsNil)
	c.Check(stats.LocationCount, qt.Equals, 2)
	c.Check(stats.AreaCount, qt.Equals, 4)
	c.Check(stats.CommodityCount, qt.Equals, 6)
	c.Check(stats.FileCount, qt.Equals, 8)
	c.Check(stats.BinaryDataSize, qt.Equals, int64(12345))
}

// The signature is checked before the payload is inflated, and the payload
// here is not a gzip stream at all. A verify-after-inflate implementation
// would report the gzip failure; reporting the signature failure is what
// proves the order.
func TestParseImportMetadata_VerifiesBeforeInflating(t *testing.T) {
	c := qt.New(t)

	ours, err := backupsign.NewSigner(seedOf(0x11))
	c.Assert(err, qt.IsNil)
	theirs, err := backupsign.NewSigner(seedOf(0x22))
	c.Assert(err, qt.IsNil)

	svc := NewImportService(nil, "", ours)
	_, err = svc.parseImportMetadata(context.Background(),
		signedArchive(c, theirs, []byte("not gzip, not a tar, not anything")))

	c.Assert(err, qt.IsNotNil)
	c.Check(err.Error(), qt.Contains, "signature verification failed")
	c.Check(err.Error(), qt.Not(qt.Contains), "gzip")
}

// The same archive signed by the right key gets past verification and then
// fails on the payload — the other half of the ordering claim.
func TestParseImportMetadata_InflatesOnceVerified(t *testing.T) {
	c := qt.New(t)

	signer, err := backupsign.NewSigner(seedOf(0x11))
	c.Assert(err, qt.IsNil)

	svc := NewImportService(nil, "", signer)
	_, err = svc.parseImportMetadata(context.Background(),
		signedArchive(c, signer, []byte("not gzip, not a tar, not anything")))

	c.Assert(err, qt.IsNotNil)
	c.Check(err.Error(), qt.Contains, "failed to open gzip reader")
}

func TestParseImportMetadata_Refuses(t *testing.T) {
	c := qt.New(t)

	signer, err := backupsign.NewSigner(seedOf(0x11))
	c.Assert(err, qt.IsNil)

	c.Run("no signer configured", func(c *qt.C) {
		// Without a key there is nothing to verify against, and an
		// unverified archive must not be read at all.
		svc := NewImportService(nil, "", nil)
		_, err := svc.parseImportMetadata(context.Background(), bytes.NewReader(nil))
		c.Assert(err, qt.IsNotNil)
		c.Check(err.Error(), qt.Contains, "backup signer is required")
	})

	c.Run("a legacy XML upload", func(c *qt.C) {
		// Not an `.inb` container, so it never reaches verification.
		xml := `<?xml version="1.0"?><inventory><locations/></inventory>`
		svc := NewImportService(nil, "", signer)
		_, err := svc.parseImportMetadata(context.Background(), strings.NewReader(xml))
		c.Assert(err, qt.IsNotNil)
		c.Check(err.Error(), qt.Contains, "not a valid signed .inb backup archive")
	})

	c.Run("a signed archive with no manifest", func(c *qt.C) {
		payload := gzippedTarBytes(c, tarMember{name: "files/photo.jpg", body: "bytes"})
		svc := NewImportService(nil, "", signer)
		_, err := svc.parseImportMetadata(context.Background(), signedArchive(c, signer, payload))
		c.Assert(err, qt.IsNotNil)
		c.Check(err.Error(), qt.Contains, "missing manifest.json")
	})
}
