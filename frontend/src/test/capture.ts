/**
 * A box for a value written from inside a request handler.
 *
 * `let body: T | null = null` reads as `null` at every later assertion:
 * TypeScript narrows the variable to its initializer and has no way to see
 * the handler run, so each property access resolves against `never`. The
 * declared return type here is opaque to that narrowing, and the assertion
 * reads the shape the test actually cares about.
 */
export function capture<T>(): { value?: T } {
  return {}
}
