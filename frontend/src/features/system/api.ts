import { http } from "@/lib/http"
import type { Schema } from "@/types"

// System/build info served by `GET /api/v1/system`. The `version`, `commit`
// and `build_date` fields are injected into the Go binary at build time via
// ldflags (see .goreleaser.yaml / Makefile); everything else is runtime.
export type SystemInfo = Schema<"apiserver.SystemInfo">

// `/system` is not group-scoped, so the http helper sends it verbatim under
// /api/v1 with no /g/{slug} rewrite.
export function getSystemInfo(signal?: AbortSignal): Promise<SystemInfo> {
  return http.get<SystemInfo>("/system", { signal })
}
