import type { ReactNode } from "react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { MemoryRouter, Routes } from "react-router-dom"
import { render, type RenderResult } from "@testing-library/react"

// renderWithProviders is the standard wrapper for guard / provider tests:
// per-test QueryClient (so cache + retry never leak), MemoryRouter so
// tests pick the initial path, and a passthrough <Routes> so callers can
// hand in their own route tree.
//
// Tests that need just one of these helpers can still drop down to
// @testing-library/react directly.
interface RenderWithProvidersOptions {
  // Initial URL the MemoryRouter starts at. Defaults to "/".
  initialPath?: string
  // Routes to render. Receives the full <Routes> children. Required.
  routes: ReactNode
  // Optional preload of the QueryClient cache so tests can seed e.g.
  // current-user data without going through MSW for every case.
  queryClient?: QueryClient
}

export function renderWithProviders({
  initialPath = "/",
  routes,
  queryClient,
}: RenderWithProvidersOptions): RenderResult & { queryClient: QueryClient } {
  const client =
    queryClient ??
    new QueryClient({
      defaultOptions: {
        queries: { retry: false, staleTime: 0 },
        mutations: { retry: false },
      },
    })
  const utils = render(
    <QueryClientProvider client={client}>
      <MemoryRouter initialEntries={[initialPath]}>
        <Routes>{routes}</Routes>
      </MemoryRouter>
    </QueryClientProvider>
  )
  return { ...utils, queryClient: client }
}
