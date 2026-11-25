// Data layer for the group's sellers (#2804). Hooks live in `./hooks.ts`.
// Backed by `/g/{slug}/sellers` and `/g/{slug}/sellers/{id}`.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

// A seller with the number of the group's items bought from it, drafts and
// inactive items included, items in Trash left out.
export type Seller = Schema<"jsonapi.SellerView"> & { id: string }

// Limits mirror go/models/seller.go.
export const SELLER_NAME_MAX = 100
export const SELLER_WEBSITE_MAX = 2048
export const SELLER_CONTACT_MAX = 500
export const SELLER_NOTE_MAX = 2000

// The JSON:API error code of a 409 on a create or rename whose name another
// seller of the group already has.
export const SELLER_NAME_TAKEN = "seller.name_taken"

export interface SellerInput {
  name: string
  website: string
  contact: string
  note: string
}

type SellerAttributes = Schema<"jsonapi.SellerView">

interface SellerResource {
  id?: string
  type?: string
  attributes?: SellerAttributes
}

interface SellerEnvelope {
  data?: SellerResource
}

interface SellersEnvelope {
  data?: SellerResource[]
  meta?: { sellers?: number }
}

function toSeller(resource: SellerResource | undefined, what: string): Seller {
  if (!resource?.attributes || !resource.id) {
    throw new Error(`Malformed ${what} response: missing data`)
  }
  return { ...resource.attributes, id: resource.id }
}

export async function listSellers(signal?: AbortSignal): Promise<Seller[]> {
  const body = await http.get<SellersEnvelope>("/sellers", { signal })
  return (body.data ?? []).map((row) => toSeller(row, "GET /sellers"))
}

export async function createSeller(input: SellerInput): Promise<Seller> {
  const body = await http.post<SellerEnvelope>("/sellers", {
    data: { type: "sellers", attributes: input },
  })
  return toSeller(body.data, "POST /sellers")
}

export async function updateSeller(id: string, input: Partial<SellerInput>): Promise<Seller> {
  const body = await http.patch<SellerEnvelope>(`/sellers/${encodeURIComponent(id)}`, {
    data: { id, type: "sellers", attributes: input },
  })
  return toSeller(body.data, `PATCH /sellers/${id}`)
}

export async function deleteSeller(id: string): Promise<void> {
  await http.del<void>(`/sellers/${encodeURIComponent(id)}`)
}

// The characters Go's unicode.IsSpace accepts, which the server splits a
// name on. JavaScript's \s differs: it lacks U+0085 and has U+FEFF.
const SERVER_SPACE = /[\t\n\v\f\r \u0085\u00a0\u1680\u2000-\u200a\u2028\u2029\u202f\u205f\u3000]+/gu

// foldChar folds one code point the way golang.org/x/text/cases.Fold does.
// Lowercasing, uppercasing and lowercasing again yields the full case
// folding for nearly every character (ß, ẞ and the ligatures expand,
// final sigma becomes σ). The exceptions are spelled out: the dotless ı has
// no folding, and x/text folds the Cherokee small letters to capitals.
function foldChar(c: string): string {
  const cp = c.codePointAt(0) ?? 0
  if (cp === 0x131) return c
  if (cp >= 0xab70 && cp <= 0xabbf) return String.fromCodePoint(cp - 0xab70 + 0x13a0)
  if (cp >= 0x13f8 && cp <= 0x13fd) return String.fromCodePoint(cp - 8)
  return c.toLowerCase().toUpperCase().toLowerCase()
}

// sellerNameKey compares seller names the way the server does
// (models.SellerNameKey): NFC, full case folding, white space trimmed and
// collapsed. Checked against the server for every code point: the only
// differences are letters newer than the browser's Unicode tables, which
// the browser leaves unfolded. For those the server's 409 still applies.
export function sellerNameKey(name: string): string {
  const folded = Array.from(name.normalize("NFC"), foldChar).join("").normalize("NFC")
  return folded.replace(SERVER_SPACE, " ").replace(/^ | $/g, "")
}

// findSellerByName returns the seller whose name matches name the way the
// server's uniqueness rule does, if there is one.
export function findSellerByName(sellers: readonly Seller[], name: string): Seller | undefined {
  const key = sellerNameKey(name)
  if (!key) return undefined
  return sellers.find((s) => sellerNameKey(s.name ?? "") === key)
}
