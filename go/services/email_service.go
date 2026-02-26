package services

import "log/slog"

// EmailService defines the interface for sending transactional emails.
// A real implementation will be provided in Phase 3; this package ships a
// logging stub so that Phase 2 registration flows can be wired end-to-end
// without an actual mail server.
type EmailService interface {
	// SendVerificationEmail sends an account-activation link to the given address.
	SendVerificationEmail(to, name, verificationURL string) error

	// SendPasswordResetEmail sends a password-reset link to the given address.
	SendPasswordResetEmail(to, name, resetURL string) error
}

// StubEmailService is a no-op EmailService that logs every call.
// It is safe for use in development and testing.
type StubEmailService struct{}

// NewStubEmailService returns a new StubEmailService.
func NewStubEmailService() *StubEmailService {
	return &StubEmailService{}
}

// SendVerificationEmail logs the verification URL instead of sending an email.
func (s *StubEmailService) SendVerificationEmail(to, name, verificationURL string) error {
	slog.Info("STUB email: verification link",
		"to", to,
		"name", name,
		"url", verificationURL,
	)
	return nil
}

// SendPasswordResetEmail logs the reset URL instead of sending an email.
func (s *StubEmailService) SendPasswordResetEmail(to, name, resetURL string) error {
	slog.Info("STUB email: password-reset link",
		"to", to,
		"name", name,
		"url", resetURL,
	)
	return nil
}

