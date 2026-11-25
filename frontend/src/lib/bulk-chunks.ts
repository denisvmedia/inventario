// sendInChunks sends ids in requests of at most size ids, one after the
// other, and returns each request's result in order.
//
// When a request fails after an earlier one went through, it stops there and
// returns what it has: the earlier requests changed their items, and throwing
// would lose their results, and with them the Undo for those changes. The
// ids of the request that failed are `unknown`: the server may have changed
// some of them before the request failed or its answer was lost, so the
// caller reports them as items that may have changed, not as items that did
// not. The ids after it are `unsent`: no request carried them, so nothing
// changed them.
//
// A failure of the first request is thrown, so the caller handles it the way
// it handles a failed single request.
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
      if (i === 0) throw err
      return { parts, unknown: ids.slice(i, i + size), unsent: ids.slice(i + size) }
    }
  }
  return { parts, unknown: [], unsent: [] }
}
