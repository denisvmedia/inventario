import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// MSW fake of the batch intake API (#2810) and the upload endpoint the
// intake dialog uses. State lives in the object `backend` returns, so a test
// can seed sessions, read what the client sent, and play the worker by
// editing sources and candidates between requests.
//
//   const fake = intakeHandlers.backend("g")
//   server.use(...fake.handlers)

export interface FakeIntakeCandidate {
  id: string
  source_id: string
  sort_order: number
  fields: Record<string, { value: unknown; confidence: number }>
  accepted_fields: string[]
  quantity: number
  selected: boolean
  outcome: "pending" | "created" | "skipped" | "failed"
  draft_id?: string
  error_code?: string
}

export interface FakeIntakeSource {
  id: string
  sort_order: number
  status: "pending" | "scanning" | "scanned" | "failed" | "canceled"
  one_item: boolean
  wait_reason?: string
  error_code?: string
  file_ids: string[]
}

export interface FakeIntakeSession {
  id: string
  client_key: string
  created_at: string
  sources: FakeIntakeSource[]
  candidates: FakeIntakeCandidate[]
}

export interface FakeIntakeLimits {
  enabled: boolean
  max_files_per_batch: number
  max_files_per_source: number
  max_file_bytes: number
  max_queued_per_group: number
  queued: number
  rate_limit_per_hour: number
  accepted_mime_types: string[]
}

export interface FakeIntakeBackend {
  limits: FakeIntakeLimits
  sessions: FakeIntakeSession[]
  // Bodies of POST /intake/sessions, in order, including repeats.
  creates: { client_key: string; sources: { file_ids: string[]; one_item: boolean }[] }[]
  // How many uploads reached the server.
  uploads: number
  // PATCH bodies of candidate reviews, in order.
  reviews: { id: string; attributes: Record<string, unknown> }[]
  // How many times drafts were requested.
  draftRequests: number
  // When set, the server applies the next create and the client never gets
  // the answer. Cleared once used.
  loseCreate: boolean
  handlers: ReturnType<typeof http.get>[]
}

export const DEFAULT_LIMITS: FakeIntakeLimits = {
  enabled: true,
  max_files_per_batch: 50,
  max_files_per_source: 5,
  max_file_bytes: 10 * 1024 * 1024,
  max_queued_per_group: 200,
  queued: 0,
  rate_limit_per_hour: 30,
  accepted_mime_types: ["image/jpeg", "image/png", "application/pdf"],
}

function summary(s: FakeIntakeSession) {
  const count = (pred: (x: FakeIntakeSource) => boolean) => s.sources.filter(pred).length
  const cand = (pred: (x: FakeIntakeCandidate) => boolean) => s.candidates.filter(pred).length
  const out = {
    sources: s.sources.length,
    files: s.sources.reduce((n, src) => n + src.file_ids.length, 0),
    pending: count((x) => x.status === "pending"),
    rate_limited: count((x) => x.status === "pending" && x.wait_reason === "rate_limited"),
    scanning: count((x) => x.status === "scanning"),
    scanned: count((x) => x.status === "scanned"),
    failed: count((x) => x.status === "failed"),
    canceled: count((x) => x.status === "canceled"),
    candidates: s.candidates.length,
    selected: cand((c) => c.selected),
    undecided: cand((c) => c.outcome === "pending"),
    unfinished: cand((c) => c.selected && (c.outcome === "pending" || c.outcome === "failed")),
    drafts_created: cand((c) => c.outcome === "created"),
    skipped: cand((c) => c.outcome === "skipped"),
    candidates_failed: cand((c) => c.outcome === "failed"),
  }
  return out
}

function stateOf(s: FakeIntakeSession): "processing" | "review" | "done" {
  const sum = summary(s)
  if (sum.pending + sum.scanning > 0) return "processing"
  if (sum.undecided + sum.unfinished > 0) return "review"
  return "done"
}

