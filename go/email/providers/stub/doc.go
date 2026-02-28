// Package stub provides a no-op sender used to exercise orchestration paths
// (templating, queueing, retries, handler wiring) without external email
// dependencies.
//
// It logs metadata and always reports success, making it suitable for local
// development and unit tests where delivery side effects are undesirable.
package stub
