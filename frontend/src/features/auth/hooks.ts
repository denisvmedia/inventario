import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { getAccessToken } from "@/lib/auth-storage"

import {
  changePassword,
  completeLoginMFA,
  disableMFA,
  forgotPassword,
  getCurrentUser,
  getMFAStatus,
  login,
  logout,
  regenerateMFABackupCodes,
  register,
  resendVerification,
  resetPassword,
  startMFASetup,
  updateProfile,
  verifyEmail,
  verifyMFASetup,
  type ChangePasswordRequest,
  type CompleteLoginMFARequest,
  type CurrentUser,
  type DisableMFARequest,
  type LoginOutcome,
  type MFASetupBody,
  type MFAStatus,
  type RegisterRequest,
  type UpdateProfileRequest,
} from "./api"
import { authKeys } from "./keys"

// Reads the authenticated user. The query only runs when an access token is
// present in localStorage — without a token /auth/me would 401, http.ts would
// try to refresh, the refresh would also 401, and we'd race the route guard
// to /login. Skipping the query for the no-token case lets ProtectedRoute
// handle the redirect cleanly via <Navigate> instead.
export function useCurrentUser() {
  return useQuery<CurrentUser>({
    queryKey: authKeys.currentUser(),
    queryFn: ({ signal }) => getCurrentUser(signal),
    enabled: !!getAccessToken(),
  })
}

// Reports whether the signed-in user carries the `is_system_admin` flag.
// The flag rides on the /auth/me payload (models.User) — the admin BE
// foundation (#1745) added it. Returns `false` while the boot probe is
// still in flight or for any non-admin user, so callers can use it as a
// plain boolean gate (sidebar entry visibility, route guard) without a
// tri-state dance. Reads through useCurrentUser so it shares the same
// cached query as the rest of the auth slice.
export function useIsSystemAdmin(): boolean {
  const { data: user } = useCurrentUser()
  return user?.is_system_admin === true
}

// Optimistic logout: drop the cached user before the server confirms so the
// UI can show the logged-out shell immediately. We `removeQueries` rather
// than `setQueryData(..., null)` so the cache stays type-true to the query's
// declared `CurrentUser` shape — consumers see `data === undefined`, never a
// surprise `null`. If the request fails, restore the snapshot.
export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, void, { previousUser: CurrentUser | undefined }>({
    mutationFn: () => logout(),
    onMutate: async () => {
      await queryClient.cancelQueries({ queryKey: authKeys.currentUser() })
      const previousUser = queryClient.getQueryData<CurrentUser>(authKeys.currentUser())
      queryClient.removeQueries({ queryKey: authKeys.currentUser(), exact: true })
      return { previousUser }
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.previousUser) {
        queryClient.setQueryData(authKeys.currentUser(), ctx.previousUser)
      }
    },
    onSettled: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.all })
    },
  })
}

interface LoginVars {
  email: string
  password: string
}

// Login mutation: on success, seed the cached user so ProtectedRoute can
// short-circuit the boot probe and render the protected tree without a
// /auth/me round-trip. We invalidate the auth namespace afterward so any
// stale "user" data anywhere in the cache settles to the new identity.
//
// When the backend short-circuits with `mfa_required`, the mutation
// resolves with that variant (no tokens have been stored yet) — the
// page is expected to switch into the MFA-prompt sub-flow and resolve
// the second step with useCompleteLoginMFA below.
export function useLogin() {
  const queryClient = useQueryClient()
  return useMutation<LoginOutcome, Error, LoginVars>({
    mutationFn: ({ email, password }) => login(email, password),
    onSuccess: (outcome) => {
      if (outcome.kind === "ok") {
        if (outcome.user) {
          queryClient.setQueryData(authKeys.currentUser(), outcome.user)
        }
        queryClient.invalidateQueries({ queryKey: authKeys.all })
      }
    },
  })
}

