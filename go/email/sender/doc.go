// Package sender defines the provider-agnostic email transport contract.
//
// Architecture:
//   - Higher-level application services (queueing, retries, templates) build a Message.
//   - A concrete provider implementation translates that Message to provider-specific
//     API calls (SMTP protocol, HTTP APIs, cloud SDKs).
//   - The provider boundary stays narrow (single Send method) so callers can swap
//     implementations without touching orchestration logic.
//
// This package intentionally does not include queueing, retries, or template
// rendering; those are handled upstream in service-layer orchestration.
package sender
