// Pure data-layer for the currency-migration feature slice (epic #202 /
// issue #1553). Backed by the four endpoints under
// /api/v1/g/{groupSlug}/currency-migrations exposed by PR #1584:
//
//   POST   /currency-migrations/preview   → CurrencyMigrationPreviewResponse
//   POST   /currency-migrations           → start (returns CurrencyMigrationResponse)
//   GET    /currency-migrations           → list (latest first, no pagination yet)
//   GET    /currency-migrations/{id}      → detail
//
// The /g/{slug}/ rewrite happens in lib/http.ts via the
// `/currency-migrations` prefix entry; callers pass the bare path.
import { http } from "@/lib/http"
import type { Schema } from "@/types"

export type CurrencyMigration = Schema<"models.CurrencyMigration">
export type CurrencyMigrationStatus = Schema<"models.CurrencyMigrationStatus">
export type CurrencyMigrationPreviewBody = Schema<"jsonapi.CurrencyMigrationPreviewBody">
export type CurrencyMigrationPreviewDiff = Schema<"jsonapi.CurrencyMigrationPreviewDiff">
export type CurrencyMigrationStartAttributes = Schema<"jsonapi.CurrencyMigrationStartAttributes">
export type CurrencyMigrationPreviewAttributes =
  Schema<"jsonapi.CurrencyMigrationPreviewAttributes">

export const CURRENCY_MIGRATION_STATUSES = [
  "pending",
  "running",
  "completed",
  "failed",
] as const satisfies readonly CurrencyMigrationStatus[]

const TERMINAL_STATUSES: ReadonlySet<CurrencyMigrationStatus> = new Set(["completed", "failed"])

export function isCurrencyMigrationTerminal(status: CurrencyMigrationStatus | undefined): boolean {
  return !!status && TERMINAL_STATUSES.has(status)
}

export function isCurrencyMigrationActive(status: CurrencyMigrationStatus | undefined): boolean {
  return status === "pending" || status === "running"
}

interface MigrationEnvelope {
  id?: string
  type?: string
  attributes?: CurrencyMigration
}

interface MigrationListEnvelope {
  data?: MigrationEnvelope[]
}

interface MigrationDetailEnvelope {
  data?: MigrationEnvelope
}

interface PreviewEnvelope {
  data?: {
    id?: string
    type?: string
    attributes?: CurrencyMigrationPreviewBody
  }
}

// Identity-resolved type — id is non-optional in TS even though the
// generated schema marks it as optional. Same pattern as Export/Restore.
export type Migration = CurrencyMigration & { id: string }
export type MigrationPreview = CurrencyMigrationPreviewBody

function resolveMigration(envelope: MigrationEnvelope, fallbackId?: string): Migration {
  if (!envelope.attributes) {
    throw new Error("Malformed currency-migrations response: missing attributes")
  }
  const id = envelope.id ?? envelope.attributes.id ?? fallbackId
  if (!id) {
    throw new Error("Malformed currency-migrations response: missing id")
  }
  return { ...envelope.attributes, id }
}

export async function listMigrations(signal?: AbortSignal): Promise<{ migrations: Migration[] }> {
  const body = await http.get<MigrationListEnvelope>("/currency-migrations", { signal })
  return { migrations: (body.data ?? []).map((row) => resolveMigration(row)) }
}

export async function getMigration(id: string, signal?: AbortSignal): Promise<Migration> {
  const body = await http.get<MigrationDetailEnvelope>(
    `/currency-migrations/${encodeURIComponent(id)}`,
    { signal }
  )
  if (!body.data) throw new Error("Malformed currency-migration detail response: missing data")
  return resolveMigration(body.data, id)
}

export interface PreviewRequest {
  from_currency: string
  to_currency: string
  exchange_rate: number
}

export async function previewMigration(req: PreviewRequest): Promise<MigrationPreview> {
  const body = await http.post<PreviewEnvelope>("/currency-migrations/preview", {
    data: {
      type: "currency-migrations",
      attributes: req,
    },
  })
  if (!body.data?.attributes) {
    throw new Error("Malformed currency-migration preview response: missing attributes")
  }
  return body.data.attributes
}

export interface StartRequest {
  from_currency: string
  to_currency: string
  exchange_rate: number
  preview_token: string
}

export async function startMigration(req: StartRequest): Promise<Migration> {
  const body = await http.post<MigrationDetailEnvelope>("/currency-migrations", {
    data: {
      type: "currency-migrations",
      attributes: req,
    },
  })
  if (!body.data) throw new Error("Malformed currency-migration start response: missing data")
  return resolveMigration(body.data)
}
