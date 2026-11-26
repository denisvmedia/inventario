import { http, HttpResponse } from "msw"

import { apiUrl } from "."

// MSW fake of the server drafts API (#2812), with the file endpoints a
// draft save uses. State lives in the object `backend` returns, so a test
// can seed it, read what the client stored, and change it behind the
// client's back (another device saving the same draft).
//
//   const fake = draftHandlers.backend("g")
//   server.use(...fake.handlers)
//   fake.drafts[0].values

export interface FakeDraft {
  id: string
  client_key: string
  title: string
  values: Record<string, unknown>
  revision: number
  // "discarding" is a draft whose discard has started: hidden from the list
  // and GET, refused by every write. "archived" stands for a state the
  // client does not know, served like an open draft.
  status: "open" | "promoted" | "discarding" | "archived"
  promoted_commodity_id?: string
  // The key of the promotion that finished the draft.
  promotion_key?: string
  updated_at: string
}

export interface FakeFile {
  id: string
  title: string
  path: string
  ext: string
  size_bytes: number
  sha256?: string
  category: string
  tags: string[]
  links: { id: string; file_id: string; entity_type: string; entity_id: string; meta?: string }[]
}

export interface FakeDraftsBackend {
  drafts: FakeDraft[]
  files: FakeFile[]
  // Bodies of POST /drafts/{id}/promote, in order.
  promotions: { id: string; body: Record<string, unknown> }[]
  // How many uploads reached the server.
  uploads: number
  // When set, the next uploads fail with this status.
  failUploads: number | null
  // What the next uploads store, in order. jsdom's File does not survive
  // the multipart round trip (see EditProfilePage.test.tsx), so the fake
  // cannot read the name or size from the request; a test that needs them
  // queues them here. An upload with nothing queued gets a made-up name.
  nextUploads: { name: string; size: number; sha256?: string }[]
  // When set, the next attaches fail with this status.
  failAttach: number | null
  // When set, the next promotions fail with this status, before the draft
  // is touched.
  failPromote: number | null
  // Ids of the files DELETE /files/{id} moved to Trash, in order.
  trashed: string[]
  // When set, the server applies the next request of that kind and the
  // client never gets the answer (a network error), as on a dropped
  // connection. Each flag is cleared once used.
  loseResponse: { create?: boolean; put?: boolean; promote?: boolean }
  // Counts of create calls, including idempotent repeats.
  creates: number
  handlers: ReturnType<typeof http.get>[]
}

function titleOf(values: Record<string, unknown>): string {
  return String(values.name ?? "").trim()
}

function fileCount(backend: FakeDraftsBackend, id: string): number {
  return backend.files.filter((f) =>
    f.links.some((l) => l.entity_type === "draft" && l.entity_id === id)
  ).length
}

function view(backend: FakeDraftsBackend, d: FakeDraft) {
  return { ...d, file_count: fileCount(backend, d.id) }
}

function one(backend: FakeDraftsBackend, d: FakeDraft, status = 200) {
  return HttpResponse.json(
    { data: { id: d.id, type: "commodity_drafts", attributes: view(backend, d) } },
    { status }
  )
}

function coded(status: number, code: string, meta: Record<string, string> = {}) {
  return HttpResponse.json({ errors: [{ status: String(status), code, meta }] }, { status })
}

