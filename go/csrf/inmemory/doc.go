// Package inmemory provides a process-local csrf.Service implementation.
//
// It is intended for development and single-process deployments. State is not
// shared across instances and is lost on restart. Use csrf/redis for
// production multi-instance setups.
package inmemory
