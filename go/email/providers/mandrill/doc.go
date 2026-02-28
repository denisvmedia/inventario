// Package mandrill provides an HTTP-based sender implementation for Mailchimp
// Transactional (Mandrill).
//
// It translates sender.Message into Mandrill's /messages/send payload and
// surfaces provider-level failures via Send errors so retries remain centralized
// in orchestration code.
package mandrill
