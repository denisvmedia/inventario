import { beforeEach, describe, expect, it } from "vitest"
import { MemoryRouter, Route, Routes } from "react-router-dom"
import { renderHook, waitFor } from "@testing-library/react"
import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { type ReactElement, type ReactNode } from "react"

import { GroupProvider } from "@/features/group/GroupContext"
import { type SettingsObject } from "@/features/settings/api"
import { usePatchSetting, useUserSettings } from "@/features/settings/hooks"
import { settingsKeys } from "@/features/settings/keys"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"
import { groupHandlers, settingsHandlers } from "@/test/handlers"
import { server } from "@/test/server"

function makeWrapper(client: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={client}>
        <MemoryRouter initialEntries={["/g/household"]}>
          <Routes>
            <Route
              path="/g/:groupSlug"
              element={<GroupProvider>{children as ReactElement}</GroupProvider>}
            />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>
    )
  }
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
  setAccessToken("good-token")
})

describe("usePatchSetting", () => {
  // The settings belong to the user; the group in the URL only routes the
  // request. A copy cached under another group must not keep the old value.
  it("writes the saved settings to every group's cached copy", async () => {
    const fake = settingsHandlers.backend("household", { whatsNewLastSeenVersion: "0.1.0" })
    server.use(...groupHandlers.list(), ...fake.handlers)
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    })
    client.setQueryData<SettingsObject>(settingsKeys.preferences("office"), {
      whatsNewLastSeenVersion: "0.1.0",
    })
    const { result } = renderHook(
      () => ({ settings: useUserSettings(), patch: usePatchSetting() }),
      {
        wrapper: makeWrapper(client),
      }
    )
    await waitFor(() => expect(result.current.settings.data).toBeDefined())

    result.current.patch.mutate({ field: "whats_new.last_seen_version", value: "0.2.2" })

    await waitFor(() =>
      expect(client.getQueryData(settingsKeys.preferences("office"))).toEqual({
        whatsNewLastSeenVersion: "0.2.2",
      })
    )
    expect(client.getQueryData(settingsKeys.preferences("household"))).toEqual({
      whatsNewLastSeenVersion: "0.2.2",
    })
  })
})
