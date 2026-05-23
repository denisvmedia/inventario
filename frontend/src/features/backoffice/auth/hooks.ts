import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import {
  backofficeCompleteMFA,
  backofficeLogin,
  backofficeLogout,
  getBackofficeMe,
  type BackofficeCompleteLoginMFARequest,
  type BackofficeLoginOutcome,
  type BackofficeUser,
} from "./api"
import { backofficeAuthKeys } from "./keys"
import { getBackofficeAccessToken } from "./storage"

interface LoginVars {
  email: string
  password: string
}

// useBackofficeMe reads the authenticated operator. The query only runs
// when a back-office access token is present in localStorage — without
// one the BE 401s, the http client tries to refresh on the back-office
// plane, and (in the cold-boot case) it fails because no refresh cookie
// is set yet. Skipping the query when the token is absent lets the
// guard's <Navigate> handle the redirect cleanly. Mirrors useCurrentUser.
export function useBackofficeMe() {
  return useQuery<BackofficeUser>({
    queryKey: backofficeAuthKeys.me(),
    queryFn: ({ signal }) => getBackofficeMe(signal),
    enabled: !!getBackofficeAccessToken(),
  })
}

// useBackofficeLogin posts the credentials. On a happy login the cached
// operator is seeded so the guard can short-circuit the boot probe.
// `mfaRequired` and `mfaNotEnrolled` outcomes are passed through to the
// page without touching the cache — the page swaps into the MFA surface
// (or the enrollment-missing nudge) and the second-step mutation seeds
// the cache only after it lands.
export function useBackofficeLogin() {
  const queryClient = useQueryClient()
  return useMutation<BackofficeLoginOutcome, Error, LoginVars>({
    mutationFn: ({ email, password }) => backofficeLogin(email, password),
    onSuccess: (outcome) => {
      if (outcome.kind === "ok") {
        if (outcome.user) {
          queryClient.setQueryData(backofficeAuthKeys.me(), outcome.user)
        }
        queryClient.invalidateQueries({ queryKey: backofficeAuthKeys.all })
      }
    },
  })
}

// useBackofficeCompleteMFA mirrors useBackofficeLogin for step-2: it
// consumes the mfa_token from step-1 + the TOTP/backup code, persists
// the resulting tokens, and seeds the operator cache. Treat the resolved
// promise as "you are now signed into the back-office plane".
export function useBackofficeCompleteMFA() {
  const queryClient = useQueryClient()
  return useMutation<BackofficeUser | undefined, Error, BackofficeCompleteLoginMFARequest>({
    mutationFn: (req) => backofficeCompleteMFA(req),
    onSuccess: (user) => {
      if (user) {
        queryClient.setQueryData(backofficeAuthKeys.me(), user)
      }
      queryClient.invalidateQueries({ queryKey: backofficeAuthKeys.all })
    },
  })
}

// useBackofficeLogout is the symmetric counterpart to useLogout for the
// back-office plane. Optimistic cache wipe so the UI flips back to the
// logged-out state immediately. On error we restore the snapshot.
export function useBackofficeLogout() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, void, { previousUser: BackofficeUser | undefined }>({
    mutationFn: () => backofficeLogout(),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: backofficeAuthKeys.me() })
      const previousUser = queryClient.getQueryData<BackofficeUser>(backofficeAuthKeys.me())
      queryClient.removeQueries({ queryKey: backofficeAuthKeys.me(), exact: true })
      return { previousUser }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.previousUser) {
        queryClient.setQueryData(backofficeAuthKeys.me(), ctx.previousUser)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: backofficeAuthKeys.all })
    },
  })
}
