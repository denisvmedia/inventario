import { http, HttpResponse, type HttpHandler } from "msw"

import type { SettingsObject } from "@/features/settings/api"

import { apiUrl } from "."

// MSW fake of the per-user settings (GET /g/{slug}/settings and PATCH
// /g/{slug}/settings/{field}). The state lives in the returned object, so a
// test can seed it and read what the client wrote.
//
//   const fake = settingsHandlers.backend("household", { whatsNewLastSeenVersion: "0.2.0" })
//   server.use(...fake.handlers)
//   expect(fake.patches).toEqual([{ field: "whats_new.last_seen_version", value: "0.2.2" }])

// The setting paths a test may PATCH, with the JSON field GET answers with.
// Extend it when a test needs another one.
const FIELDS: Record<string, keyof SettingsObject> = {
  "whats_new.last_seen_version": "whatsNewLastSeenVersion",
}

export interface SettingsBackend {
  state: SettingsObject
  patches: Array<{ field: string; value: unknown }>
  // How many times GET answered, so a test can wait for the read.
  reads: number
  handlers: HttpHandler[]
}

export function backend(slug: string, initial: SettingsObject = {}): SettingsBackend {
  const fake: SettingsBackend = {
    state: { ...initial },
    patches: [],
    reads: 0,
    handlers: [
      http.get(apiUrl(`/g/${encodeURIComponent(slug)}/settings`), () => {
        fake.reads++
        return HttpResponse.json(fake.state)
      }),
      http.patch(
        apiUrl(`/g/${encodeURIComponent(slug)}/settings/:field`),
        async ({ params, request }) => {
          const field = String(params.field)
          const value: unknown = await request.json()
          fake.patches.push({ field, value })
          const key = FIELDS[field]
          if (!key) return new HttpResponse(`unknown setting ${field}`, { status: 400 })
          const next: Record<string, unknown> = { ...fake.state }
          if (value === null) delete next[key]
          else next[key] = value
          fake.state = next as SettingsObject
          return HttpResponse.json(fake.state)
        }
      ),
    ],
  }
  return fake
}
