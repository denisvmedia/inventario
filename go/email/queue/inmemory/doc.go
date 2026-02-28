// Package inmemory provides a process-local queue.Queue implementation.
//
// It is intended for tests and single-process development environments. State is
// not shared across instances and is lost on restart.
package inmemory
