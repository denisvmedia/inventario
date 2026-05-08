import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"

import { commodityKeys } from "@/features/commodities/keys"
import { useCurrentGroup } from "@/features/group/GroupContext"

import {
  deleteLoan,
  getLoanCounts,
  listGroupLoans,
  listLoansForCommodity,
  returnLoan,
  startLoan,
  updateLoan,
  type ListGroupLoansOptions,
  type ListedLoan,
  type LoanEntity,
  type StartLoanRequest,
  type UpdateLoanRequest,
} from "./api"
import { loanKeys } from "./keys"

interface QueryOptions {
  enabled?: boolean
}

export function useLoansForCommodity(
  commodityID: string | undefined,
  { enabled = true }: QueryOptions = {}
) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<{ loans: Array<LoanEntity & { id: string }>; total: number }>({
    queryKey: loanKeys.byCommodity(slug, commodityID ?? ""),
    queryFn: ({ signal }) => {
      if (!commodityID) {
        throw new Error("useLoansForCommodity called without a commodity id")
      }
      return listLoansForCommodity(commodityID, signal)
    },
    enabled: enabled && !!commodityID,
  })
}

export function useGroupLoans(opts: ListGroupLoansOptions = {}, query: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<{ loans: ListedLoan[]; total: number }>({
    queryKey: loanKeys.groupList(slug, opts),
    queryFn: ({ signal }) => listGroupLoans({ ...opts, signal }),
    enabled: query.enabled ?? true,
    placeholderData: (prev) => prev,
  })
}

// useLoanCounts powers the list-page "lent out" badge. Empty input
// short-circuits to no query — the badge column is hidden until the
// commodity list resolves a non-empty page.
export function useLoanCounts(commodityIDs: string[], { enabled = true }: QueryOptions = {}) {
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return useQuery<Record<string, number>>({
    queryKey: loanKeys.counts(slug, commodityIDs),
    queryFn: ({ signal }) => getLoanCounts(commodityIDs, signal),
    enabled: enabled && commodityIDs.length > 0,
    placeholderData: (prev) => prev,
  })
}

function useInvalidate() {
  const qc = useQueryClient()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  return {
    all: () => qc.invalidateQueries({ queryKey: loanKeys.group(slug) }),
    forCommodity: (commodityID: string) =>
      qc.invalidateQueries({ queryKey: loanKeys.byCommodity(slug, commodityID) }),
    // Loan mutations emit `lent_out` / `returned` / `loan_updated`
    // audit events server-side (#1507). The events feed the
    // commodity history timeline, so its cache has to roll forward
    // too — without this, the timeline silently shows a stale view
    // until the user navigates away and back.
    eventsForCommodity: (commodityID: string) =>
      qc.invalidateQueries({
        queryKey: [...commodityKeys.group(slug), "events", commodityID],
      }),
  }
}

export function useStartLoan() {
  const invalidate = useInvalidate()
  return useMutation<LoanEntity & { id: string }, Error, StartLoanRequest>({
    mutationFn: (req) => startLoan(req),
    onSuccess: (_loan, vars) => {
      invalidate.forCommodity(vars.commodity_id)
      invalidate.eventsForCommodity(vars.commodity_id)
      invalidate.all()
    },
  })
}

interface UpdateLoanVars {
  commodityID: string
  loanID: string
  req: UpdateLoanRequest
}

export function useUpdateLoan() {
  const invalidate = useInvalidate()
  return useMutation<LoanEntity & { id: string }, Error, UpdateLoanVars>({
    mutationFn: ({ commodityID, loanID, req }) => updateLoan(commodityID, loanID, req),
    onSuccess: (_loan, vars) => {
      invalidate.forCommodity(vars.commodityID)
      invalidate.eventsForCommodity(vars.commodityID)
      invalidate.all()
    },
  })
}

interface ReturnLoanVars {
  commodityID: string
  loanID: string
  returnedAt?: string
}

export function useReturnLoan() {
  const invalidate = useInvalidate()
  return useMutation<LoanEntity & { id: string }, Error, ReturnLoanVars>({
    mutationFn: ({ commodityID, loanID, returnedAt }) =>
      returnLoan(commodityID, loanID, returnedAt),
    onSuccess: (_loan, vars) => {
      invalidate.forCommodity(vars.commodityID)
      invalidate.eventsForCommodity(vars.commodityID)
      invalidate.all()
    },
  })
}

interface DeleteLoanVars {
  commodityID: string
  loanID: string
}

export function useDeleteLoan() {
  const invalidate = useInvalidate()
  return useMutation<void, Error, DeleteLoanVars>({
    mutationFn: ({ commodityID, loanID }) => deleteLoan(commodityID, loanID),
    onSuccess: (_void, vars) => {
      invalidate.forCommodity(vars.commodityID)
      invalidate.eventsForCommodity(vars.commodityID)
      invalidate.all()
    },
  })
}
