import { useSystemInfo } from "@/features/system/hooks"

// A faint, non-interactive watermark pinned to the bottom-right corner that
// shows the build commit short-hash (#1972). The backend is the source of
// truth: the hash is injected into the binary at build time and surfaced via
// `GET /api/v1/system`; this component only renders it.
//
// Mounted inside the authenticated Shell, so it's auth-gated for free and
// never appears on /login or other public pages. Hidden on mobile.
//
// goreleaser injects the full 40-char SHA while `make` injects the 7-char
// short hash — we truncate to 7 for display and keep the full SHA (plus the
// version) in the tooltip, which works for either build path. In dev / tests
// the binary reports "unknown", so we render nothing.
export function CommitBadge() {
  const { data } = useSystemInfo()
  const commit = data?.commit
  if (!commit || commit === "unknown") {
    return null
  }

  const short = commit.slice(0, 7)
  return (
    <div
      aria-hidden="true"
      title={data?.version ? `${data.version} · ${commit}` : commit}
      className="pointer-events-none fixed right-2 bottom-1.5 z-40 hidden font-mono text-[10px] leading-none text-muted-foreground/50 select-none sm:block"
    >
      {short}
    </div>
  )
}
