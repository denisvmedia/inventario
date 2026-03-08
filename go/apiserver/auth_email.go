package apiserver

import "time"

const (
	// detachedAuthEmailTimeout bounds detached auth-email sends that preserve
	// request-scoped values across registration and password-reset flows.
	// TODO(Phase 3): revisit this timeout once the real SMTP transport is implemented.
	detachedAuthEmailTimeout = 30 * time.Second
)
