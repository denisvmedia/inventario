import { describe, expect, it } from "vitest"
import { http as msw, HttpResponse } from "msw"

import { http, HttpError, __resetHttpForTests } from "@/lib/http"
import { formatCurrency, formatDateTime } from "@/lib/intl"
import { server } from "@/test/server"

const api = (path: string) => `${window.location.origin}/api/v1${path}`

// #2127: a json content-type over a body that is not JSON threw a raw
// SyntaxError, which is not an HttpError — so the 503 maintenance bounce and
// the HttpError throw below it were both skipped.
describe("parseBody", () => {
  it("surfaces a malformed json body as an HttpError, not a SyntaxError", async () => {
    __resetHttpForTests()
    server.use(
      msw.get(api("/broken"), () =>
        HttpResponse.text("<html>502 Bad Gateway</html>", {
          status: 502,
          headers: { "content-type": "application/json" },
        })
      )
    )

    const err = await http.get("/broken").catch((e: unknown) => e)

    expect(err).toBeInstanceOf(HttpError)
    expect(err).not.toBeInstanceOf(SyntaxError)
  })
})

// #2127: Intl throws RangeError on a structurally invalid tag, and every
// price and date goes through it, so one bad stored preference whited out
// the screen.
describe("intl formatters", () => {
  it("formats rather than throwing on an invalid locale", () => {
    expect(() => formatCurrency(12.5, "USD", { locale: "en_US" })).not.toThrow()
    expect(() => formatDateTime(new Date("2026-01-02T03:04:05Z"), { locale: "c" })).not.toThrow()
  })
})
