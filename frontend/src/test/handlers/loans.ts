import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// Loan attributes mirror models.CommodityLoan — `commodity_id`,
// `borrower_*`, dates, `returned_at` (nullable on open loans).
type LoanAttrs = {
  id: string
  commodity_id: string
  borrower_name: string
  borrower_contact?: string
  borrower_note?: string
  lent_at: string
  due_back_at?: string | null
  returned_at?: string | null
}

type LoanWithCommodity = LoanAttrs & {
  commodity?: { id: string; name: string; short_name?: string }
}

// listForCommodity backs GET /commodities/{id}/loans — entities are
// FLAT inside `data`. Mirrors the per-commodity envelope used by the
// Lend tab.
export function listForCommodity(slug: string, commodityID: string, items: LoanAttrs[] = []) {
  return [
    http.get(
      apiUrl(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/loans`),
      () =>
        HttpResponse.json({
          data: items,
          meta: { loans: items.length, total: items.length },
        })
    ),
  ]
}

// listGroup backs GET /loans — group-wide list with the per-row
// `commodity` denorm block. Tests pass items with the optional commodity
// ref attached so the page renders the "Item" column.
export function listGroup(slug: string, items: LoanWithCommodity[] = []) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/loans`), ({ request }) => {
      // GET /loans paginates server-side (commodity_loans.go::listGroupLoans),
      // so the mock slices too — otherwise a page-2 request would be answered
      // with page 1 and a pager test would pass without a pager. `state` is
      // left to the caller's fixture: deriving open/overdue/returned from the
      // dates here would just be a second implementation of it.
      const params = new URL(request.url).searchParams
      const perPage = Math.min(Number(params.get("per_page")) || 50, 100)
      const page = Math.max(Number(params.get("page")) || 1, 1)
      const start = Math.min((page - 1) * perPage, items.length)
      const paged = items.slice(start, start + perPage)
      return HttpResponse.json({
        data: paged,
        meta: { loans: paged.length, total: items.length },
      })
    }),
  ]
}

// counts backs GET /loans/counts?commodity_id=... — flat
// commodity_id → open-loan count map. Empty map if not set.
export function counts(slug: string, byCommodity: Record<string, number> = {}) {
  return [
    http.get(apiUrl(`/g/${encodeURIComponent(slug)}/loans/counts`), () =>
      HttpResponse.json({ data: byCommodity })
    ),
  ]
}

// startLoan backs POST /commodities/{id}/loans. The handler echoes the
// request attributes back as a 201 — fine for tests that just need to
// observe the mutation fired. Single-loan responses now use the
// canonical `{data: {id, type, attributes}}` envelope (#1510 bugfix).
export function startLoan(slug: string, commodityID: string, response: LoanAttrs) {
  return [
    http.post(
      apiUrl(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/loans`),
      () =>
        HttpResponse.json(
          { data: { id: response.id, type: "commodity_loans", attributes: response } },
          { status: 201 }
        )
    ),
  ]
}

export function returnLoan(slug: string, commodityID: string, loanID: string, response: LoanAttrs) {
  return [
    http.post(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/loans/${encodeURIComponent(loanID)}/return`
      ),
      () =>
        HttpResponse.json({
          data: { id: response.id, type: "commodity_loans", attributes: response },
        })
    ),
  ]
}

export function deleteLoan(slug: string, commodityID: string, loanID: string) {
  return [
    http.delete(
      apiUrl(
        `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodityID)}/loans/${encodeURIComponent(loanID)}`
      ),
      () => new HttpResponse(null, { status: 204 })
    ),
  ]
}
