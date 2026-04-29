import type { ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { render, type RenderResult } from "@testing-library/react"

import { ThemeProvider } from "@/components/theme-provider"
import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { DensityProvider } from "@/hooks/useDensity"

interface RenderWithProvidersBaseOptions {
  // Initial URL the MemoryRouter starts at. Defaults to "/".
  initialPath?: string
  // Optional preload of the QueryClient cache so tests can seed e.g.
  // current-user data without going through MSW for every case.
  queryClient?: QueryClient
  // Wrap the test subtree in <AuthProvider>. Defaults to false because
  // route-level tests typically want to mount AuthProvider INSIDE the
  // route element so the /auth/me probe fires only after navigation
  // resolves. Pass `true` for leaf-component tests that read useAuth()
  // without owning the route boundary.
  withAuth?: boolean
  // Wrap the test subtree in <GroupProvider>. Same motivation as
  // `withAuth` — most route-level tests nest GroupProvider per route so
  // useParams() resolves the right :groupSlug. Pass `true` for leaf
  // tests that read useCurrentGroup() without route plumbing.
  withGroup?: boolean
}

interface RoutesOption {
  // Routes to render. Receives the full <Routes> children. Use this
  // when the test wants to assert routing behavior (redirects, matches).
  routes: ReactNode
  children?: never
}

interface ChildrenOption {
  // Single subtree to render. Use this when the test renders a leaf
  // component without route-level concerns.
  children: ReactNode
  routes?: never
}

export type RenderWithProvidersOptions = RenderWithProvidersBaseOptions &
  (RoutesOption | ChildrenOption)

// renderWithProviders is the standard wrapper for component / hook / page
// tests in the new frontend. Composes the same provider stack the app
// uses at boot, in the same order, so a passing test maps to a passing
// production render:
//
//   ThemeProvider           — light/dark class on <html>
//   └─ DensityProvider     — data-density attribute
//      └─ QueryClientProvider — TanStack cache (per-test instance)
//         └─ MemoryRouter   — controllable initial URL
//            └─ AuthProvider — /auth/me probe + AuthContext
//               └─ GroupProvider — /groups query + GroupContext
//                  └─ test subtree
//
// MSW is started in `src/test/setup.ts`; tests register per-case
// handlers via `server.use(...authHandlers.signedIn(), ...)` from
// `src/test/handlers`.
//
// Variants (skipAuth, skipGroup) drop the corresponding layer for tests
// that don't need it — useful for components that don't read those
// contexts and would otherwise pull in unnecessary MSW dependencies.
export function renderWithProviders(
  options: RenderWithProvidersOptions
): RenderResult & { queryClient: QueryClient } {
  const {
    initialPath = "/",
    queryClient,
    withAuth = false,
    withGroup = false,
    routes,
    children,
  } = options

  // Per-test client: cache + retry settings never leak between cases.
  // `retry: false` is what makes assertion-on-error tests deterministic.
  const client =
    queryClient ??
    new QueryClient({
      defaultOptions: {
        queries: { retry: false, staleTime: 0 },
        mutations: { retry: false },
      },
    })

  let tree: ReactNode = routes ? <Routes>{routes}</Routes> : children
  if (withGroup) {
    tree = <GroupProvider>{tree}</GroupProvider>
  }
  if (withAuth) {
    tree = <AuthProvider>{tree}</AuthProvider>
  }

  const utils = render(
    <ThemeProvider defaultTheme="light" storageKey="test-theme">
      <DensityProvider defaultDensity="comfortable" storageKey="test-density">
        <QueryClientProvider client={client}>
          <MemoryRouter initialEntries={[initialPath]}>{tree}</MemoryRouter>
        </QueryClientProvider>
      </DensityProvider>
    </ThemeProvider>
  )
  return { ...utils, queryClient: client }
}

// Re-export `Route` so test files can write the route tree inline without
// importing react-router-dom alongside `@/test/render`.
export { Route }
