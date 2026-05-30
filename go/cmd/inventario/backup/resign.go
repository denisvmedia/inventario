package backup

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/internal/backupsign"
	"github.com/denisvmedia/inventario/internal/inb"
)

// resignOptions holds the parsed flags for `inventario backup resign`.
type resignOptions struct {
	signingKey string
	output     string
	verifyKey  string
	noVerify   bool
	force      bool
}

// newResignCmd builds the `resign` subcommand. It re-signs an existing `.inb`
// archive under the server's current backup signing key WITHOUT touching the
// payload — only the `payload.tar.gz.sig` member changes, so payload.tar.gz
// stays byte-identical (a key-rotation tool, issue #534).
func newResignCmd() *cobra.Command {
	opts := &resignOptions{}
	cmd := &cobra.Command{
		Use:   "resign <input.inb>",
		Short: "Re-sign an .inb archive under the current backup signing key",
		Long: `Re-sign an .inb backup archive under the current backup signing key.

The payload.tar.gz member is left byte-identical; only the detached signature is
replaced. This is the key-rotation path: after rotating the backup signing key
(INVENTARIO_RUN_BACKUP_SIGNING_KEY), run resign so previously-produced archives
verify under the new key.

Verification before re-signing:
  --verify-key <hex|file>  verify the existing signature against this public key
                           (PEM, base64, or hex) and abort on mismatch.
  (default)                if the current key already verifies the archive, this
                           is a no-op unless --force.
  --no-verify              skip the old-signature check entirely (explicit).

By default the input file is overwritten in place (atomically); use -o to write
to a different path.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runResign(cmd, args[0], opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.signingKey, "backup-signing-key", "", "Ed25519 seed (64 hex chars or 32 raw bytes); falls back to "+backupSigningKeyEnv)
	flags.StringVarP(&opts.output, "output", "o", "", "output .inb path (default: overwrite input)")
	flags.StringVar(&opts.verifyKey, "verify-key", "", "verify the existing signature against this public key (PEM/base64/hex, or a file path) before re-signing")
	flags.BoolVar(&opts.noVerify, "no-verify", false, "skip the old-signature check entirely")
	flags.BoolVar(&opts.force, "force", false, "re-sign even if the current key already verifies the archive")
	return cmd
}

// runResign drives the resign flow: read container, spool payload while
// digesting, run the configured verification, then write the new container.
func runResign(cmd *cobra.Command, inputPath string, opts *resignOptions) error {
	signer, err := loadSigner(opts.signingKey)
	if err != nil {
		return err
	}

	in, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input archive: %w", err)
	}
	defer in.Close()

	oldSig, payload, err := inb.ReadContainer(in, inb.DefaultLimits())
	if err != nil {
		return fmt.Errorf("not a valid signed .inb archive: %w", err)
	}

	// Spool the payload to a temp file while digesting it. The payload is
	// written back byte-for-byte; only the signature changes.
	tmpPayload, err := os.CreateTemp("", "inventario-resign-*.payload")
	if err != nil {
		return fmt.Errorf("failed to create temp payload: %w", err)
	}
	tmpPayloadName := tmpPayload.Name()
	defer func() {
		_ = tmpPayload.Close()
		_ = os.Remove(tmpPayloadName)
	}()

	digest := backupsign.NewDigest()
	if _, err := io.Copy(io.MultiWriter(tmpPayload, digest), payload); err != nil {
		return fmt.Errorf("failed to spool payload: %w", err)
	}
	sum := digest.Sum(nil)

	skip, err := verifyForResign(cmd, signer, sum, oldSig, opts)
	if err != nil {
		return err
	}
	if skip {
		fmt.Fprintln(cmd.OutOrStdout(), "archive already verifies under the current key; nothing to do (use --force to re-sign anyway)")
		return nil
	}

	newSig := signer.SignDigest(sum)

	info, err := tmpPayload.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat payload: %w", err)
	}
	if _, err := tmpPayload.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to rewind payload: %w", err)
	}

	outputPath := opts.output
	if outputPath == "" {
		outputPath = inputPath
	}
	if err := writeContainerAtomically(outputPath, newSig, tmpPayload, info.Size()); err != nil {
		return err
	}

	fmt.Fprintf(cmd.OutOrStdout(), "re-signed %s -> %s (fingerprint %s)\n", inputPath, outputPath, signer.Fingerprint())
	return nil
}

// verifyForResign runs the configured verification step and reports whether the
// resign can be skipped (already valid under the current key, no --force).
func verifyForResign(cmd *cobra.Command, signer *backupsign.Signer, digest, oldSig []byte, opts *resignOptions) (skip bool, err error) {
	switch {
	case opts.verifyKey != "":
		pub, perr := loadVerifyKey(opts.verifyKey)
		if perr != nil {
			return false, perr
		}
		if verr := backupsign.VerifyDigestWithPublicKey(pub, digest, oldSig); verr != nil {
			return false, fmt.Errorf("existing signature does not verify against --verify-key: %w", verr)
		}
		fmt.Fprintln(cmd.OutOrStdout(), "existing signature verified against the provided key")
		return false, nil
	case opts.noVerify:
		// Explicitly skip the old-signature check.
		return false, nil
	default:
		// No external key: if the current key already verifies, it's a no-op
		// unless --force.
		if backupsign.VerifyDigestWithPublicKey(signer.PublicKey(), digest, oldSig) == nil && !opts.force {
			return true, nil
		}
		return false, nil
	}
}

// loadVerifyKey loads a public key from --verify-key: either an inline value
// (PEM/base64/hex) or, if the value names an existing file, the file's contents.
func loadVerifyKey(value string) ([]byte, error) {
	if info, statErr := os.Stat(value); statErr == nil && !info.IsDir() {
		data, err := os.ReadFile(value)
		if err != nil {
			return nil, fmt.Errorf("failed to read --verify-key file: %w", err)
		}
		return parsePub(data)
	}
	return parsePub([]byte(value))
}

func parsePub(data []byte) ([]byte, error) {
	pub, err := backupsign.ParsePublicKey(data)
	if err != nil {
		return nil, err
	}
	return pub, nil
}

// writeContainerAtomically writes the re-signed container to a temp file in the
// destination directory, then renames it over the target so a crash never
// leaves a half-written archive.
func writeContainerAtomically(outputPath string, sig []byte, payload io.Reader, payloadSize int64) (err error) {
	dir := filepath.Dir(outputPath)
	tmpOut, err := os.CreateTemp(dir, ".inventario-resign-*.inb")
	if err != nil {
		return fmt.Errorf("failed to create temp output: %w", err)
	}
	tmpOutName := tmpOut.Name()
	defer func() {
		if err != nil {
			_ = os.Remove(tmpOutName)
		}
	}()

	if werr := inb.WriteContainer(tmpOut, sig, payload, payloadSize); werr != nil {
		_ = tmpOut.Close()
		return fmt.Errorf("failed to write container: %w", werr)
	}
	if cerr := tmpOut.Close(); cerr != nil {
		return fmt.Errorf("failed to close temp output: %w", cerr)
	}

	if rerr := os.Rename(tmpOutName, outputPath); rerr != nil {
		return fmt.Errorf("failed to move output into place: %w", rerr)
	}
	return nil
}