export function backend(slug: string, seed: Partial<FakeDraft>[] = []): FakeDraftsBackend {
  const base = `/g/${encodeURIComponent(slug)}`
  let seq = 0
  const now = () => new Date().toISOString()
  const state: FakeDraftsBackend = {
    drafts: seed.map((d, i) => ({
      id: d.id ?? `draft-${i + 1}`,
      client_key: d.client_key ?? `seed-${i + 1}`,
      values: d.values ?? {},
      title: d.title ?? titleOf(d.values ?? {}),
      revision: d.revision ?? 1,
      status: d.status ?? "open",
      promoted_commodity_id: d.promoted_commodity_id,
      updated_at: d.updated_at ?? now(),
    })),
    files: [],
    promotions: [],
    uploads: 0,
    failUploads: null,
    nextUploads: [],
    failAttach: null,
    failPromote: null,
    trashed: [],
    loseResponse: {},
    creates: 0,
    handlers: [],
  }
  const find = (id: string) => state.drafts.find((d) => d.id === id)
  const notFound = () => HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
  // lose applies the flag of kind once: the answer the server built is
  // dropped on the way to the client.
  const lose = (kind: keyof FakeDraftsBackend["loseResponse"], answer: Response) => {
    if (!state.loseResponse[kind]) return answer
    state.loseResponse[kind] = false
    return HttpResponse.error()
  }
  const promoted = (d: FakeDraft) =>
    coded(409, "draft.promoted", { commodity_id: d.promoted_commodity_id ?? "" })

  state.handlers = [
    http.get(apiUrl(`${base}/drafts`), ({ request }) => {
      const params = new URL(request.url).searchParams
      const perPage = Number(params.get("per_page")) || 20
      const open = state.drafts.filter((d) => d.status === "open")
      return HttpResponse.json({
        data: open.slice(0, perPage).map((d) => view(state, d)),
        meta: { drafts: open.length },
      })
    }),
    http.get(apiUrl(`${base}/drafts/:id`), ({ params }) => {
      const d = find(String(params.id))
      if (!d || d.status === "discarding") return notFound()
      return one(state, d)
    }),
    http.post(apiUrl(`${base}/drafts`), async ({ request }) => {
      state.creates++
      const body = (await request.json()) as {
        data: { attributes: { client_key: string; values: Record<string, unknown> } }
      }
      const { client_key, values } = body.data.attributes
      const existing = state.drafts.find((d) => d.client_key === client_key)
      if (existing?.status === "discarding") return coded(409, "draft.discarded")
      if (existing) return lose("create", one(state, existing))
      const d: FakeDraft = {
        id: `draft-new-${++seq}`,
        client_key,
        values,
        title: titleOf(values),
        revision: 1,
        status: "open",
        updated_at: now(),
      }
      state.drafts.push(d)
      return lose("create", one(state, d, 201))
    }),
    http.put(apiUrl(`${base}/drafts/:id`), async ({ params, request }) => {
      const d = find(String(params.id))
      if (!d) return HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
      const body = (await request.json()) as {
        data: { attributes: { revision: number; values: Record<string, unknown> } }
      }
      const { revision, values } = body.data.attributes
      if (d.status === "promoted") return promoted(d)
      if (d.status === "discarding") return coded(409, "draft.discarded")
      if (revision !== d.revision) {
        return coded(409, "draft.revision_conflict", { revision: String(d.revision) })
      }
      d.values = values
      d.title = titleOf(values)
      d.revision++
      d.updated_at = now()
      return lose("put", one(state, d))
    }),
    http.delete(apiUrl(`${base}/drafts/:id`), ({ params }) => {
      const id = String(params.id)
      state.drafts = state.drafts.filter((d) => d.id !== id)
      for (const f of state.files) {
        f.links = f.links.filter((l) => !(l.entity_type === "draft" && l.entity_id === id))
      }
      return new HttpResponse(null, { status: 204 })
    }),
    http.post(apiUrl(`${base}/drafts/:id/promote`), async ({ params, request }) => {
      const d = find(String(params.id))
      if (!d) return HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
      const body = (await request.json()) as {
        data: { attributes: Record<string, unknown> }
        meta: { revision: number; promotion_key?: string }
      }
      state.promotions.push({ id: d.id, body })
      const key = body.meta.promotion_key
      if (!key) return HttpResponse.json({ errors: [{ status: "422" }] }, { status: 422 })
      if (d.status === "discarding") return coded(409, "draft.discarded")
      if (state.failPromote) {
        return HttpResponse.json({ errors: [{ status: "500" }] }, { status: state.failPromote })
      }
      if (d.status === "promoted") {
        // Only the promotion that finished the draft gets the item back.
        if (key !== d.promotion_key) return promoted(d)
        return HttpResponse.json({
          data: {
            id: d.promoted_commodity_id,
            type: "commodities",
            attributes: { ...body.data.attributes, id: d.promoted_commodity_id },
          },
        })
      }
      if (body.meta.revision !== d.revision) {
        return coded(409, "draft.revision_conflict", { revision: String(d.revision) })
      }
      const itemId = `item-from-${d.id}`
      d.status = "promoted"
      d.promoted_commodity_id = itemId
      d.promotion_key = key
      for (const f of state.files) {
        for (const l of f.links) {
          if (l.entity_type === "draft" && l.entity_id === d.id) {
            l.entity_type = "commodity"
            l.entity_id = itemId
          }
        }
      }
      return lose(
        "promote",
        HttpResponse.json(
          {
            data: {
              id: itemId,
              type: "commodities",
              attributes: { ...body.data.attributes, id: itemId },
            },
          },
          { status: 201 }
        )
      )
    }),
    http.post(apiUrl(`${base}/uploads/file`), () => {
      if (state.failUploads) {
        return HttpResponse.json({ errors: [{ status: "500" }] }, { status: state.failUploads })
      }
      state.uploads++
      const next = state.nextUploads.shift() ?? { name: `upload-${state.uploads}.bin`, size: 0 }
      const dot = next.name.lastIndexOf(".")
      const f: FakeFile = {
        id: `file-${++seq}`,
        // The server names an upload after the file and the time it came.
        title: `${dot > 0 ? next.name.slice(0, dot) : next.name}-1767225600`,
        path: dot > 0 ? next.name.slice(0, dot) : next.name,
        ext: dot > 0 ? next.name.slice(dot) : "",
        size_bytes: next.size,
        sha256: next.sha256,
        category: "images",
        tags: [],
        links: [],
      }
      state.files.push(f)
      return HttpResponse.json({ id: f.id, type: "files", attributes: f }, { status: 201 })
    }),
    http.put(apiUrl(`${base}/files/:id`), async ({ params, request }) => {
      const f = state.files.find((x) => x.id === String(params.id))
      if (!f) return HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
      const body = (await request.json()) as {
        data?: { attributes?: { title?: string; tags?: string[] } }
      }
      // The PUT replaces the metadata: a title left out is cleared.
      f.title = body.data?.attributes?.title ?? ""
      f.tags = body.data?.attributes?.tags ?? f.tags
      return HttpResponse.json({ id: f.id, type: "files", attributes: f })
    }),
    http.get(apiUrl(`${base}/files/:id`), ({ params }) => {
      const f = state.files.find((x) => x.id === String(params.id))
      if (!f) return HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
      return HttpResponse.json({ id: f.id, type: "files", attributes: f })
    }),
    http.post(apiUrl(`${base}/files/:id/links`), async ({ params, request }) => {
      const f = state.files.find((x) => x.id === String(params.id))
      if (!f) return HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
      const body = (await request.json()) as {
        data: { attributes: { entity_type: string; entity_id: string; meta?: string } }
      }
      const { entity_type, entity_id } = body.data.attributes
      if (entity_type === "draft") {
        const d = find(entity_id)
        if (!d) return HttpResponse.json({ errors: [{ status: "422" }] }, { status: 422 })
        if (d.status === "promoted") return promoted(d)
        if (d.status === "discarding") return coded(409, "draft.discarded")
      }
      if (state.failAttach) {
        return HttpResponse.json({ errors: [{ status: "500" }] }, { status: state.failAttach })
      }
      const existing = f.links.find(
        (l) => l.entity_type === entity_type && l.entity_id === entity_id
      )
      if (existing) return HttpResponse.json({ id: f.id, type: "files", attributes: f })
      f.links.push({ id: `link-${++seq}`, file_id: f.id, ...body.data.attributes })
      return HttpResponse.json({ id: f.id, type: "files", attributes: f }, { status: 201 })
    }),
    http.delete(apiUrl(`${base}/files/:id/links/:linkId`), ({ params }) => {
      const f = state.files.find((x) => x.id === String(params.id))
      if (!f) return HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
      f.links = f.links.filter((l) => l.id !== String(params.linkId))
      return HttpResponse.json({ id: f.id, type: "files", attributes: f })
    }),
    http.delete(apiUrl(`${base}/files/:id`), ({ params }) => {
      const id = String(params.id)
      if (!state.files.some((x) => x.id === id)) {
        return HttpResponse.json({ errors: [{ status: "404" }] }, { status: 404 })
      }
      state.files = state.files.filter((x) => x.id !== id)
      state.trashed.push(id)
      return new HttpResponse(null, { status: 204 })
    }),
    http.get(apiUrl(`${base}/files`), ({ request }) => {
      const params = new URL(request.url).searchParams
      const type = params.get("linked_entity_type")
      const id = params.get("linked_entity_id")
      const matched = state.files.filter(
        (f) => !type || f.links.some((l) => l.entity_type === type && l.entity_id === id)
      )
      return HttpResponse.json({
        data: matched,
        meta: { files: matched.length, total: matched.length },
      })
    }),
  ]
  return state
}
