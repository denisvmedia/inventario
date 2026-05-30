package backup_test

import (
	"archive/tar"
	"bytes"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/inventario/backup"
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

// resignCmd builds the public `backup` command tree and pre-positions args at
// the `resign` subcommand, so tests drive the CLI exactly as an operator would
// (`inventario backup resign ...`) through the exported surface only.
func resignCmd(args ...string) *cobra.Command {
	cmd := backup.New()
	cmd.SetArgs(append([]string{"resign"}, args...))
	return cmd
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

	cmd := resignCmd(path, "--backup-signing-key", hexSeed(0x02), "--no-verify")
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

	cmd := resignCmd(path, "--backup-signing-key", hexSeed(0x03))
	var out bytes.Buffer
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

	cmd := resignCmd(path, "--backup-signing-key", hexSeed(0x06), "--verify-key", wrongSigner.PublicKeyBase64())
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
	cmd := resignCmd(inPath, "-o", outPath, "--backup-signing-key", hexSeed(0x08), "--no-verify")
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

// TestResign_InPlaceOverwrite drives the default (no -o) in-place re-sign, which
// closes the input before renaming the temp output over it. On Windows the
// rename would fail if the input were still open, so this exercises that path
// end to end through the public command.
func TestResign_InPlaceOverwrite(t *testing.T) {
	c := qt.New(t)
	dir := c.TempDir()
	oldSigner := must.Must(backupsign.NewSigner(seed(0x09)))
	payload := []byte("in-place-overwrite-test")
	path, _ := writeArchive(c, dir, oldSigner, payload)

	cmd := resignCmd(path, "--backup-signing-key", hexSeed(0x0a), "--no-verify")
	cmd.SetOut(io.Discard)
	c.Assert(cmd.Execute(), qt.IsNil)

	newSigner := must.Must(backupsign.NewSigner(seed(0x0a)))
	afterSig, afterPayload := readArchive(c, path)
	c.Assert(afterPayload, qt.DeepEquals, payload)
	digest := backupsign.NewDigest()
	_, _ = digest.Write(afterPayload)
	c.Assert(newSigner.VerifyDigest(digest.Sum(nil), afterSig), qt.IsNil)
}

// hexSeed returns the hex encoding of a uniform seed byte, for the
// --backup-signing-key flag (64 hex chars → 32 bytes).
func hexSeed(b byte) string {
	return hex.EncodeToString(seed(b))
}
