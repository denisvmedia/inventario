import { useEffect, useState } from "react"
import { beforeEach, describe, expect, it } from "vitest"
import { act, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { LocationFormDialog } from "@/components/locations/LocationFormDialog"
import { renderWithProviders } from "@/test/render"
import type { Location } from "@/features/locations/api"

// Direct-mount tests for LocationFormDialog. The host pages
// (LocationsListPage / LocationDetailPage) wire the dialog into
// useLocation/useUpdateLocation; here we exercise the dialog as a
// pure component so the reset/prefill behavior is exercised without
// staging an entire mutation cycle. See #1662 for the inline-error
// race the reset logic exists to fix, and the Copilot review on
// PR #1666 for the deep-link-:id-change case the "swaps form
// contents …" test pins down.

interface Controls {
  setLocation: (loc: Location | null | undefined) => void
}

// Renders the dialog with a host that exposes setState through a
// `bind` callback. Direct buttons would trip Radix Dialog's
// outside-click handler and close the modal; the imperative
// setState path lets the test mutate `location` without simulating
// a click outside the modal.
function ControlledHost({
  initialLocation,
  bind,
}: {
  initialLocation: Location | null | undefined
  bind: (c: Controls) => void
}) {
  const [loc, setLoc] = useState<Location | null | undefined>(initialLocation)
  useEffect(() => {
    bind({ setLocation: setLoc })
  }, [bind])
  return (
    <LocationFormDialog
      open
      onOpenChange={() => undefined}
      location={loc}
      onSubmit={async () => undefined}
    />
  )
}

beforeEach(() => {
  // Cleanup happens implicitly via React Testing Library's
  // auto-cleanup; no shared mutable state to reset here.
})

describe("<LocationFormDialog />", () => {
  it("prefills from the location prop on mount", async () => {
    renderWithProviders({
      children: (
        <ControlledHost
          initialLocation={{
            id: "loc-a",
            name: "Alpha",
            address: "Alpha St",
            icon: "🅰️",
            description: "alpha desc",
          }}
          bind={() => undefined}
        />
      ),
    })
    const nameInput = await screen.findByTestId("location-name-input")
    await waitFor(() => expect(nameInput).toHaveValue("Alpha"))
    expect(screen.getByTestId("location-address-input")).toHaveValue("Alpha St")
  })

  it("resets to the new location when `id` changes while the dialog stays open (#1666 Copilot review)", async () => {
    const user = userEvent.setup()
    let controls: Controls | null = null
    renderWithProviders({
      children: (
        <ControlledHost
          initialLocation={{
            id: "loc-a",
            name: "Alpha",
            address: "Alpha St",
            icon: "🅰️",
            description: "",
          }}
          bind={(c) => {
            controls = c
          }}
        />
      ),
    })
    const nameInput = await screen.findByTestId("location-name-input")
    await waitFor(() => expect(nameInput).toHaveValue("Alpha"))
    // User types something; without the id-change reset that would
    // ride along with the saved data into the next location.
    await user.type(nameInput, " edited")
    expect(nameInput).toHaveValue("Alpha edited")
    // Deep-link nav to /locations/loc-b/edit: same dialog instance,
    // different `location.id`. The form MUST reset to loc-b's
    // values — saving as-is would send loc-a's typed name to
    // /locations/loc-b which is a real data-corruption foot-gun.
    act(() => {
      controls!.setLocation({
        id: "loc-b",
        name: "Beta",
        address: "Beta Ave",
        icon: "🅱️",
        description: "",
      })
    })
    await waitFor(() => expect(nameInput).toHaveValue("Beta"))
    expect(screen.getByTestId("location-address-input")).toHaveValue("Beta Ave")
  })

  it("does NOT reset on a same-id reference change (preserves typing + inline error)", async () => {
    const user = userEvent.setup()
    let controls: Controls | null = null
    const initial: Location = {
      id: "loc-a",
      name: "Alpha",
      address: "",
      icon: "",
      description: "",
    }
    renderWithProviders({
      children: (
        <ControlledHost
          initialLocation={initial}
          bind={(c) => {
            controls = c
          }}
        />
      ),
    })
    const nameInput = await screen.findByTestId("location-name-input")
    await waitFor(() => expect(nameInput).toHaveValue("Alpha"))
    await user.clear(nameInput)
    await user.type(nameInput, "Mid-edit")
    expect(nameInput).toHaveValue("Mid-edit")
    // Same id, fresh ref — models the optimistic-update /
    // onSettled-refetch reference churn that previously wiped the
    // user's typing (and the inline serverError) on every render.
    act(() => {
      controls!.setLocation({ ...initial })
    })
    // Tick once for the effect to run; nameInput must still hold
    // the typed-but-unsaved value.
    await Promise.resolve()
    expect(nameInput).toHaveValue("Mid-edit")
  })
})
