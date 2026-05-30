package backup

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/internal/backupsign"
	"github.com/denisvmedia/inventario/internal/inb"
)

func seed(b byte) []byte {
	s := make([]byte, backupsign.SeedSize)
	for i := range s {
		s[i] = b
	}
	return s
}

// writeArchive builds a minimal .inb archive on disk signed by signer and
// returns its path and the payload bytes.
func writeArchive(c *qt.C, dir string, signer *backupsign.Signer, payload []byte) (string, []byte) {
	digest := backupsign.NewDigest()
	_, _ = digest.Write(payload)
	sig := signer.SignDigest(digest.Sum(nil))

	path := filepath.Join(dir, "backup.inb")
	f := must.Must(os.Create(path))
	defer f.Close()
	c.Assert(inb.WriteContainer(f, sig, bytes.NewReader(payload), int64(len(payload))), qt.IsNil)
	return path, payload
}

// readArchive returns the (sig, payload) of an .inb file on disk.
func readArchive(c *qt.C, path string) (sig, payload []byte) {
	data := must.Must(os.ReadFile(path))
	tr := tar.NewReader(bytes.NewReader(data))
	_ = must.Must(tr.Next())
	sig = must.Must(io.ReadAll(tr))
	_ = must.Must(tr.Next())
	payload = must.Must(io.ReadAll(tr))
	return sig, payload
}

func TestResign_ChangesSigKeepsPayload(t *testing.T) {
	c := qt.New(t)
	dir := c.TempDir()
	oldSigner := must.Must(backupsign.NewSigner(seed(0x01)))
	newSigner := must.Must(backupsign.NewSigner(seed(0x02)))

	payload := []byte("payload-bytes-that-stay-identical")
	path, _ := writeArchive(c, dir, oldSigner, payload)
	_, beforePayload := readArchive(c, path)

	cmd := newResignCmd()
	cmd.SetArgs([]string{path, "--backup-signing-key", hexSeed(0x02), "--no-verify"})
	cmd.SetOut(io.Discard)
	c.Assert(cmd.Execute(), qt.IsNil)

	afterSig, afterPayload := readArchive(c, path)
	// Payload stays byte-identical; only the signature changed.
	c.Assert(afterPayload, qt.DeepEquals, beforePayload)
	// The new signature verifies under the new key, not the old.
	digest := backupsign.NewDigest()
	_, _ = digest.Write(afterPayload)
	c.Assert(newSigner.VerifyDigest(digest.Sum(nil), afterSig), qt.IsNil)
	c.Assert(oldSigner.VerifyDigest(digest.Sum(nil), afterSig), qt.ErrorIs, backupsign.ErrBadSignature)
}

func TestResign_NoOpWhenCurrentKeyVerifies(t *testing.T) {
	c := qt.New(t)
	dir := c.TempDir()
	signer := must.Must(backupsign.NewSigner(seed(0x03)))
	payload := []byte("already-signed-by-current-key")
	path, _ := writeArchive(c, dir, signer, payload)
	beforeSig, _ := readArchive(c, path)

	cmd := newResignCmd()
	var out bytes.Buffer
	cmd.SetArgs([]string{path, "--backup-signing-key", hexSeed(0x03)})
	cmd.SetOut(&out)
	c.Assert(cmd.Execute(), qt.IsNil)

	afterSig, _ := readArchive(c, path)
	// No-op: signature unchanged.
	c.Assert(afterSig, qt.DeepEquals, beforeSig)
	c.Assert(out.String(), qt.Contains, "nothing to do")
}

func TestResign_VerifyKeyMismatchAborts(t *testing.T) {
	c := qt.New(t)
	dir := c.TempDir()
	oldSigner := must.Must(backupsign.NewSigner(seed(0x04)))
	wrongSigner := must.Must(backupsign.NewSigner(seed(0x05)))
	payload := []byte("verify-key-mismatch")
	path, _ := writeArchive(c, dir, oldSigner, payload)

	cmd := newResignCmd()
	cmd.SetArgs([]string{path, "--backup-signing-key", hexSeed(0x06), "--verify-key", wrongSigner.PublicKeyBase64()})
	cmd.SetOut(io.Discard)
	err := cmd.Execute()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "does not verify")
}

func TestResign_OutputFlagLeavesInputUntouched(t *testing.T) {
	c := qt.New(t)
	dir := c.TempDir()
	oldSigner := must.Must(backupsign.NewSigner(seed(0x07)))
	payload := []byte("output-flag-test")
	inPath, _ := writeArchive(c, dir, oldSigner, payload)
	inSigBefore, _ := readArchive(c, inPath)

	outPath := filepath.Join(dir, "resigned.inb")
	cmd := newResignCmd()
	cmd.SetArgs([]string{inPath, "-o", outPath, "--backup-signing-key", hexSeed(0x08), "--no-verify"})
	cmd.SetOut(io.Discard)
	c.Assert(cmd.Execute(), qt.IsNil)

	// Input file unchanged; output file exists with a new signature.
	inSigAfter, _ := readArchive(c, inPath)
	c.Assert(inSigAfter, qt.DeepEquals, inSigBefore)
	outSig, outPayload := readArchive(c, outPath)
	c.Assert(outPayload, qt.DeepEquals, payload)
	newSigner := must.Must(backupsign.NewSigner(seed(0x08)))
	digest := backupsign.NewDigest()
	_, _ = digest.Write(outPayload)
	c.Assert(newSigner.VerifyDigest(digest.Sum(nil), outSig), qt.IsNil)
}

// hexSeed returns the hex encoding of a uniform seed byte, for the
// --backup-signing-key flag (64 hex chars → 32 bytes).
func hexSeed(b byte) string {
	s := seed(b)
	const hexdigits = "0123456789abcdef"
	out := make([]byte, len(s)*2)
	for i, v := range s {
		out[i*2] = hexdigits[v>>4]
		out[i*2+1] = hexdigits[v&0x0f]
	}
	return string(out)
}
