// jest-axe ships its matcher typing as an augmentation of jest's `Matchers`,
// so vitest's `expect` never picks it up even though setup.ts registers the
// matcher at runtime. Declared here instead.
import "vitest"

interface AxeMatchers<R = unknown> {
  toHaveNoViolations(): R
}

declare module "vitest" {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  interface Assertion<T = any> extends AxeMatchers<T> {}
  interface AsymmetricMatchersContaining extends AxeMatchers {}
}
