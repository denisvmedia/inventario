// Package sendgrid provides an HTTP-based sender implementation for the SendGrid
// Mail Send API.
//
// It maps sender.Message into SendGrid's JSON payload format and treats non-2xx
// HTTP responses as delivery failures to be handled by upstream retry logic.
package sendgrid
