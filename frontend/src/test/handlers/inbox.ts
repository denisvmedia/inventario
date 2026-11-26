import { http, HttpResponse, type JsonBodyType } from "msw"

import { apiUrl } from "."

// MSW fake of the inbox (#2794) with the calls that clear an entry: DELETE
// /drafts/{id} discards a draft, and PUT /commodities/{id} with draft false
// reviews an item (draft true puts it back). State lives in the returned
// object, so a test can seed it and read what the client changed.
//
//   const fake = inboxHandlers.backend("g", [inboxHandlers.draft("d1", "Drill")])
//   server.use(...fake.handlers)

export interface FakeInboxEntry {
  kind: string
  id: string
  title: string
  reasons: string[]
  updated_at?: string | null
  file_count?: number
  intake?: { state: string; summary: Record<string, number> }
}

export interface FakeInboxCheck {
  key: string
  query: string
  count: number
}

// The checks the server sends, in its order, with no items in any.
export function defaultChecks(): FakeInboxCheck[] {
  return [
    { key: "no_location", query: "include_inactive=false&unassigned=true", count: 0 },
    { key: "no_photo", query: "include_inactive=false&no_photo=true", count: 0 },
    { key: "no_purchase_price", query: "include_inactive=false&no_purchase_price=true", count: 0 },
    { key: "no_purchase_date", query: "include_inactive=false&no_purchase_date=true", count: 0 },
    { key: "untagged", query: "include_inactive=false&no_tags=true", count: 0 },
  ]
}

export interface FakeInboxBackend {
  entries: FakeInboxEntry[]
  // What GET /inbox/checks answers; never part of the counts.
  checks: FakeInboxCheck[]
  // Ids of the items whose review was completed, in order.
  reviewed: string[]
  // When set, PATCH /commodities/{id}/draft answers with this status and
  // body; see refusedReview for the server's 422.
  failReview: { status: number; body: JsonBodyType } | null
  // How many times GET /inbox/count was served.
  countCalls: number
  handlers: ReturnType<typeof http.get>[]
}

export function draft(id: string, title: string, extra: Partial<FakeInboxEntry> = {}) {
  return {
    kind: "draft",
    id,
    title,
    reasons: ["draft"],
    updated_at: "2026-11-20T10:00:00Z",
    file_count: 0,
    ...extra,
  }
}

export function itemDraft(id: string, title: string, extra: Partial<FakeInboxEntry> = {}) {
  return { kind: "commodity", id, title, reasons: ["item_draft"], updated_at: null, ...extra }
}

// intakeSession is an open batch of files (#2810) in the given state with the
// summary counts given; the rest of the summary is zero.
export function intakeSession(
  id: string,
  state: "processing" | "review" | "done",
  summary: Record<string, number> = {},
  extra: Partial<FakeInboxEntry> = {}
): FakeInboxEntry {
  return {
    kind: "intake_session",
    id,
    title: "",
    reasons: ["intake"],
    updated_at: "2026-11-20T10:00:00Z",
    file_count: summary.files ?? 1,
    intake: { state, summary: { sources: 1, files: 1, ...summary } },
    ...extra,
  }
}

// refusedReview is the server's answer to a review of an item priced in
// another currency without an amount in the group currency: a 422 with the
// tree of field errors and codes, and no detail or title.
export function refusedReview(): { status: number; body: JsonBodyType } {
  return {
    status: 422,
    body: {
      errors: [
        {
          status: "Unprocessable Entity",
          error: {
            error: {
              data: {
                attributes: {
                  original_price:
                    "if original price is not in the group currency, converted original price or current price must be set",
                },
              },
            },
            errorCodes: {
              data: {
                attributes: { original_price: { code: "validation_no_price_in_group_currency" } },
              },
            },
            type: "validation.Errors",
          },
        },
      ],
    },
  }
}

function counts(entries: FakeInboxEntry[]) {
  const reasons: Record<string, number> = { draft: 0, item_draft: 0, intake: 0 }
  for (const e of entries) for (const r of e.reasons) reasons[r] = (reasons[r] ?? 0) + 1
  return { total: entries.length, reasons }
}

export function backend(slug: string, seed: FakeInboxEntry[] = []): FakeInboxBackend {
  const base = `/g/${encodeURIComponent(slug)}`
  const state: FakeInboxBackend = {
    entries: seed.map((e) => ({ ...e })),
    checks: defaultChecks(),
    reviewed: [],
    failReview: null,
    countCalls: 0,
    handlers: [],
  }
  // The items the fake knows, so a review can put one back.
  const items = new Map(state.entries.filter((e) => e.kind === "commodity").map((e) => [e.id, e]))

  state.handlers = [
    http.get(apiUrl(`${base}/inbox/count`), () => {
      state.countCalls += 1
      return HttpResponse.json({ data: counts(state.entries) })
    }),
    http.get(apiUrl(`${base}/inbox/checks`), () => HttpResponse.json({ data: state.checks })),
    http.get(apiUrl(`${base}/inbox`), ({ request }) => {
      const url = new URL(request.url)
      const reason = url.searchParams.get("reason")
      const page = Number(url.searchParams.get("page") ?? "1")
      const perPage = Number(url.searchParams.get("per_page") ?? "50")
      const kept = reason ? state.entries.filter((e) => e.reasons.includes(reason)) : state.entries
      const start = (page - 1) * perPage
      return HttpResponse.json({
        data: kept.slice(start, start + perPage),
        meta: { entries: kept.length, counts: counts(state.entries) },
      })
    }),
    http.delete(apiUrl(`${base}/drafts/:id`), ({ params }) => {
      state.entries = state.entries.filter((e) => !(e.kind === "draft" && e.id === params.id))
      return new HttpResponse(null, { status: 204 })
    }),
    http.delete(apiUrl(`${base}/intake/sessions/:id`), ({ params }) => {
      state.entries = state.entries.filter(
        (e) => !(e.kind === "intake_session" && e.id === params.id)
      )
      return new HttpResponse(null, { status: 204 })
    }),
    http.get(apiUrl(`${base}/commodities/:id`), ({ params }) => {
      const item = items.get(String(params.id))
      if (!item) return HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
      return HttpResponse.json({
        data: {
          id: item.id,
          type: "commodities",
          attributes: {
            name: item.title,
            draft: state.entries.some((e) => e.id === item.id),
          },
        },
      })
    }),
    http.patch(apiUrl(`${base}/commodities/:id/draft`), async ({ params, request }) => {
      if (state.failReview !== null) {
        return HttpResponse.json(state.failReview.body, { status: state.failReview.status })
      }
      const id = String(params.id)
      const item = items.get(id)
      if (!item) return HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
      const body = (await request.json()) as { data: { attributes: { draft: boolean } } }
      const isDraft = body.data.attributes.draft
      state.entries = state.entries.filter((e) => e.id !== id)
      if (isDraft) state.entries.push(item)
      else state.reviewed.push(id)
      return HttpResponse.json({
        data: { id, type: "commodities", attributes: { name: item.title, draft: isDraft } },
      })
    }),
  ]
  return state
}

// empty answers the count with an empty inbox, for tests of the surfaces
// that show the count but are not about it.
export function empty(slug: string) {
  return backend(slug).handlers
}
