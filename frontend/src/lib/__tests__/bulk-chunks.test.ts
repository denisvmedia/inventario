import { describe, expect, it } from "vitest"

import { sendInChunks } from "@/lib/bulk-chunks"
import { HttpError, NetworkError } from "@/lib/http"

const ids = (n: number) => Array.from({ length: n }, (_, i) => `id${i}`)

describe("sendInChunks", () => {
  it("sends the ids in chunks of the given size, in order", async () => {
    const sent: string[][] = []
    const out = await sendInChunks(ids(5), 2, async (chunk) => {
      sent.push(chunk)
      return chunk.length
    })
    expect(sent).toEqual([["id0", "id1"], ["id2", "id3"], ["id4"]])
    expect(out).toEqual({ parts: [2, 2, 1], unknown: [], unsent: [] })
  })

  it("keeps what earlier chunks did when a later one fails", async () => {
    let calls = 0
    const out = await sendInChunks(ids(7), 2, async (chunk) => {
      calls += 1
      if (calls === 2) throw new Error("boom")
      return chunk
    })
    expect(out.parts).toEqual([["id0", "id1"]])
    // The failed request may have changed some of its ids before it failed;
    // the ids after it were never sent.
    expect(out.unknown).toEqual(["id2", "id3"])
    expect(out.unsent).toEqual(["id4", "id5", "id6"])
    expect(calls).toBe(2)
  })

  it("has nothing unsent when the last chunk fails", async () => {
    let calls = 0
    const out = await sendInChunks(ids(3), 2, async (chunk) => {
      calls += 1
      if (calls === 2) throw new Error("boom")
      return chunk
    })
    expect(out).toEqual({ parts: [["id0", "id1"]], unknown: ["id2"], unsent: [] })
  })

  it("throws when the server refuses the first chunk, as a single request would", async () => {
    const refused = new HttpError("refused", 422, "/bulk", undefined)
    await expect(
      sendInChunks(ids(3), 2, async () => {
        throw refused
      })
    ).rejects.toBe(refused)
    await expect(
      sendInChunks(ids(3), 2, async () => {
        throw new Error("boom")
      })
    ).rejects.toThrow("boom")
  })

  it.each([
    ["gets no answer", new NetworkError("/bulk", new TypeError("offline"))],
    ["fails on the server", new HttpError("failed", 502, "/bulk", undefined)],
  ])("reports the first chunk as unknown when it %s", async (_, err) => {
    let calls = 0
    const out = await sendInChunks(ids(5), 2, async () => {
      calls += 1
      throw err
    })
    // The server may have changed some of the first chunk's ids before the
    // answer was lost; the ids after it were never sent.
    expect(out).toEqual({ parts: [], unknown: ["id0", "id1"], unsent: ["id2", "id3", "id4"] })
    expect(calls).toBe(1)
  })

  it("sends nothing for no ids", async () => {
    const out = await sendInChunks([], 2, async () => {
      throw new Error("not called")
    })
    expect(out).toEqual({ parts: [], unknown: [], unsent: [] })
  })
})
