import { MutationCache, QueryCache, QueryClient } from "@tanstack/react-query"

import { notifyGlobalServerError } from "./global-error-toast"
import { HttpError } from "./http"

// One QueryClient per app. Defaults match issue #1403:
//   - 30s staleTime so navigation between pages reuses cached data
//   - 1 retry, but never on auth or other 4xx (a 401 has already been
//     resolved or escalated by http.ts; retrying makes the redirect race)
// QueryCache/MutationCache onError surface 5xx responses globally (#1210)
// so silently failing requests still show the user something happened.
export function createQueryClient(): QueryClient {
  return new QueryClient({
    queryCache: new QueryCache({
      onError: (error, query) => notifyGlobalServerError(error, query.meta),
    }),
    mutationCache: new MutationCache({
      onError: (error, _vars, _ctx, mutation) => notifyGlobalServerError(error, mutation.meta),
    }),
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
