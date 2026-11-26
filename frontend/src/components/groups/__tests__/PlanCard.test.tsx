import { afterEach, beforeEach, describe, expect, it } from "vitest"
import { delay, http as msw, HttpResponse } from "msw"
import { screen, waitFor } from "@testing-library/react"

import { PlanCard } from "@/components/groups/PlanCard"
import { FEATURE_FLAGS_FALLBACK } from "@/features/feature-flags/api"
import { featureFlagsKeys } from "@/features/feature-flags/keys"
import { planKeys } from "@/features/plan/keys"
import { apiUrl, featureFlagHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetHttpForTests } from "@/lib/http"

const planPayload = {
  plan: {
    id: "free",
    name: "Free",
    max_items: 500,
    max_locations: 10,
    max_storage_bytes: null,
    allows_restore: false,
    allows_api_access: false,
  },
  usage: { items: 7, locations: 2, storage_bytes: 12_582_912 },
}

function renderCard() {
  return renderWithProviders({
    children: <PlanCard groupSlug="household" ownerName="Alex" />,
  })
}

beforeEach(() => {
  clearAuth()
  __resetHttpForTests()
  setAccessToken("good-token")
})

afterEach(() => {
  clearAuth()
})

describe("<PlanCard /> and the plans flag (#1389)", () => {
  it("keeps the skeleton while the flags load, then draws the plan card", async () => {
    // The flags answer only when the test lets them; the plan answers at
    // once. Without the wait on the flags, the card would draw the usage
    // card first (plans read as off) and swap to the plan card later.
    let releaseFlags: () => void = () => {}
    const flagsGate = new Promise<void>((resolve) => {
      releaseFlags = resolve
    })
    server.use(
      msw.get(apiUrl("/feature-flags"), async () => {
        await flagsGate
        return HttpResponse.json({ ...FEATURE_FLAGS_FALLBACK, plans: true })
      }),
      msw.get(apiUrl("/g/household/plan"), () => HttpResponse.json(planPayload))
    )

    const { queryClient } = renderCard()
    await waitFor(() =>
      expect(queryClient.getQueryState(planKeys.group("household"))?.status).toBe("success")
    )
    expect(queryClient.getQueryState(featureFlagsKeys.all)?.status).toBe("pending")
    expect(screen.getByTestId("plan-card-skeleton")).toBeInTheDocument()
    expect(screen.queryByTestId("usage-card")).not.toBeInTheDocument()

    releaseFlags()
    expect(await screen.findByTestId("plan-card")).toBeInTheDocument()
    expect(screen.queryByTestId("usage-card")).not.toBeInTheDocument()
  })

  it("draws a skeleton without the plan parts when plans are off", async () => {
    server.use(
      ...featureFlagHandlers.set({ plans: false }),
      msw.get(apiUrl("/g/household/plan"), async () => {
        await delay("infinite")
        return HttpResponse.json(planPayload)
      })
    )

    const { queryClient } = renderCard()
    await waitFor(() =>
      expect(queryClient.getQueryState(featureFlagsKeys.all)?.status).toBe("success")
    )
    expect(screen.getByTestId("plan-card-skeleton")).toHaveAttribute("data-with-plan", "false")
  })

  it("draws a skeleton with the plan parts when plans are on", async () => {
    server.use(
      ...featureFlagHandlers.set({ plans: true }),
      msw.get(apiUrl("/g/household/plan"), async () => {
        await delay("infinite")
        return HttpResponse.json(planPayload)
      })
    )

    const { queryClient } = renderCard()
    await waitFor(() =>
      expect(queryClient.getQueryState(featureFlagsKeys.all)?.status).toBe("success")
    )
    await waitFor(() =>
      expect(screen.getByTestId("plan-card-skeleton")).toHaveAttribute("data-with-plan", "true")
    )
  })
})
