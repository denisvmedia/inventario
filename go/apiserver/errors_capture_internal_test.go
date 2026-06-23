package apiserver

import (
	"net/http"
	"testing"

	qt "github.com/frankban/quicktest"
)

// TestShouldCaptureStatus pins the #844 capture boundary: only 5xx are reported
// to Sentry; 4xx are expected business outcomes (validation, auth, not-found)
// and must NOT be captured, or every such error would page. A future refactor
// that flips the comparison, or a business error newly mapped to 500, is caught
// here.
func TestShouldCaptureStatus(t *testing.T) {
	c := qt.New(t)

	for _, code := range []int{
		http.StatusBadRequest, http.StatusUnauthorized, http.StatusForbidden,
		http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity,
		http.StatusTooManyRequests,
	} {
		c.Assert(shouldCaptureStatus(code), qt.IsFalse, qt.Commentf("status %d must NOT be captured", code))
	}
	for _, code := range []int{
		http.StatusInternalServerError, http.StatusNotImplemented,
		http.StatusBadGateway, http.StatusServiceUnavailable,
	} {
		c.Assert(shouldCaptureStatus(code), qt.IsTrue, qt.Commentf("status %d must be captured", code))
	}
}
