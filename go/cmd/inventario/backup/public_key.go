package backup

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/internal/backupsign"
)

// newPublicKeyCmd prints the backup signing public key (PEM) and its fingerprint
// so an operator can publish the key external verifiers use to check `.inb`
// archives without holding the private seed.
func newPublicKeyCmd() *cobra.Command {
	var signingKey string
	cmd := &cobra.Command{
		Use:   "public-key",
		Short: "Print the backup signing public key (PEM) and fingerprint",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			signer, err := loadSigner(signingKey)
			if err != nil {
				return err
			}
			pem, err := signer.PublicKeyPEM()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprint(out, string(pem))
			fmt.Fprintf(out, "fingerprint: %s\n", signer.Fingerprint())
			fmt.Fprintf(out, "algorithm:   %s\n", backupsign.Algorithm)
			return nil
		},
	}
	cmd.Flags().StringVar(&signingKey, "backup-signing-key", "", "Ed25519 seed (64 hex chars or 32 raw bytes); falls back to "+backupSigningKeyEnv)
	return cmd
}
