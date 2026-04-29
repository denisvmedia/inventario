import path from "node:path"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vitest/config"

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    include: ["src/**/*.{test,spec}.{ts,tsx}"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json", "html"],
      include: ["src/**/*.{ts,tsx}"],
      exclude: [
        // Tests themselves and their fixtures.
        "src/**/*.{test,spec}.{ts,tsx}",
        "src/test/**",
        // App entry + module-glue files: no logic, full coverage would
        // require real-DOM mounting that's already exercised by the
        // Playwright suite (#1419).
        "src/main.tsx",
        "src/app/**",
        "src/vite-env.d.ts",
        // Generated TypeScript types — nothing to cover.
        "src/types/**",
        // Vendored shadcn primitives. The design-mock repo is the upstream
        // owner and validates them; covering each Radix wrapper here would
        // be busywork that drifts every time we resync the mock.
        "src/components/ui/**",
        // The composite shell components (sidebar + command palette) are
        // tested via the higher-level Shell render in #1419; covering them
        // unit-style requires mounting the full Auth/Group/Sidebar provider
        // stack with MSW, which #1418 sets up but doesn't enforce on these
        // specific surfaces. They'll be covered as feature pages start
        // exercising the shell in their own integration tests.
        "src/components/AppSidebar.tsx",
        "src/components/CommandPalette.tsx",
        "src/components/GroupSelector.tsx",
        // i18n config's lazy backend resolves cs/ru via dynamic import; the
        // path is a one-liner exercised by the Settings page locale toggle
        // (#1414). Until then, the eager en bundle path is what coverage
        // reflects.
        "src/i18n/i18next.config.ts",
        // codegen plumbing.
        "src/i18n/index.ts",
      ],
      // Coverage gate per #1418 AC. Branches sit one rung lower because
      // a lot of the codebase's branches are defensive null-fallbacks
      // (`?? null`, optional chains in fixtures) that don't carry their
      // weight in tests.
      thresholds: {
        lines: 80,
        functions: 80,
        statements: 80,
        branches: 70,
      },
    },
  },
})