function sessionView(s: FakeIntakeSession) {
  return {
    id: s.id,
    client_key: s.client_key,
    origin: "upload",
    state: stateOf(s),
    summary: summary(s),
    created_at: s.created_at,
    updated_at: s.created_at,
  }
}

function detail(s: FakeIntakeSession, status = 200, meta?: Record<string, number>) {
  return HttpResponse.json(
    {
      data: {
        id: s.id,
        type: "intake_sessions",
        attributes: {
          ...sessionView(s),
          sources: s.sources.map((src) => ({
            ...src,
            session_id: s.id,
            attempts: 0,
            files: src.file_ids.map((id) => ({ id, title: `${id}.jpg`, mime_type: "image/jpeg" })),
          })),
          candidates: s.candidates.map((c) => ({ ...c, session_id: s.id })),
        },
      },
      meta,
    },
    { status }
  )
}

export function backend(
  slug: string,
  seed: FakeIntakeSession[] = [],
  limits: Partial<FakeIntakeLimits> = {}
): FakeIntakeBackend {
  const base = `/g/${encodeURIComponent(slug)}`
  let seq = 0
  const state: FakeIntakeBackend = {
    limits: { ...DEFAULT_LIMITS, ...limits },
    sessions: seed,
    creates: [],
    uploads: 0,
    reviews: [],
    draftRequests: 0,
    loseCreate: false,
    handlers: [],
  }
  const find = (id: string) => state.sessions.find((s) => s.id === id)
  const notFound = () => HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })

  state.handlers = [
    http.get(apiUrl(`${base}/intake/limits`), () =>
      HttpResponse.json({ data: { type: "intake_limits", attributes: state.limits } })
    ),
    http.get(apiUrl(`${base}/intake/sessions`), ({ request }) => {
      const perPage = Number(new URL(request.url).searchParams.get("per_page")) || 20
      return HttpResponse.json({
        data: state.sessions.slice(0, perPage).map(sessionView),
        meta: { sessions: state.sessions.length },
      })
    }),
    http.get(apiUrl(`${base}/intake/sessions/:id`), ({ params }) => {
      const s = find(String(params.id))
      return s ? detail(s) : notFound()
    }),
    http.post(apiUrl(`${base}/uploads/file`), () => {
      state.uploads++
      const id = `file-${++seq}`
      return HttpResponse.json(
        { id, type: "files", attributes: { id, path: id, ext: ".jpg", links: [] } },
        { status: 201 }
      )
    }),
    http.post(apiUrl(`${base}/intake/sessions`), async ({ request }) => {
      const body = (await request.json()) as {
        data: {
          attributes: {
            client_key: string
            sources: { file_ids: string[]; one_item: boolean }[]
          }
        }
      }
      const attrs = body.data.attributes
      state.creates.push(attrs)
      const existing = state.sessions.find((s) => s.client_key === attrs.client_key)
      if (existing) return detail(existing)
      const id = `session-${state.sessions.length + 1}`
      const s: FakeIntakeSession = {
        id,
        client_key: attrs.client_key,
        created_at: new Date().toISOString(),
        sources: attrs.sources.map((src, i) => ({
          id: `${id}-source-${i + 1}`,
          sort_order: i,
          status: "pending",
          one_item: src.one_item,
          file_ids: src.file_ids,
        })),
        candidates: [],
      }
      state.sessions.unshift(s)
      if (state.loseCreate) {
        state.loseCreate = false
        return HttpResponse.error()
      }
      return detail(s, 201)
    }),
    http.patch(
      apiUrl(`${base}/intake/sessions/:id/candidates/:cid`),
      async ({ params, request }) => {
        const s = find(String(params.id))
        const c = s?.candidates.find((x) => x.id === String(params.cid))
        if (!s || !c) return notFound()
        const body = (await request.json()) as { data: { attributes: Record<string, unknown> } }
        const a = body.data.attributes
        state.reviews.push({ id: c.id, attributes: a })
        if (c.outcome === "created") {
          return HttpResponse.json(
            { errors: [{ status: "409", code: "intake.candidate_finished" }] },
            { status: 409 }
          )
        }
        if (typeof a.selected === "boolean") {
          c.selected = a.selected
          if (a.selected && c.outcome === "skipped") c.outcome = "pending"
        }
        if (Array.isArray(a.accepted_fields)) c.accepted_fields = a.accepted_fields as string[]
        if (typeof a.quantity === "number") c.quantity = a.quantity
        return HttpResponse.json({
          data: { id: c.id, type: "intake_candidates", attributes: { ...c, session_id: s.id } },
        })
      }
    ),
    http.post(apiUrl(`${base}/intake/sessions/:id/drafts`), ({ params }) => {
      const s = find(String(params.id))
      if (!s) return notFound()
      state.draftRequests++
      let created = 0
      let skipped = 0
      for (const c of s.candidates) {
        if (c.outcome === "created") continue
        if (!c.selected) {
          if (c.outcome === "pending") {
            c.outcome = "skipped"
            skipped++
          }
          continue
        }
        c.outcome = "created"
        c.draft_id = `draft-${c.id}`
        created++
      }
      return detail(s, 200, { created, skipped, failed: 0 })
    }),
    http.post(apiUrl(`${base}/intake/sessions/:id/cancel`), ({ params }) => {
      const s = find(String(params.id))
      if (!s) return notFound()
      for (const src of s.sources) if (src.status === "pending") src.status = "canceled"
      return detail(s)
    }),
    http.post(apiUrl(`${base}/intake/sessions/:id/resume`), ({ params }) => {
      const s = find(String(params.id))
      if (!s) return notFound()
      for (const src of s.sources) {
        if (src.status === "canceled" || src.status === "failed") src.status = "pending"
      }
      return detail(s)
    }),
    http.post(apiUrl(`${base}/intake/sessions/:id/sources/:sid/rescan`), ({ params }) => {
      const s = find(String(params.id))
      const src = s?.sources.find((x) => x.id === String(params.sid))
      if (!s || !src) return notFound()
      src.status = "pending"
      s.candidates = s.candidates.filter((c) => c.source_id !== src.id)
      return detail(s)
    }),
    http.delete(apiUrl(`${base}/intake/sessions/:id`), ({ params }) => {
      const before = state.sessions.length
      state.sessions = state.sessions.filter((s) => s.id !== String(params.id))
      if (state.sessions.length === before) return notFound()
      return new HttpResponse(null, { status: 204 })
    }),
  ]
  return state
}

