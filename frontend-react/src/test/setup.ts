import "@testing-library/jest-dom/vitest"
import { afterAll, afterEach, beforeAll, expect } from "vitest"
import { cleanup } from "@testing-library/react"
import { toHaveNoViolations } from "jest-axe"

import { server } from "./server"
import { initI18n } from "@/i18n"

expect.extend(toHaveNoViolations)

// Boot i18n once for the whole suite. The en bundle is already in memory
// (statically imported by i18next.config.ts), so this resolves in a single
// microtask and every useTranslation() in test render returns real strings
// instead of the bare keys.
beforeAll(async () => {
  await initI18n({ lng: "en" })
  server.listen({ onUnhandledRequest: "error" })
})
afterEach(() => {
  cleanup()
  server.resetHandlers()
})
afterAll(() => server.close())
