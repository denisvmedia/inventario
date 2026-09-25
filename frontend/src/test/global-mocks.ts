import { vi } from "vitest"

// Spies for the modules test/setup.ts mocks for the whole suite.
//
// They are parked on globalThis rather than held in module scope. setup.ts is
// a setup file, so it runs once per test file; with a shared module registry
// (see vitest.config.ts) the components under test keep the mocked module
// from the first evaluation while a later evaluation would build a new one.
// Reusing the same spy functions across evaluations keeps both sides looking
// at the same object.
//
// A test that wants to assert on a toast, or to program a password score,
// imports the spies from here instead of re-mocking the module locally. Two
// files mocking one module differently is what a shared registry cannot
// resolve, and the local mock is the thing that breaks.

type Spy = ReturnType<typeof vi.fn>
// The zxcvbn spies stand in for a constructor call and a method call, so they
// are typed by their signatures rather than by vi.fn's default, which widens
// to "callable or constructable" and then refuses a plain call.
type ArgsSpy = ReturnType<typeof vi.fn<(...args: unknown[]) => void>>
type CheckSpy = ReturnType<
  typeof vi.fn<(...args: unknown[]) => { score: number; feedback: { suggestions: string[] } }>
>

interface GlobalMocks {
  toast: Record<string, Spy>
  zxcvbn: { factory: ArgsSpy; check: CheckSpy }
}

const KEY = "__inventarioGlobalMocks"

function create(): GlobalMocks {
  const toastSpy = () => vi.fn(() => "stub-toast-id")
  return {
    toast: {
      base: toastSpy(),
      success: toastSpy(),
      error: toastSpy(),
      info: toastSpy(),
      warning: toastSpy(),
      message: toastSpy(),
      promise: toastSpy(),
      loading: toastSpy(),
      custom: toastSpy(),
      dismiss: vi.fn(),
    },
    zxcvbn: {
      factory: vi.fn<(...args: unknown[]) => void>(),
      // The default answer: a weak password with nothing to say about it.
      // PasswordStrengthMeter's own tests reset this and program their own.
      check: vi.fn<(...args: unknown[]) => { score: number; feedback: { suggestions: string[] } }>(
        () => ({ score: 0, feedback: { suggestions: [] } })
      ),
    },
  }
}

const store = globalThis as typeof globalThis & { [KEY]?: GlobalMocks }
store[KEY] ??= create()

export const globalMocks: GlobalMocks = store[KEY]
export const toastSpies = globalMocks.toast
export const zxcvbnSpies = globalMocks.zxcvbn

/** Drops the call history of every spy. Called from the suite's afterEach. */
export function resetGlobalMockSpies(): void {
  for (const spy of Object.values(globalMocks.toast)) {
    spy.mockClear()
  }
  globalMocks.zxcvbn.factory.mockClear()
  globalMocks.zxcvbn.check.mockClear()
  globalMocks.zxcvbn.check.mockImplementation(() => ({ score: 0, feedback: { suggestions: [] } }))
}
