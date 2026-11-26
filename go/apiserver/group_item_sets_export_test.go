package apiserver

import (
	"testing"
	"time"
)

// SetCollectionSetSetupTimeoutForTest shortens the deadline of adding the
// Collection set to a new group for one test, restoring it afterwards. It
// compiles only under `go test`.
func SetCollectionSetSetupTimeoutForTest(t *testing.T, d time.Duration) {
	t.Helper()
	prev := collectionSetSetupTimeout
	collectionSetSetupTimeout = d
	t.Cleanup(func() { collectionSetSetupTimeout = prev })
}
