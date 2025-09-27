package middleware

import (
	"context"
	"net/http"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// Context keys for upload operation metadata
type contextKey string

const (
	uploadOperationKey contextKey = "upload_operation"
)

// SetUploadOperation creates middleware that sets the upload operation name in context
func SetUploadOperation(operationName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), uploadOperationKey, operationName)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUploadOperationFromContext retrieves the upload operation name from context
func GetUploadOperationFromContext(ctx context.Context) (string, bool) {
	operationName, ok := ctx.Value(uploadOperationKey).(string)
	return operationName, ok
}

// UploadLimiter creates middleware that enforces concurrent upload limits
func UploadLimiter(concurrentUploadService services.ConcurrentUploadService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user from context
			user, err := appctx.RequireUserFromContext(r.Context())
			if err != nil {
				// Let the handler deal with auth errors
				next.ServeHTTP(w, r)
				return
			}

			// Get operation name from context (set by SetUploadOperation middleware)
			operationName, ok := GetUploadOperationFromContext(r.Context())
			if !ok {
				// If no operation is set in context, this is not an upload endpoint
				next.ServeHTTP(w, r)
				return
			}

			// Check if user can start upload
			canStart, err := concurrentUploadService.CanStartUpload(r.Context(), user.ID, operationName)
			if err != nil {
				// Let the handler deal with service errors
				next.ServeHTTP(w, r)
				return
			}

			if !canStart {
				// Return 429 Too Many Requests
				http.Error(w, "Too many concurrent uploads. Please try again later.", http.StatusTooManyRequests)
				return
			}

			// Start the upload (increment counter)
			err = concurrentUploadService.StartUpload(r.Context(), user.ID, operationName)
			if err != nil {
				// If we can't start upload due to race condition, return 429
				if registry.ErrTooManyRequests.Error() == err.Error() { // TODO: errors should be compared with errors.Is
					http.Error(w, "Too many concurrent uploads. Please try again later.", http.StatusTooManyRequests)
					return
				}
				// Other errors, let handler deal with them
				next.ServeHTTP(w, r)
				return
			}

			// Create a custom response writer to track completion
			lrw := &limitedResponseWriter{
				ResponseWriter:          w,
				concurrentUploadService: concurrentUploadService,
				userID:                  user.ID,
				operationName:           operationName,
				finished:                false,
			}

			// Ensure we finish the upload when the request completes
			defer lrw.finishUpload(r)

			// Continue with the request
			next.ServeHTTP(lrw, r)
		})
	}
}

// limitedResponseWriter wraps http.ResponseWriter to track upload completion
type limitedResponseWriter struct {
	http.ResponseWriter
	concurrentUploadService services.ConcurrentUploadService
	userID                  string
	operationName           string
	finished                bool
}

// finishUpload decrements the upload counter
func (lrw *limitedResponseWriter) finishUpload(r *http.Request) {
	if !lrw.finished {
		lrw.finished = true
		_ = lrw.concurrentUploadService.FinishUpload(r.Context(), lrw.userID, lrw.operationName)
	}
}
