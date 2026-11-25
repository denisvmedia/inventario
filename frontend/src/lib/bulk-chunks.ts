import { groupScopedPath, http, HttpError } from "@/lib/http"

// sendInChunks sends ids in requests of at most size ids, one after the
// other, and returns each request's result in order.
//
// When a request fails, it stops there and returns what it has: the earlier
// requests changed their items, and throwing would lose their results, and
// with them the Undo for those changes. The ids of the request that failed
// are `unknown`: the server may have changed some of them before the request
// failed or its answer was lost, so the caller reports them as items that
// may have changed, not as items that did not. The ids after it are
// `unsent`: no request carried them, so nothing changed them.
//
// The first request is the exception when the server refused it (any answer
// but a 5xx): nothing changed then, so the error is thrown and the caller
// handles it as it handles a refused single request. When it got no answer
// or a 5xx, its ids are `unknown` as above.
export async function sendInChunks<P>(
  ids: string[],
  size: number,
  send: (chunk: string[]) => Promise<P>
): Promise<{ parts: P[]; unknown: string[]; unsent: string[] }> {
  const parts: P[] = []
  for (let i = 0; i < ids.length; i += size) {
    try {
      parts.push(await send(ids.slice(i, i + size)))
    } catch (err) {
      if (i === 0 && !outcomeUnknown(err)) throw err
      return { parts, unknown: ids.slice(i, i + size), unsent: ids.slice(i + size) }
    }
  }
  return { parts, unknown: [], unsent: [] }
}

// outcomeUnknown tells a request that may have changed items before it
// failed (no answer at all, or a server error) from one the server refused.
function outcomeUnknown(err: unknown): boolean {
  return err instanceof HttpError && (err.status === 0 || err.status >= 500)
}

// pinnedPost returns a POST to path in the group active now. All requests of
// one chunked action go through it, so they reach the group the action
// started in, even when the user switches groups while it runs.
export function pinnedPost<T>(path: string): (body: unknown) => Promise<T> {
  const pinned = groupScopedPath(path)
  return (body) => http.post<T>(pinned, body, { skipGroupRewrite: true })
}
