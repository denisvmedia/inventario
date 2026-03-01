// Package redis provides a Redis-backed csrf.Service implementation suitable
// for multi-instance deployments.
//
// It uses a Redis sorted set per user where each member is a CSRF token and
// its score is the token's expiry unix timestamp. This allows atomic pruning
// of expired tokens and enforcement of the rolling-window size limit via
// standard Redis ZSET commands.
package redis
