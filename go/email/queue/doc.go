// Package queue defines provider-agnostic contracts for reliable payload
// delivery with delayed retries.
//
// Architecture:
//   - Callers enqueue opaque payload bytes representing domain-specific jobs.
//   - Workers dequeue ready payloads for processing.
//   - Failed payloads are scheduled for retry and promoted later.
//
// The package intentionally avoids domain coupling (email templates/providers);
// orchestration layers own payload schema and retry policy.
package queue
