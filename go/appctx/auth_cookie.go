package appctx

// RefreshTokenCookieName is the cookie that carries the refresh token. It
// lives here (instead of inside apiserver) so other packages — notably
// services/file_signing_service.go, which derives the signed-URL session
// binding from this cookie — can reference it without creating an
// `apiserver` import cycle.
const RefreshTokenCookieName = "refresh_token"
