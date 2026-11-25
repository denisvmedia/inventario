// sendInChunks sends ids in requests of at most size ids, one after the
// other, and returns each request's result in order.
//
// When a request fails after an earlier one went through, it stops there:
// the earlier requests changed their items, so their results are returned,
// with the ids it did not get to in `unsent` for the caller to report as
// failed. Throwing would lose what did change, and with it the Undo for it.
// A failure of the first request is thrown, since nothing changed yet and
// the caller handles it as a failed action.
export async function sendInChunks<P>(
  ids: string[],
  size: number,
  send: (chunk: string[]) => Promise<P>
): Promise<{ parts: P[]; unsent: string[] }> {
  const parts: P[] = []
  for (let i = 0; i < ids.length; i += size) {
    try {
      parts.push(await send(ids.slice(i, i + size)))
    } catch (err) {
      if (i === 0) throw err
      return { parts, unsent: ids.slice(i) }
    }
  }
  return { parts, unsent: [] }
}
