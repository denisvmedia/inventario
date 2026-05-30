package apiserver

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-extras/errx"

	"github.com/denisvmedia/inventario/internal/backupsign"
)

// errBackupSignerUnavailable is returned if the public-key handler is reached
// without a configured signer. This is unreachable in a validated deployment
// (Params.Validate requires BackupSigner) but guards against a misuse.
var errBackupSignerUnavailable = errx.NewSentinel("backup signing key is not configured")

// BackupPublicKeyResponse is the payload of GET /api/v1/backup/public-key. It
// surfaces the server's backup-signing PUBLIC key (and only the public key) so
// external tooling can verify a `.inb` archive's signature without being able to
// mint one (issue #534).
type BackupPublicKeyResponse struct {
	// PublicKey is the PKIX-encoded PEM public key ("PUBLIC KEY" block) — the
	// form openssl and Go's x509.ParsePKIXPublicKey expect.
	PublicKey string `json:"public_key"`
	// Fingerprint is the lowercase hex SHA-256 of the raw public key, a stable
	// short identifier for the signing key (useful around key rotation).
	Fingerprint string `json:"fingerprint"`
	// Algorithm names the hash-then-sign construction ("ed25519-sha256").
	Algorithm string `json:"algorithm"`
}

// BackupSigning mounts the authenticated, server-global backup-signing routes
// (#534). It is NOT group-scoped: the signing key is a deployment-wide secret,
// and any authenticated user may read the public half so the frontend / CLI can
// verify downloaded archives.
//
// @Summary Get backup signing public key
// @Description Returns the server's backup-signing public key (PEM), its fingerprint, and the signing algorithm.
// @Description The private key is never exposed — only the public half, so external tooling can verify a `.inb`
// @Description archive's signature without being able to forge one (#534).
// @Tags system
// @Produce json
// @Success 200 {object} BackupPublicKeyResponse "OK"
// @Failure 500 {object} jsonapi.Errors "Internal Server Error"
// @Router /backup/public-key [get].
func BackupSigning(params Params) func(r chi.Router) {
	return func(r chi.Router) {
		r.Get("/public-key", backupPublicKeyHandler(params.BackupSigner))
	}
}

// backupPublicKeyHandler renders the backup-signing public key. The signer is a
// required Param (validated at boot), so it is never nil here; the guard is
// belt-and-suspenders.
func backupPublicKeyHandler(signer *backupsign.Signer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if signer == nil {
			internalServerError(w, r, errBackupSignerUnavailable)
			return
		}
		pem, err := signer.PublicKeyPEM()
		if err != nil {
			internalServerError(w, r, err)
			return
		}
		resp := BackupPublicKeyResponse{
			PublicKey:   string(pem),
			Fingerprint: signer.Fingerprint(),
			Algorithm:   backupsign.Algorithm,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}
}
