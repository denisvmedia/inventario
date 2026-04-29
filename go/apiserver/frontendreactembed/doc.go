// Package frontendreactembed contains a smoke test for the React frontend
// embed (frontend-react/frontend.go).
//
// It lives in its own package so the smoke test compiles in isolation —
// without dragging in the apiserver package's apiserver_with_frontend.go,
// which has its own //go:embed dependency on the legacy frontend bundle.
// That way the React-bundle embed smoke workflow only needs frontend-react/
// to be built, and the legacy embed smoke workflow only needs frontend/.
//
// All test logic lives in embed_test.go behind the //go:build with_frontend
// tag; this file exists so the package is always defined for `go list ./...`.
package frontendreactembed
