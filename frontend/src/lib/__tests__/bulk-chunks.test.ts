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
    expect(out).toEqual({ parts: [2, 2, 1], unsent: [] })
  })

  it("keeps what earlier chunks did when a later one fails", async () => {
    let calls = 0
    const out = await sendInChunks(ids(5), 2, async (chunk) => {
      calls += 1
      if (calls === 2) throw new Error("boom")
      return chunk
    })
    expect(out.parts).toEqual([["id0", "id1"]])
    expect(out.unsent).toEqual(["id2", "id3", "id4"])
    expect(calls).toBe(2)
  })

  it("throws when the first chunk fails, since nothing changed", async () => {
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
    expect(out).toEqual({ parts: [], unsent: [] })
  })
})
