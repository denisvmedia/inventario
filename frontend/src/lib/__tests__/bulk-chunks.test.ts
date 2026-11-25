import { describe, expect, it } from "vitest"

import { sendInChunks } from "@/lib/bulk-chunks"

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

  it("throws when the first chunk fails, as a single request would", async () => {
    await expect(
      sendInChunks(ids(3), 2, async () => {
        throw new Error("boom")
      })
    ).rejects.toThrow("boom")
  })

  it("sends nothing for no ids", async () => {
    const out = await sendInChunks([], 2, async () => {
      throw new Error("not called")
    })
    expect(out).toEqual({ parts: [], unknown: [], unsent: [] })
  })
})
