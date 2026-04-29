// Re-exports + small helpers built on top of the auto-generated `api.d.ts`.
// Hand-written domain types should live in this folder alongside this file
// and re-export from here.
import type { components, paths } from "./api"

export type { components, paths }

// Convenience: pull a JSON:API resource shape out of the generated schemas.
// Example: `Schema<"Commodity">` → `components["schemas"]["Commodity"]`.
export type Schema<K extends keyof components["schemas"]> = components["schemas"][K]

// Convenience: pull the JSON body of a 200 response for a given path/method.
export type ApiResponse<
  P extends keyof paths,
  M extends keyof paths[P],
> = paths[P][M] extends {
  responses: { 200: { content: { "application/json": infer T } } }
}
  ? T
  : never
