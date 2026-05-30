package inb_test

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/inb"
)

func TestWriteReadContainer_RoundTrip(t *testing.T) {
	c := qt.New(t)

	payload := []byte("gzip payload bytes")
	sig := []byte("signature bytes")

	var buf bytes.Buffer
	c.Assert(inb.WriteContainer(&buf, sig, bytes.NewReader(payload), int64(len(payload))), qt.IsNil)

	gotSig, payloadReader, err := inb.ReadContainer(&buf, inb.DefaultLimits())
	c.Assert(err, qt.IsNil)
	c.Assert(gotSig, qt.DeepEquals, sig)
	gotPayload, err := io.ReadAll(payloadReader)
	c.Assert(err, qt.IsNil)
	c.Assert(gotPayload, qt.DeepEquals, payload)
}

func TestWriteContainer_SignatureMemberFirst(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	c.Assert(inb.WriteContainer(&buf, []byte("s"), bytes.NewReader([]byte("p")), 1), qt.IsNil)

	tr := tar.NewReader(&buf)
	first, err := tr.Next()
	c.Assert(err, qt.IsNil)
	c.Assert(first.Name, qt.Equals, inb.SignatureName)
	second, err := tr.Next()
	c.Assert(err, qt.IsNil)
	c.Assert(second.Name, qt.Equals, inb.PayloadName)
}

func TestReadContainer_MissingPayload(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	writeRaw(c, tw, inb.SignatureName, []byte("sig only"))
	c.Assert(tw.Close(), qt.IsNil)

	_, _, err := inb.ReadContainer(&buf, inb.DefaultLimits())
	c.Assert(err, qt.ErrorIs, inb.ErrMissingPayload)
}

func TestReadContainer_MissingSignature(t *testing.T) {
	c := qt.New(t)

	// payload but no signature member.
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	writeRaw(c, tw, inb.PayloadName, []byte("payload only"))
	c.Assert(tw.Close(), qt.IsNil)

	// First member is the payload → caught as out-of-order (signature first).
	_, _, err := inb.ReadContainer(&buf, inb.DefaultLimits())
	c.Assert(err, qt.ErrorIs, inb.ErrBadOrder)
}

func TestReadContainer_EmptyArchive(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	c.Assert(tw.Close(), qt.IsNil)

	_, _, err := inb.ReadContainer(&buf, inb.DefaultLimits())
	c.Assert(err, qt.ErrorIs, inb.ErrMissingSignature)
}

func TestReadContainer_RejectsPayloadBeforeSignature(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	writeRaw(c, tw, inb.PayloadName, []byte("p"))
	writeRaw(c, tw, inb.SignatureName, []byte("s"))
	c.Assert(tw.Close(), qt.IsNil)

	_, _, err := inb.ReadContainer(&buf, inb.DefaultLimits())
	c.Assert(err, qt.ErrorIs, inb.ErrBadOrder)
}

func TestReadContainer_RejectsUnexpectedFirstMember(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	writeRaw(c, tw, "evil.sh", []byte("rm -rf"))
	c.Assert(tw.Close(), qt.IsNil)

	_, _, err := inb.ReadContainer(&buf, inb.DefaultLimits())
	c.Assert(err, qt.ErrorIs, inb.ErrUnexpectedMember)
}

func TestReadContainer_RejectsUnexpectedSecondMember(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	writeRaw(c, tw, inb.SignatureName, []byte("s"))
	writeRaw(c, tw, "evil.sh", []byte("rm -rf"))
	c.Assert(tw.Close(), qt.IsNil)

	_, _, err := inb.ReadContainer(&buf, inb.DefaultLimits())
	c.Assert(err, qt.ErrorIs, inb.ErrUnexpectedMember)
}

func TestReadContainer_RejectsSymlinkSignatureMember(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	c.Assert(tw.WriteHeader(&tar.Header{
		Name:     inb.SignatureName,
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
		Mode:     0o777,
	}), qt.IsNil)
	c.Assert(tw.Close(), qt.IsNil)

	_, _, err := inb.ReadContainer(&buf, inb.DefaultLimits())
	c.Assert(err, qt.ErrorIs, inb.ErrUnexpectedMember)
}

func TestReadContainer_RejectsDuplicateSignature(t *testing.T) {
	c := qt.New(t)

	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	writeRaw(c, tw, inb.SignatureName, []byte("first"))
	writeRaw(c, tw, inb.SignatureName, []byte("second"))
	c.Assert(tw.Close(), qt.IsNil)

	_, _, err := inb.ReadContainer(&buf, inb.DefaultLimits())
	c.Assert(err, qt.ErrorIs, inb.ErrDuplicateMember)
}

func TestReadContainer_RejectsOversizedPayload(t *testing.T) {
	c := qt.New(t)

	payload := bytes.Repeat([]byte("a"), 100)
	var buf bytes.Buffer
	c.Assert(inb.WriteContainer(&buf, []byte("s"), bytes.NewReader(payload), int64(len(payload))), qt.IsNil)

	_, _, err := inb.ReadContainer(&buf, inb.Limits{MaxPayloadBytes: 10, MaxSignatureBytes: 4 << 10})
	c.Assert(err, qt.ErrorIs, inb.ErrPayloadTooLarge)
}

func TestReadContainer_RejectsOversizedSignature(t *testing.T) {
	c := qt.New(t)

	sig := bytes.Repeat([]byte("s"), 100)
	var buf bytes.Buffer
	c.Assert(inb.WriteContainer(&buf, sig, bytes.NewReader([]byte("p")), 1), qt.IsNil)

	_, _, err := inb.ReadContainer(&buf, inb.Limits{MaxPayloadBytes: 1 << 20, MaxSignatureBytes: 10})
	c.Assert(err, qt.ErrorIs, inb.ErrSignatureTooLarge)
}

func writeRaw(c *qt.C, tw *tar.Writer, name string, data []byte) {
	c.Helper()
	c.Assert(tw.WriteHeader(&tar.Header{
		Name:     name,
		Mode:     0o600,
		Size:     int64(len(data)),
		Typeflag: tar.TypeReg,
	}), qt.IsNil)
	_, err := tw.Write(data)
	c.Assert(err, qt.IsNil)
}
