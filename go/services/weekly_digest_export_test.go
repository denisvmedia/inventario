package services

import "context"

// RunWeeklyDigestTickForTest runs one tick of the worker without starting its
// goroutine, so a test can place the clock on a given instant and assert what
// that tick did. It compiles only under `go test`.
func RunWeeklyDigestTickForTest(w *WeeklyDigestWorker, ctx context.Context) {
	w.tick(ctx)
}
