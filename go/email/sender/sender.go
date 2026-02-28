package sender

import "context"

// Message is a normalized outbound email payload independent of any concrete
// transport provider.
//
// Provider implementations map this structure into their own request format
// (SMTP MIME body, SendGrid JSON payload, SES SDK request, etc.) while preserving
// semantic meaning across providers.
type Message struct {
	// To is the primary recipient email address.
	To string
	// From is the envelope/header sender used by the provider.
	From string
	// ReplyTo is optional and, when set, overrides where recipients reply.
	ReplyTo string
	// Subject is the human-readable message subject.
	Subject string
	// HTML is the rich-text body variant.
	HTML string
	// Text is the plain-text body variant used by text-only clients.
	Text string
}

// Sender defines the provider boundary used by higher-level orchestration code.
//
// Implementations are expected to perform one synchronous delivery attempt per
// call and return an error when the provider rejects the request or transport
// fails. Retry policy, backoff, and dead-letter behavior are intentionally
// managed by the caller (queue/orchestrator), not by this interface.
type Sender interface {
	// Send delivers a single outbound email message.
	Send(ctx context.Context, message Message) error
}