// receiptSession is a scanned batch of two files: a receipt with three
// products and a photo the scan could not read.
export function receiptSession(id = "session-r"): FakeIntakeSession {
  const src = `${id}-receipt`
  const product = (n: number, name: string, price: number, quantity = 1): FakeIntakeCandidate => ({
    id: `${id}-c${n}`,
    source_id: src,
    sort_order: n,
    fields: {
      name: { value: name, confidence: 0.9 },
      original_price: { value: price, confidence: 0.9 },
      original_price_currency: { value: "EUR", confidence: 0.9 },
    },
    accepted_fields: ["name", "original_price", "original_price_currency"],
    quantity,
    selected: true,
    outcome: "pending",
  })
  return {
    id,
    client_key: `${id}-key`,
    created_at: new Date().toISOString(),
    sources: [
      { id: src, sort_order: 0, status: "scanned", one_item: false, file_ids: ["receipt"] },
      {
        id: `${id}-photo`,
        sort_order: 1,
        status: "failed",
        one_item: false,
        error_code: "file_unreadable",
        file_ids: ["photo"],
      },
    ],
    candidates: [
      product(0, "Espresso Machine", 449),
      product(1, "Milk Frother", 39.9),
      product(2, "Espresso Cups", 24, 2),
    ],
  }
}
