import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { draftKeys } from "@/features/drafts/keys"
import { fileKeys } from "@/features/files/keys"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { useInvalidateTrash } from "@/features/trash/hooks"
import type { GlobalErrorToastMeta } from "@/lib/global-error-toast"

import {
  cancelIntakeSession,
  createIntakeDrafts,
  deleteIntakeSession,
  getIntakeLimits,
  getIntakeSession,
  listIntakeSessions,
  rescanIntakeSource,
  resumeIntakeSession,
  reviewIntakeCandidate,
  type IntakeCandidate,
  type IntakeCandidateReview,
  type IntakeDraftsResult,
  type IntakeLimits,
  type IntakeListOptions,
  type IntakeSessionDetail,
  type IntakeSessionsPage,
} from "./api"
import { intakeKeys } from "./keys"

// Every intake action shows its own error toast, which names the action, so
// the global "Can't reach the server" toast would only repeat it.
const QUIET = { suppressGlobalErrorToast: true } satisfies GlobalErrorToastMeta

// How often a list or a session with files still in the queue is read again.
export const INTAKE_POLL_MS = 3000

function useSlug(): string {
  return useOptionalCurrentGroup()?.currentGroup?.slug ?? ""
}

export function useIntakeLimits(enabled = true) {
  const slug = useSlug()
  return useQuery<IntakeLimits>({
    queryKey: intakeKeys.limits(slug),
    queryFn: ({ signal }) => getIntakeLimits(signal),
    enabled: enabled && !!slug,
    staleTime: 30_000,
  })
}

// useIntakeSessions lists the group's batches, newest first. While one of
// them still has files in the queue it is read again every few seconds, so
// the progress moves without a reload.
export function useIntakeSessions(opts: IntakeListOptions = {}) {
  const slug = useSlug()
  return useQuery<IntakeSessionsPage>({
    queryKey: intakeKeys.list(slug, opts),
    queryFn: ({ signal }) => listIntakeSessions(opts, signal),
    enabled: !!slug,
    placeholderData: (prev) => prev,
    refetchInterval: (query) =>
      query.state.data?.sessions.some((s) => s.state === "processing") ? INTAKE_POLL_MS : false,
  })
}

export function useIntakeSession(id: string | null | undefined) {
  const slug = useSlug()
  return useQuery<IntakeSessionDetail>({
    queryKey: intakeKeys.detail(slug, id ?? ""),
    queryFn: ({ signal }) => getIntakeSession(id ?? "", signal),
    enabled: !!slug && !!id,
    retry: false,
    refetchInterval: (query) => (query.state.data?.state === "processing" ? INTAKE_POLL_MS : false),
  })
}

// useInvalidateIntake refreshes the batches, the limits (the queue count
// changes) and the file lists, which show the dropped files.
export function useInvalidateIntake() {
  const qc = useQueryClient()
  const slug = useSlug()
  return () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: intakeKeys.group(slug) }),
      qc.invalidateQueries({ queryKey: fileKeys.group(slug) }),
    ]).then(() => undefined)
}

// useStoreSession puts a session a mutation returned into the cache, so the
// page shows it at once, and refreshes the list.
function useStoreSession() {
  const qc = useQueryClient()
  const slug = useSlug()
  return (session: IntakeSessionDetail) => {
    qc.setQueryData(intakeKeys.detail(slug, session.id), session)
    return Promise.all([
      qc.invalidateQueries({ queryKey: [...intakeKeys.group(slug), "list"] }),
      qc.invalidateQueries({ queryKey: intakeKeys.limits(slug) }),
    ]).then(() => undefined)
  }
}

export interface ReviewCandidateInput extends IntakeCandidateReview {
  sessionId: string
  candidateId: string
}

export function useReviewIntakeCandidate() {
  const qc = useQueryClient()
  const slug = useSlug()
  return useMutation<IntakeCandidate, Error, ReviewCandidateInput>({
    meta: QUIET,
    mutationFn: ({ sessionId, candidateId, ...review }) =>
      reviewIntakeCandidate(sessionId, candidateId, review),
    onSuccess: (candidate, { sessionId }) => {
      qc.setQueryData<IntakeSessionDetail>(intakeKeys.detail(slug, sessionId), (prev) =>
        prev
          ? {
              ...prev,
              candidates: prev.candidates.map((c) => (c.id === candidate.id ? candidate : c)),
            }
          : prev
      )
      return qc.invalidateQueries({ queryKey: intakeKeys.group(slug) })
    },
  })
}

export function useCreateIntakeDrafts() {
  const qc = useQueryClient()
  const slug = useSlug()
  const store = useStoreSession()
  return useMutation<IntakeDraftsResult, Error, string>({
    meta: QUIET,
    mutationFn: (sessionId) => createIntakeDrafts(sessionId),
    onSuccess: (result) =>
      Promise.all([
        store(result.session),
        qc.invalidateQueries({ queryKey: draftKeys.group(slug) }),
        qc.invalidateQueries({ queryKey: fileKeys.group(slug) }),
      ]).then(() => undefined),
  })
}

export function useCancelIntakeSession() {
  const store = useStoreSession()
  return useMutation<IntakeSessionDetail, Error, string>({
    meta: QUIET,
    mutationFn: (sessionId) => cancelIntakeSession(sessionId),
    onSuccess: (session) => store(session),
  })
}

export function useResumeIntakeSession() {
  const store = useStoreSession()
  return useMutation<IntakeSessionDetail, Error, string>({
    meta: QUIET,
    mutationFn: (sessionId) => resumeIntakeSession(sessionId),
    onSuccess: (session) => store(session),
  })
}

export function useRescanIntakeSource() {
  const store = useStoreSession()
  return useMutation<IntakeSessionDetail, Error, { sessionId: string; sourceId: string }>({
    meta: QUIET,
    mutationFn: ({ sessionId, sourceId }) => rescanIntakeSource(sessionId, sourceId),
    onSuccess: (session) => store(session),
  })
}

// Deleting a batch moves the dropped files that nothing else uses to Trash;
// the drafts made from it stay.
export function useDeleteIntakeSession() {
  const invalidate = useInvalidateIntake()
  const invalidateTrash = useInvalidateTrash()
  return useMutation<void, Error, string>({
    meta: QUIET,
    mutationFn: (sessionId) => deleteIntakeSession(sessionId),
    onSuccess: () => Promise.all([invalidate(), invalidateTrash()]).then(() => undefined),
  })
}
