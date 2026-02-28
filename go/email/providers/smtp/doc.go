// Package smtp provides a sender implementation that talks directly to SMTP
// servers using the standard net/smtp client.
//
// It builds multipart/alternative MIME messages (text + HTML), optionally
// upgrades the connection with STARTTLS, and performs one synchronous delivery
// attempt per Send call.
package smtp
