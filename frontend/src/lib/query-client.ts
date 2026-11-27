import { MutationCache, QueryCache, QueryClient, type Mutation } from "@tanstack/react-query"

import { authIdentityGeneration } from "./auth-storage"
import { notifyGlobalServerError } from "./global-error-toast"
import { HttpError } from "./http"

// Set `meta: { changesIdentity: true }` on a mutation that signs a user in or
// out itself (sign-in, sign-out, impersonation). Its callbacks have to run
// after the change it made; every other mutation's callbacks are dropped when
// the signed-in user changes while it runs (see below).
export interface AuthIdentityMeta {
  changesIdentity?: boolean
}

type AnyMutation = Mutation<unknown, unknown, unknown, unknown>

// A promise that never settles. TanStack awaits the cache-level onSuccess and
// onError before anything else that reacts to a finished mutation, so
// returning this from them stops the mutation there.
function never(): Promise<never> {
  return new Promise<never>(() => {})
}

// One QueryClient per app. Defaults match issue #1403:
//   - 30s staleTime so navigation between pages reuses cached data
//   - 1 retry, but never on auth or other 4xx (a 401 has already been
//     resolved or escalated by http.ts; retrying makes the redirect race)
// QueryCache/MutationCache onError surface 5xx responses globally (#1210)
// so silently failing requests still show the user something happened.
//
// A mutation that is still running when the signed-in user changes belongs to
// the previous user. AuthProvider clears the cache at the change, but the
// mutation keeps running, and its callbacks would write the previous user's
// data into the next user's cache (useUpdateProfile puts the saved profile
// into the cached current user, for one). So the mutation cache records the
// identity generation when a mutation starts, and when it finishes under
// another generation, its cache-level onSuccess and onError never resolve.
// TanStack awaits them first, so the hook's onSuccess, onError and onSettled,
// the callbacks passed to mutate(), and the mutateAsync() promise never run
// or settle either. The mutation stays pending for the screen that started it,
// which is a screen of the previous user.
export function createQueryClient(): QueryClient {
  const startedUnder = new WeakMap<AnyMutation, number>()
  const startedForAnotherUser = (mutation: AnyMutation): boolean => {
    if ((mutation.meta as AuthIdentityMeta | undefined)?.changesIdentity === true) return false
    const generation = startedUnder.get(mutation)
    return generation !== undefined && generation !== authIdentityGeneration()
  }

  return new QueryClient({
    queryCache: new QueryCache({
      onError: (error, query) => notifyGlobalServerError(error, query.meta),
    }),
    mutationCache: new MutationCache({
      onMutate: (_vars, mutation) => {
        startedUnder.set(mutation, authIdentityGeneration())
      },
      onSuccess: (_data, _vars, _ctx, mutation) =>
        startedForAnotherUser(mutation) ? never() : undefined,
      onError: (error, _vars, _ctx, mutation) => {
        if (startedForAnotherUser(mutation)) return never()
        notifyGlobalServerError(error, mutation.meta)
        return undefined
      },
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
