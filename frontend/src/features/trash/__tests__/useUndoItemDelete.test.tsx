import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { renderHook, waitFor } from "@testing-library/react"
import { http, HttpResponse } from "msw"
import { type ReactNode } from "react"
// test/setup.ts mocks sonner with one spy per variant.
import { toast } from "sonner"
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest"

import { useUndoItemDelete } from "@/features/trash/useUndoItemDelete"
import { initI18n } from "@/i18n"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests, setCurrentGroupSlug } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { apiUrl } from "@/test/handlers"
import { server } from "@/test/server"

const SLUG = "household"

function wrapper({ children }: { children: ReactNode }) {
  const client = new QueryClient({ defaultOptions: { mutations: { retry: false } } })
  return <QueryClientProvider client={client}>{children}</QueryClientProvider>
}

beforeAll(async () => {
  await initI18n({ lng: "en" })
})

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
  setCurrentGroupSlug(SLUG)
  vi.mocked(toast.success).mockClear()
  vi.mocked(toast.warning).mockClear()
})

describe("useUndoItemDelete", () => {
  it("restores the deleted items when Undo is clicked", async () => {
    let body: unknown = null
    server.use(
      http.post(apiUrl(`/g/${SLUG}/trash/commodities/restore`), async ({ request }) => {
        body = await request.json()
        return HttpResponse.json({
          data: {
            type: "commodities.bulk_restore_result",
            id: "bulk-result",
            attributes: { succeeded: ["c1"], failed: [], area_removed: [] },
          },
        })
      })
    )
    const { result } = renderHook(() => useUndoItemDelete(), { wrapper })

    const action = result.current(["c1"])
    expect(action.label).toBe("Undo")
    action.onClick()

    await waitFor(() =>
      expect(body).toEqual({ data: { type: "commodities", attributes: { ids: ["c1"] } } })
    )
    await waitFor(() => expect(toast.success).toHaveBeenCalledWith("1 item restored.", undefined))
  })

  it("warns when some items could not be restored", async () => {
    server.use(
      http.post(apiUrl(`/g/${SLUG}/trash/commodities/restore`), () =>
        HttpResponse.json({
          data: {
            type: "commodities.bulk_restore_result",
            id: "bulk-result",
            attributes: {
              succeeded: ["c1"],
              failed: [{ id: "c2", error: "not found" }],
              area_removed: [],
            },
          },
        })
      )
    )
    const { result } = renderHook(() => useUndoItemDelete(), { wrapper })
    result.current(["c1", "c2"]).onClick()

    await waitFor(() => expect(toast.warning).toHaveBeenCalledWith("1 done, 1 failed.", undefined))
  })
})
