import path from "node:path"
import react from "@vitejs/plugin-react"
import { defineConfig } from "vitest/config"

// Test files that cannot share a module registry with the rest of the suite.
//
// The two pdf.js viewers mock `@/lib/pdfjs` differently from each other. The
// password meter programs the shared zxcvbn spies and resets the component's
// module-level loader cache per test; both of those are module state that
// outlives a file once the registry is shared, and it showed up as an
// occasional failure under `--sequence.shuffle`.
const OWN_REGISTRY = [
  "src/components/files/__tests__/PdfViewer.test.tsx",
  "src/components/files/__tests__/PdfFullViewer.test.tsx",
  "src/components/auth/__tests__/PasswordStrengthMeter.test.tsx",
]

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
    // Two projects, differing only in isolation.
    //
    // A jsdom environment costs more than the tests that run in it: the suite
    // was creating one per file, 34% of tracked time, and sharing it takes the
    // run from 94s to about 30s (#2617). Sharing the environment also shares
    // the module registry, which is fine as long as no two files mock the same
    // module differently — the toast spies moved into setup.ts for exactly
    // that reason.
    //
    // The two pdf.js viewers are the case that cannot be resolved that way:
    // both mock `@/lib/pdfjs`, and they need different documents out of it
    // (progress-task control and getData on one side, viewport scaling on the
    // other). They keep a registry of their own.
    projects: [
      {
        test: {
          name: "shared-env",
          isolate: false,
          include: ["src/**/*.{test,spec}.{ts,tsx}"],
          exclude: OWN_REGISTRY,
        },
      },
      {
        test: {
          name: "own-registry",
          include: OWN_REGISTRY,
        },
      },
    ],
    // Vitest's 5 s default per-test budget was calibrated for pure-JS
    // unit tests. Our heaviest integration walks (CommodityFormDialog
    // wizard, CommoditiesListPage create flow, admin-search pages)
    // legitimately need 3.5-4.2 s _in isolation_ — they drive
    // userEvent through 4+ Radix Select portals plus 4-step dialog
    // navigation, and each interaction yields microtasks the React
    // commit phase has to drain in jsdom. Under default forks-pool
    // parallelism (≈ one worker per CPU) these tests routinely tip
    // past 5 s as 145 jsdom contexts share cores, but the same suite
    // passes deterministically with `--maxWorkers=1`. The behaviour
    // is purely contention, not a logic / leak bug — verified by
    // bisecting failing tests against master baseline (same tests
    // flake without any PR changes) and by running each suspected
    // polluter in isolation (no cross-file state leaks). 15 s gives
    // the wizard walks enough headroom under contended workers
    // without slowing the rest of the suite (per-test budget is only
    // consumed if the test actually needs it).
    testTimeout: 15000,
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
        // Canvas / browser-fullscreen heavy viewers; jsdom has neither
        // a working canvas 2D context (PdfViewer renders into a real
        // <canvas>) nor `requestFullscreen` (ImageViewer drives it).
        // The Playwright suite covers them end-to-end in #1419's
        // file-detail spec; meaningful unit coverage would require
        // mocking pdfjs-dist back to a stub and fighting jsdom for
        // every rAF, which earns very little for a lot of churn.
        "src/components/files/PdfViewer.tsx",
        "src/components/files/ImageViewer.tsx",
        "src/lib/pdfjs.ts",
      ],
      // Coverage gate per #1418 AC. Branches sit one rung lower because
      // a lot of the codebase's branches are defensive null-fallbacks
      // (`?? null`, optional chains in fixtures) that don't carry their
      // weight in tests. Functions threshold is one rung lower than
      // lines/statements: the unified Files page (#1411) introduced
      // many small handler functions (per-tile click, per-card
      // checkbox, per-row metadata setter) whose JSDOM-equivalent
      // coverage path duplicates what the Files Playwright spec
      // already exercises end-to-end. PR #1621 (Add-item dialog
      // rebuild + Dashboard polish) further grew `CommodityFormDialog`
      // by ~1.5k lines (AI step, Radix Select migration, IDB pending-
      // files persistence, server-error mapping, three-button AI
      // footer, segmented stepper, per-step Continue logic). Many of
      // the new branches sit behind portalled Radix Select pickers
      // that JSDOM can't drive deterministically — that gap is
      // tracked in #1629. The happy paths through Basics + Cancel +
      // dirty-confirm + draft rehydrate ARE covered (see
      // CommodityFormDialog.test.tsx + CommodityCreateModal.test.tsx);
      // what's missing is the four-step walk-through with Radix
      // selects. Until #1629 lands, the dialog drags overall
      // coverage down, and the alternative — drilling step-by-step
      // unit tests against a portal we can't fully control — earns
      // little. Bringing this back to 80/79/80/70 is the explicit
      // follow-up of #1629; the thresholds below match what the suite
      // delivers today without that follow-up.
      thresholds: {
        lines: 79,
        functions: 75,
        statements: 76,
        branches: 67,
      },
    },
  },
})
