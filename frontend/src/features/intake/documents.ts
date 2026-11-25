// Documents other systems deliver to the intake endpoint (#2809), such as
// an invoice a Paperless-ngx workflow sends. Each one becomes a draft; the
// draft and the item it becomes show where the document came from.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

type DocumentsEnvelope = Schema<"jsonapi.IntakeDocumentsResponse">

export type IntakeDocumentState = "pending" | "draft" | "item" | "removed"

// The other system a draft or an item came from, as shown to the member.
export interface DocumentSource {
  name: string
  // The document in the other system; null when none was sent or it is not
  // a web address.
  url: string | null
}

export interface IntakeDocument {
  id: string
  source: DocumentSource
  draftId: string | null
  commodityId: string | null
  state: IntakeDocumentState
}

// safeSourceUrl keeps an http or https address and drops anything else. The
// server only stores such addresses; checking again keeps a link from ever
// running script.
export function safeSourceUrl(raw: string | null | undefined): string | null {
  if (!raw) return null
  try {
    const url = new URL(raw)
    return url.protocol === "http:" || url.protocol === "https:" ? raw : null
  } catch {
    return null
  }
}

// toDocumentSource names the system by its display name, or by its id when
// it sent none.
export function toDocumentSource(
  name: string | null | undefined,
  system: string | null | undefined,
  url: string | null | undefined
): DocumentSource | null {
  const shown = (name ?? "").trim() || (system ?? "").trim()
  if (!shown) return null
  return { name: shown, url: safeSourceUrl(url) }
}

function toState(raw: string | undefined): IntakeDocumentState {
  return raw === "pending" || raw === "item" || raw === "removed" ? raw : "draft"
}

export interface IntakeDocumentsQuery {
  draftIds?: string[]
  commodityIds?: string[]
}

export async function listIntakeDocuments(
  query: IntakeDocumentsQuery,
  signal?: AbortSignal
): Promise<IntakeDocument[]> {
  const params = new URLSearchParams()
  if (query.draftIds?.length) params.set("draft_id", query.draftIds.join(","))
  if (query.commodityIds?.length) params.set("commodity_id", query.commodityIds.join(","))
  const body = await http.get<DocumentsEnvelope>(`/intake/documents?${params.toString()}`, {
    signal,
  })
  const out: IntakeDocument[] = []
  for (const d of body.data ?? []) {
    const a = d.attributes
    const source = toDocumentSource(a?.source_name, a?.external_system, a?.source_url)
    if (!d.id || !source) continue
    out.push({
      id: d.id,
      source,
      draftId: a?.draft_id || null,
      commodityId: a?.commodity_id || null,
      state: toState(a?.state),
    })
  }
  return out
}
