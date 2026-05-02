import { QueryClient } from "@tanstack/react-query"

import { HttpError } from "./http"

// One QueryClient per app. Defaults match issue #1403:
//   - 30s staleTime so navigation between pages reuses cached data
//   - 1 retry, but never on auth or other 4xx (a 401 has already been
//     resolved or escalated by http.ts; retrying makes the redirect race)
export function createQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        retry: (failureCount, error) => {
          if (error instanceof HttpError && error.status >= 400 && error.status < 500) {
            return false
          }
          return failureCount < 1
        },
        refetchOnWindowFocus: true,
      },
      mutations: {
        retry: false,
      },
    },
  })
}
