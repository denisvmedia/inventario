// Pure data-layer functions for the commodity-loans feature slice.
// Hooks live in `./hooks.ts`. Backed by the `/loans` + `/commodities/{id}/loans`
// endpoints introduced under #1452. The dedicated /lent page consumes
// the group-wide list shape; the per-commodity Lend tab consumes the
// per-commodity list.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type LoanEntity = Schema<"models.CommodityLoan">
export type LoanState = "all" | "open" | "overdue" | "returned"

// LoanCommodityRef mirrors the BE's tiny denormalised view returned
// alongside loans on the group-wide endpoint so the FE can render
// "name → borrower" without a second round-trip.
export interface LoanCommodityRef {
  id: string
  name: string
  short_name?: string
}

export interface ListedLoan {
  loan: LoanEntity & { id: string }
  commodity?: LoanCommodityRef
}

interface LoanDetailEnvelope {
  id?: string
  type?: string
  attributes?: LoanEntity
}

// Per-commodity envelope: the BE flattens the loan onto the row and
// omits the commodity ref (it's implied by the URL path). Mirrors the
// shape returned by GET /commodities/{id}/loans.
interface PerCommodityListEnvelope {
  data?: Array<LoanEntity & { id: string }>
  meta?: { loans?: number; total?: number }
}

// Group-wide envelope: each row carries an optional `commodity` ref.
interface GroupListEnvelope {
  data?: Array<LoanEntity & { id: string; commodity?: LoanCommodityRef }>
  meta?: { loans?: number; total?: number }
}

interface LoanCountsEnvelope {
  data?: Record<string, number>
}

export interface ListLoansForCommodityResult {
  loans: Array<LoanEntity & { id: string }>
  total: number
}

export async function listLoansForCommodity(
  commodityID: string,
  signal?: AbortSignal
): Promise<ListLoansForCommodityResult> {
  const body = await http.get<PerCommodityListEnvelope>(
    `/commodities/${encodeURIComponent(commodityID)}/loans`,
    { signal }
  )
  return {
    loans: body.data ?? [],
    total: body.meta?.total ?? body.data?.length ?? 0,
  }
}

export interface ListGroupLoansOptions {
  page?: number
  perPage?: number
  state?: LoanState
  signal?: AbortSignal
}

export async function listGroupLoans(
  opts: ListGroupLoansOptions = {}
): Promise<{ loans: ListedLoan[]; total: number }> {
  const params = new URLSearchParams()
  if (opts.page !== undefined) params.set("page", String(opts.page))
  if (opts.perPage !== undefined) params.set("per_page", String(opts.perPage))
  if (opts.state) params.set("state", opts.state)
  const qs = params.toString()
  const path = qs ? `/loans?${qs}` : "/loans"
  const body = await http.get<GroupListEnvelope>(path, { signal: opts.signal })
  return {
    loans: (body.data ?? []).map((row) => {
      const { commodity, ...rest } = row
      return { loan: rest, commodity }
    }),
    total: body.meta?.total ?? body.data?.length ?? 0,
  }
}

export async function getLoanCounts(
  commodityIDs: string[],
  signal?: AbortSignal
): Promise<Record<string, number>> {
  if (commodityIDs.length === 0) return {}
  const params = new URLSearchParams()
  for (const id of commodityIDs) params.append("commodity_id", id)
  const body = await http.get<LoanCountsEnvelope>(`/loans/counts?${params.toString()}`, { signal })
  return body.data ?? {}
}

export interface StartLoanRequest {
  commodity_id: string
  borrower_name: string
  borrower_contact?: string
  borrower_note?: string
  lent_at: string // YYYY-MM-DD
  due_back_at?: string | null
}

export async function startLoan(req: StartLoanRequest): Promise<LoanEntity & { id: string }> {
  const { commodity_id, ...attrs } = req
  const body = await http.post<LoanDetailEnvelope>(
    `/commodities/${encodeURIComponent(commodity_id)}/loans`,
    {
      data: { type: "commodity_loans", attributes: attrs },
    }
  )
  if (!body.attributes) {
    throw new Error(`Malformed POST /loans response: missing attributes`)
  }
  return { ...body.attributes, id: body.id ?? "" }
}

export interface UpdateLoanRequest {
  borrower_name?: string
  borrower_contact?: string
  borrower_note?: string
  due_back_at?: string
}

export async function updateLoan(
  commodityID: string,
  loanID: string,
  req: UpdateLoanRequest
): Promise<LoanEntity & { id: string }> {
  const body = await http.patch<LoanDetailEnvelope>(
    `/commodities/${encodeURIComponent(commodityID)}/loans/${encodeURIComponent(loanID)}`,
    {
      data: { id: loanID, type: "commodity_loans", attributes: req },
    }
  )
  if (!body.attributes) {
    throw new Error(`Malformed PATCH /loans/${loanID} response: missing attributes`)
  }
  return { ...body.attributes, id: body.id ?? loanID }
}

// returnedAt defaults to today (server-side) — pass undefined for the
// canonical case. Override by passing a YYYY-MM-DD string for late
// "user marks return on the day-of, after midnight rolled over" cases.
export async function returnLoan(
  commodityID: string,
  loanID: string,
  returnedAt?: string
): Promise<LoanEntity & { id: string }> {
  const payload = returnedAt
    ? {
        data: {
          type: "commodity_loans",
          attributes: { returned_at: returnedAt },
        },
      }
    : undefined
  const body = await http.post<LoanDetailEnvelope>(
    `/commodities/${encodeURIComponent(commodityID)}/loans/${encodeURIComponent(loanID)}/return`,
    payload
  )
  if (!body.attributes) {
    throw new Error(`Malformed POST /loans/${loanID}/return response: missing attributes`)
  }
  return { ...body.attributes, id: body.id ?? loanID }
}

export async function deleteLoan(commodityID: string, loanID: string): Promise<void> {
  await http.del<void>(
    `/commodities/${encodeURIComponent(commodityID)}/loans/${encodeURIComponent(loanID)}`
  )
}

// isOpen / daysOverdue are display helpers. They mirror models.CommodityLoan
// methods on the BE so the FE renders the same overdue badge as the
// /loans list state filter — single source of truth for "what's open."
export function isOpen(loan: Pick<LoanEntity, "returned_at">): boolean {
  return !loan.returned_at
}

export function daysOverdue(
  loan: Pick<LoanEntity, "due_back_at" | "returned_at">,
  now: Date = new Date()
): number {
  if (!isOpen(loan) || !loan.due_back_at) return 0
  const due = new Date(`${loan.due_back_at}T00:00:00`)
  if (Number.isNaN(due.getTime())) return 0
  const diff = now.getTime() - due.getTime()
  if (diff <= 0) return 0
  return Math.floor(diff / (1000 * 60 * 60 * 24))
}
