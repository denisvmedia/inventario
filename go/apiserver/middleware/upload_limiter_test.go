package middleware_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/apiserver/middleware"
	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// fakeUploadService records what the middleware asked of it and answers
// however the test needs. Counting FinishUpload matters as much as the
// status code: a slot that is taken and never released locks the user out
// of uploads until the record expires.
type fakeUploadService struct {
	canStart    bool
	canStartErr error
	startErr    error
	finishErr   error

	canStartCalls int
	startCalls    int
	finishCalls   int
	lastUserID    string
	lastOperation string
}

func (f *fakeUploadService) CanStartUpload(_ context.Context, userID, operation string) (bool, error) {
	f.canStartCalls++
	f.lastUserID = userID
	f.lastOperation = operation
	return f.canStart, f.canStartErr
}

func (f *fakeUploadService) StartUpload(_ context.Context, _, _ string) error {
	f.startCalls++
	return f.startErr
}

func (f *fakeUploadService) FinishUpload(_ context.Context, _, _ string) error {
	f.finishCalls++
	return f.finishErr
}

// Unused by the middleware; present to satisfy the interface.
func (f *fakeUploadService) GetUploadStatus(_ context.Context, _, _ string) (*models.UploadStatus, error) {
	return nil, nil
}

func (f *fakeUploadService) GetOperationConfig(_ string) models.OperationSlotConfig {
	return models.OperationSlotConfig{}
}

const (
	testUserID    = "user-1"
	testOperation = "file-upload"
)

// serve runs the middleware chain the way the router composes it and reports
// whether the handler behind it was reached. An empty userID leaves the
// request unauthenticated; an empty operation leaves SetUploadOperation out of
// the chain, which is what a route that is not an upload endpoint looks like.
func serve(svc *fakeUploadService, userID, operation string) (*httptest.ResponseRecorder, bool) {
	reached := false
	var handler http.Handler = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached = true
		w.WriteHeader(http.StatusOK)
	})
	handler = middleware.UploadLimiter(svc)(handler)
	if operation != "" {
		handler = middleware.SetUploadOperation(operation)(handler)
	}

	req := httptest.NewRequest(http.MethodPost, "/uploads/file", nil)
	if userID != "" {
		user := &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: userID},
				TenantID: "tenant-1",
			},
		}
		req = req.WithContext(appctx.WithUser(req.Context(), user))
	}

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr, reached
}

func TestUploadLimiter_AllowsAndReleasesTheSlot(t *testing.T) {
	c := qt.New(t)
	svc := &fakeUploadService{canStart: true}

	rr, reached := serve(svc, testUserID, testOperation)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(reached, qt.IsTrue)
	c.Assert(svc.startCalls, qt.Equals, 1)
	// Exactly once. A leaked slot is not visible in this response — it is
	// visible the next time the user tries to upload anything.
	c.Assert(svc.finishCalls, qt.Equals, 1)
	c.Assert(svc.lastUserID, qt.Equals, testUserID)
	c.Assert(svc.lastOperation, qt.Equals, testOperation)
}

func TestUploadLimiter_RefusesWhenOverTheCap(t *testing.T) {
	c := qt.New(t)
	svc := &fakeUploadService{canStart: false}

	rr, reached := serve(svc, testUserID, testOperation)

	c.Assert(rr.Code, qt.Equals, http.StatusTooManyRequests)
	c.Assert(reached, qt.IsFalse)
	c.Assert(svc.startCalls, qt.Equals, 0)
	// Nothing was taken, so nothing may be released — a decrement here
	// would hand the user a free slot every time they hit the cap.
	c.Assert(svc.finishCalls, qt.Equals, 0)
}

// The pre-check and the atomic check are two different guards. Two requests
// can both pass CanStartUpload; StartUpload is what catches the second, and
// it reports it by wrapping the sentinel. Comparing messages instead of
// unwrapping let that request through above the cap (#2531).
func TestUploadLimiter_RefusesWhenStartLosesTheRace(t *testing.T) {
	c := qt.New(t)
	svc := &fakeUploadService{
		canStart: true,
		startErr: errxtrace.Wrap("maximum concurrent uploads reached", registry.ErrTooManyRequests),
	}

	rr, reached := serve(svc, testUserID, testOperation)

	c.Assert(rr.Code, qt.Equals, http.StatusTooManyRequests)
	c.Assert(reached, qt.IsFalse)
	c.Assert(svc.finishCalls, qt.Equals, 0)
}

// Everything below is a pass-through: the limiter refuses to be the component
// that answers a request it cannot reason about.
func TestUploadLimiter_PassesThroughWithoutAUser(t *testing.T) {
	c := qt.New(t)
	svc := &fakeUploadService{canStart: true}

	rr, reached := serve(svc, "", testOperation)

	// Auth is the handler's business, not the limiter's.
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(reached, qt.IsTrue)
	c.Assert(svc.canStartCalls, qt.Equals, 0)
}

func TestUploadLimiter_PassesThroughOnANonUploadRoute(t *testing.T) {
	c := qt.New(t)
	svc := &fakeUploadService{canStart: true}

	// No SetUploadOperation in the chain: this is not an upload endpoint.
	rr, reached := serve(svc, testUserID, "")

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(reached, qt.IsTrue)
	c.Assert(svc.canStartCalls, qt.Equals, 0)
}

func TestUploadLimiter_PassesThroughWhenTheServiceIsBroken(t *testing.T) {
	c := qt.New(t)

	// A service that cannot answer must not become a 429: that would turn a
	// backend fault into "you upload too much" for every user at once.
	checkFailed := &fakeUploadService{canStartErr: errors.New("redis is down")}
	rr, reached := serve(checkFailed, testUserID, testOperation)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(reached, qt.IsTrue)

	startFailed := &fakeUploadService{
		canStart: true,
		startErr: errors.New("redis is down"),
	}
	rr, reached = serve(startFailed, testUserID, testOperation)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(reached, qt.IsTrue)
	// No slot was taken, so none is released.
	c.Assert(startFailed.finishCalls, qt.Equals, 0)
}

func TestSetUploadOperation_RoundTripsThroughContext(t *testing.T) {
	c := qt.New(t)

	var got string
	var ok bool
	handler := middleware.SetUploadOperation(testOperation)(http.HandlerFunc(
		func(_ http.ResponseWriter, r *http.Request) {
			got, ok = middleware.GetUploadOperationFromContext(r.Context())
		},
	))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/uploads/file", nil))

	c.Assert(ok, qt.IsTrue)
	c.Assert(got, qt.Equals, testOperation)

	_, ok = middleware.GetUploadOperationFromContext(context.Background())
	c.Assert(ok, qt.IsFalse)
}
