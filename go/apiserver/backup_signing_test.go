package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/backupsign"
	"github.com/denisvmedia/inventario/internal/checkers"
)

func TestBackupPublicKey(t *testing.T) {
	c := qt.New(t)

	params := apiserver.Params{BackupSigner: testBackupSigner}
	r := chi.NewRouter()
	r.Route("/backup", apiserver.BackupSigning(params))

	req := httptest.NewRequest(http.MethodGet, "/backup/public-key", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.algorithm"), backupsign.Algorithm)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.fingerprint"), testBackupSigner.Fingerprint())
	// The PEM block round-trips back to the same public key.
	c.Assert(rr.Body.Bytes(), checkers.JSONPathMatches("$.public_key", qt.Contains), "BEGIN PUBLIC KEY")
}