// useCompleteLoginMFA mirrors useLogin for the second step: it consumes
// the mfa_token issued by step-1 + the TOTP/backup code, persists the
// resulting tokens, and seeds the user cache. Treat the resolved
// promise as "you are now signed in" — the page navigates onward.
export function useCompleteLoginMFA() {
  const queryClient = useQueryClient()
  return useMutation<CurrentUser | undefined, Error, CompleteLoginMFARequest>({
    mutationFn: (req) => completeLoginMFA(req),
    onSuccess: (user) => {
      if (user) {
        queryClient.setQueryData(authKeys.currentUser(), user)
      }
      queryClient.invalidateQueries({ queryKey: authKeys.all })
    },
  })
}

export function useRegister() {
  return useMutation<string, Error, RegisterRequest>({
    mutationFn: (req) => register(req),
  })
}

export function useVerifyEmail() {
  return useMutation<string, Error, string>({
    mutationFn: (token) => verifyEmail(token),
  })
}

export function useResendVerification() {
  return useMutation<string, Error, string>({
    mutationFn: (email) => resendVerification(email),
  })
}

export function useForgotPassword() {
  return useMutation<string, Error, string>({
    mutationFn: (email) => forgotPassword(email),
  })
}

interface ResetPasswordVars {
  token: string
  newPassword: string
}

export function useResetPassword() {
  return useMutation<string, Error, ResetPasswordVars>({
    mutationFn: ({ token, newPassword }) => resetPassword(token, newPassword),
  })
}

// Updates the authenticated user's profile (name + default_group_id).
// On success we set the new user object directly into the cache so the
// next /auth/me read short-circuits and consumers see the fresh values
// immediately, then invalidate the auth namespace so anything tied to
// the user identity (e.g. RootRedirect's default_group_id) refetches.
export function useUpdateProfile() {
  const queryClient = useQueryClient()
  return useMutation<CurrentUser, Error, UpdateProfileRequest>({
    mutationFn: (req) => updateProfile(req),
    onSuccess: (user) => {
      queryClient.setQueryData(authKeys.currentUser(), user)
      queryClient.invalidateQueries({ queryKey: authKeys.all })
    },
  })
}

// Changes the user's password. The backend invalidates every active
// session on success — call sites should treat the resolution as a
// "you are about to be logged out" cue (sign-out + /login redirect).
export function useChangePassword() {
  return useMutation<string, Error, ChangePasswordRequest>({
    mutationFn: (req) => changePassword(req),
  })
}

// --- MFA management hooks ------------------------------------------------

// useMFAStatus reads the user's enrollment state. SettingsPage uses it
// to flip the Active/Inactive badge in the Privacy & Security row.
// Cached alongside the user via `authKeys.mfaStatus()` so disable /
// regenerate / enroll mutations can invalidate it surgically.
export function useMFAStatus() {
  return useQuery<MFAStatus>({
    queryKey: authKeys.mfaStatus(),
    queryFn: ({ signal }) => getMFAStatus(signal),
  })
}

// useStartMFASetup mints a fresh TOTP secret. The mutation deliberately
// does NOT invalidate the status query — Setup leaves the row in an
// `enrollment_in_progress` state that Verify completes.
export function useStartMFASetup() {
  return useMutation<MFASetupBody, Error, void>({
    mutationFn: () => startMFASetup(),
  })
}

// useVerifyMFASetup completes enrollment and returns the issued backup
// codes. We invalidate the status query so the SettingsPage row re-reads
// and flips to Active.
export function useVerifyMFASetup() {
  const queryClient = useQueryClient()
  return useMutation<string[], Error, string>({
    mutationFn: (code) => verifyMFASetup(code),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.mfaStatus() })
    },
  })
}

// useDisableMFA wipes the row after password + code reverification.
// Same invalidation strategy as Verify.
export function useDisableMFA() {
  const queryClient = useQueryClient()
  return useMutation<void, Error, DisableMFARequest>({
    mutationFn: (req) => disableMFA(req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.mfaStatus() })
    },
  })
}

// useRegenerateMFABackupCodes mints a fresh set, invalidating any
// previously-issued unused codes. Returns the new codes for the page
// to show once.
export function useRegenerateMFABackupCodes() {
  const queryClient = useQueryClient()
  return useMutation<string[], Error, string>({
    mutationFn: (code) => regenerateMFABackupCodes(code),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: authKeys.mfaStatus() })
    },
  })
}
