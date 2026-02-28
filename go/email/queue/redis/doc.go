// Package redis provides a Redis-backed queue.Queue implementation suitable for
// multi-instance deployments.
//
// It uses:
//   - Redis list for ready payloads,
//   - Redis sorted set for delayed retry payloads.
package redis
