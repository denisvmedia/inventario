import { useEffect, useMemo, useRef } from "react"
import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query"

import { draftKeys } from "@/features/drafts/keys"
import { fileKeys } from "@/features/files/keys"
import { useOptionalCurrentGroup } from "@/features/group/GroupContext"
import { inboxKeys } from "@/features/inbox/keys"
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
  type IntakeSessionState,
  type IntakeSessionsPage,
} from "./api"
import { listIntakeDocuments, type DocumentSource } from "./documents"
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
  const query = useQuery<IntakeSessionsPage>({
    queryKey: intakeKeys.list(slug, opts),
    queryFn: ({ signal }) => listIntakeSessions(opts, signal),
    enabled: !!slug,
    placeholderData: (prev) => prev,
    refetchInterval: (query) =>
      query.state.data?.sessions.some((s) => s.state === "processing") ? INTAKE_POLL_MS : false,
  })
  useRefreshWhenScanned(query.data?.sessions)
  return query
}

export function useIntakeSession(id: string | null | undefined) {
  const slug = useSlug()
  const query = useQuery<IntakeSessionDetail>({
    queryKey: intakeKeys.detail(slug, id ?? ""),
    queryFn: ({ signal }) => getIntakeSession(id ?? "", signal),
    enabled: !!slug && !!id,
    retry: false,
    refetchInterval: (query) => (query.state.data?.state === "processing" ? INTAKE_POLL_MS : false),
  })
  const session = query.data
  const scanned = useMemo(
    () => (session ? [{ id: session.id, state: session.state }] : undefined),
    [session]
  )
  useRefreshWhenScanned(scanned)
  return query
}

// useRefreshWhenScanned reads a batch and the file lists once more after the
// batch stops scanning (#1626). Once the server stores a file's scan it puts
// the scan's tag and title on the file, so the read that finds the batch
// out of "processing" can still show the files as they were. The second
// read comes one poll interval later, enough for those writes. A read that
// finds the batch done from the start reads nothing again.
export function useRefreshWhenScanned(
  sessions: readonly { id: string; state: IntakeSessionState }[] | undefined
) {
  const qc = useQueryClient()
  const slug = useSlug()
  const lastState = useRef(new Map<string, IntakeSessionState>())
  const timers = useRef(new Set<ReturnType<typeof setTimeout>>())
  useEffect(() => {
    const pending = timers.current
    return () => {
      for (const timer of pending) clearTimeout(timer)
      pending.clear()
    }
  }, [])
  useEffect(() => {
    const settled: string[] = []
    for (const s of sessions ?? []) {
      if (lastState.current.get(s.id) === "processing" && s.state !== "processing") {
        settled.push(s.id)
      }
      lastState.current.set(s.id, s.state)
    }
    if (settled.length === 0 || !slug) return
    const timer = setTimeout(() => {
      timers.current.delete(timer)
      for (const id of settled) {
        void qc.invalidateQueries({ queryKey: intakeKeys.detail(slug, id) })
      }
      void qc.invalidateQueries({ queryKey: fileKeys.group(slug) })
      void qc.invalidateQueries({ queryKey: draftKeys.group(slug) })
    }, INTAKE_POLL_MS)
    timers.current.add(timer)
  }, [sessions, qc, slug])
}

// useItemSource returns the other system an item's document came from
// (#2809), or null for an item made in the app. A failed read shows no
// source rather than an error: the item page works without it.
export function useItemSource(commodityId: string | null | undefined) {
  const slug = useSlug()
  return useQuery<DocumentSource | null>({
    queryKey: intakeKeys.itemDocuments(slug, commodityId ?? ""),
    queryFn: async ({ signal }) => {
      const docs = await listIntakeDocuments({ commodityIds: [commodityId ?? ""] }, signal)
      return docs.find((d) => d.commodityId === commodityId)?.source ?? null
    },
    enabled: !!slug && !!commodityId,
    staleTime: 5 * 60_000,
    retry: false,
    meta: QUIET,
  })
}

// useInvalidateIntake refreshes the batches, the limits (the queue count
// changes), the file lists, which show the dropped files, and the inbox,
// which lists the open batches.
export function useInvalidateIntake() {
  const qc = useQueryClient()
  const slug = useSlug()
  return () =>
    Promise.all([
      qc.invalidateQueries({ queryKey: intakeKeys.group(slug) }),
      qc.invalidateQueries({ queryKey: fileKeys.group(slug) }),
      qc.invalidateQueries({ queryKey: inboxKeys.all(slug) }),
    ]).then(() => undefined)
}

// useStoreSession puts a session a mutation returned into the cache, so the
// page shows it at once, and refreshes the list and the inbox.
function useStoreSession() {
  const qc = useQueryClient()
  const slug = useSlug()
  return (session: IntakeSessionDetail) => {
    qc.setQueryData(intakeKeys.detail(slug, session.id), session)
    return Promise.all([
      qc.invalidateQueries({ queryKey: [...intakeKeys.group(slug), "list"] }),
      qc.invalidateQueries({ queryKey: intakeKeys.limits(slug) }),
      qc.invalidateQueries({ queryKey: inboxKeys.all(slug) }),
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
    mutationKey: intakeKeys.review(slug),
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

// reviewsSettled resolves once no product review of the group is being
// saved. The drafts take what the server holds, so creating them waits for
// those saves; a quantity is saved when its field loses focus, which
// happens on the same click that creates the drafts.
export function reviewsSettled(qc: QueryClient, slug: string): Promise<void> {
  const mutationKey = intakeKeys.review(slug)
  if (qc.isMutating({ mutationKey }) === 0) return Promise.resolve()
  return new Promise((resolve) => {
    const unsubscribe = qc.getMutationCache().subscribe(() => {
      if (qc.isMutating({ mutationKey }) > 0) return
      unsubscribe()
      resolve()
    })
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

// useRescanIntakeSource scans one file again. The server refuses while the
// file is still being scanned or has drafts made from it, which this page may
// not know yet (a draft made by a request whose answer was lost); the session
// is read again after a refusal, so the page shows why.
export function useRescanIntakeSource() {
  const qc = useQueryClient()
  const slug = useSlug()
  const store = useStoreSession()
  return useMutation<IntakeSessionDetail, Error, { sessionId: string; sourceId: string }>({
    meta: QUIET,
    mutationFn: ({ sessionId, sourceId }) => rescanIntakeSource(sessionId, sourceId),
    onSuccess: (session) => store(session),
    onError: (_err, { sessionId }) =>
      qc.invalidateQueries({ queryKey: intakeKeys.detail(slug, sessionId) }),
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
