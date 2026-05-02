// Re-exports + small helpers built on top of the auto-generated `api.d.ts`.
// Hand-written domain types should live in this folder alongside this file
// and re-export from here.
import type { components, paths } from "./api"

export type { components, paths }

// Convenience: pull a JSON:API resource shape out of the generated schemas.
// Example: `Schema<"Commodity">` → `components["schemas"]["Commodity"]`.
export type Schema<K extends keyof components["schemas"]> = components["schemas"][K]

// Successful HTTP statuses we treat as the "data" response shape. Numeric
// keys are how openapi-typescript emits them; string keys are kept as a
// safety net in case a generator pass switches representation.
type SuccessStatus =
  | 200
  | 201
  | 202
  | 203
  | 204
  | 205
  | 206
  | 207
  | 208
  | 226
  | "200"
  | "201"
  | "202"
  | "203"
  | "204"
  | "205"
  | "206"
  | "207"
  | "208"
  | "226"

// Most Inventario endpoints serve `application/vnd.api+json` (JSON:API), but
// a handful (e.g. /auth/login) use plain `application/json`. Try JSON:API
// first, fall back to plain JSON, so `ApiResponse<...>` resolves for the
// whole API surface instead of returning `never`.
type ResponseContent<R> = R extends { content: infer C }
  ? C extends Record<PropertyKey, unknown>
    ? "application/vnd.api+json" extends keyof C
      ? C["application/vnd.api+json"]
      : "application/json" extends keyof C
        ? C["application/json"]
        : never
    : never
  : never

// Convenience: pull the body of a successful response for a given path/method.
export type ApiResponse<P extends keyof paths, M extends keyof paths[P]> = paths[P][M] extends {
  responses: infer R
}
  ? R extends Record<PropertyKey, unknown>
    ? ResponseContent<R[Extract<keyof R, SuccessStatus>]>
    : never
  : never
