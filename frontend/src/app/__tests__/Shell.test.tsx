import { afterEach, beforeEach, describe, expect, it, vi } from "vitest"
import { Route } from "react-router-dom"
import { act, screen, waitFor } from "@testing-library/react"
import userEvent from "@testing-library/user-event"

import { Shell } from "@/app/Shell"
import { AuthProvider, useAuth } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { TOUR_BLOCKER_GRACE_MS, useOnboardingTourControls } from "@/hooks/useOnboardingTourControls"
import { authHandlers, groupHandlers } from "@/test/handlers"
import { renderWithProviders } from "@/test/render"
import { server } from "@/test/server"
import { clearAuth, setAccessToken } from "@/lib/auth-storage"
import { __resetGroupContextForTests } from "@/lib/group-context"
import { __resetHttpForTests } from "@/lib/http"

// #2787 — the onboarding tour points at the sidebar's group entries. These
// tests mount the real Shell (sidebar + tour overlay) in the no-group state
// and with a group, and check what the tour does in each.

const SEEN_KEY = `inventario-tour-seen-v1:${authHandlers.fixtureUser.id}`

// The suite-wide matchMedia stub from test/setup.ts. A test that needs a
// narrow screen swaps in its own and afterEach puts this one back. It is a
// shared vi.fn, so vi.spyOn + mockRestore would wipe its implementation.
const originalMatchMedia = window.matchMedia

// Swaps in a matchMedia whose mobile (max-width) query answers from a
// switch. set() flips it and fires the change listeners, which is what a
// resize across the breakpoint does.
function mockMobileQuery() {
  let narrow = false
  const listeners = new Set<() => void>()
  window.matchMedia = vi.fn().mockImplementation((query: string) => {
    const isMobileQuery = query.includes("max-width")
    return {
      get matches() {
        return isMobileQuery && narrow
      },
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: (_type: string, cb: () => void) => {
        if (isMobileQuery) listeners.add(cb)
      },
      removeEventListener: (_type: string, cb: () => void) => {
        listeners.delete(cb)
      },
      dispatchEvent: vi.fn(),
    } as unknown as MediaQueryList
  })
  return {
    set(width: "narrow" | "wide") {
      act(() => {
        narrow = width === "narrow"
        listeners.forEach((cb) => cb())
      })
    },
  }
}

// Stands in for the routed page. It exposes the tour controls Shell
// provides, so a test can read the blocker and force a restart the UI
// does not offer.
function TourProbe() {
  const { user } = useAuth()
  const controls = useOnboardingTourControls()
  return (
    <div>
      <p data-testid="probe-user">{user?.id ?? ""}</p>
      <p data-testid="probe-blocker">{controls.blocker ?? "none"}</p>
      <button type="button" data-testid="probe-restart" onClick={controls.restart}>
        restart
      </button>
    </div>
  )
}

function renderShell(initialPath: string) {
  setAccessToken("good-token")
  return renderWithProviders({
    initialPath,
    routes: (
      <Route
        element={
          <AuthProvider>
            <GroupProvider>
              <Shell />
            </GroupProvider>
          </AuthProvider>
        }
      >
        <Route path="/no-group" element={<TourProbe />} />
        <Route path="/g/:groupSlug" element={<TourProbe />} />
      </Route>
    ),
  })
}

beforeEach(() => {
  clearAuth()
  __resetGroupContextForTests()
  __resetHttpForTests()
})

afterEach(() => {
  window.matchMedia = originalMatchMedia
})

describe("<Shell /> — onboarding tour without a group (#2787)", () => {
  it("does not open the tour, does not mark it seen, and offers no restart", async () => {
    server.use(...authHandlers.signedIn(), ...groupHandlers.empty())
    const user = userEvent.setup()
    renderShell("/no-group")

    // Wait until the user is known and the groups list resolved empty: the
    // group sections retract only then.
    await waitFor(() => expect(screen.getByTestId("probe-user")).toHaveTextContent("u1"))
    await waitFor(() =>
      expect(screen.queryByTestId("sidebar-inventory-group")).not.toBeInTheDocument()
    )
    expect(screen.getByTestId("probe-blocker")).toHaveTextContent("noGroup")
    expect(screen.queryByTestId("onboarding-tour")).not.toBeInTheDocument()
    expect(window.localStorage.getItem(SEEN_KEY)).toBeNull()

    // The user menu leaves the restart entry out instead of offering one
    // that cannot show anything.
    await user.click(screen.getByTestId("user-menu"))
    expect(await screen.findByTestId("sign-out")).toBeInTheDocument()
    expect(screen.queryByTestId("restart-tour")).not.toBeInTheDocument()
  })

  it("closes a forced restart at once without marking the tour seen", async () => {
    // Nothing in the UI restarts the tour without a group, but the restart
    // handler still exists; with nothing on the page to point at, the tour
    // must close at once rather than walk through centered cards.
    //
    // The user saw the tour before; restart() clears that flag, so a null
    // flag afterwards shows that the restart ran and that closing did not
    // count as a skip or a finish, both of which would set it again.
    window.localStorage.setItem(SEEN_KEY, "1")
    server.use(...authHandlers.signedIn(), ...groupHandlers.empty())
    const user = userEvent.setup()
    renderShell("/no-group")
    await waitFor(() => expect(screen.getByTestId("probe-blocker")).toHaveTextContent("noGroup"))
    await waitFor(() =>
      expect(screen.queryByTestId("sidebar-inventory-group")).not.toBeInTheDocument()
    )

    await user.click(screen.getByTestId("probe-restart"))

    await waitFor(() => expect(window.localStorage.getItem(SEEN_KEY)).toBeNull())
    expect(screen.queryByTestId("onboarding-tour")).not.toBeInTheDocument()
    expect(screen.queryByTestId("onboarding-card")).not.toBeInTheDocument()
  })
})

describe("<Shell /> — onboarding tour with a group", () => {
  it("auto-launches and highlights the sidebar's Add item button", async () => {
    server.use(...authHandlers.signedIn(), ...groupHandlers.list())
    const user = userEvent.setup()
    renderShell("/g/household")

    expect(await screen.findByTestId("onboarding-tour")).toBeInTheDocument()
    expect(screen.getByTestId("onboarding-card")).toHaveAttribute("data-step", "welcome")
    expect(screen.getByTestId("probe-blocker")).toHaveTextContent("none")
    // Every step has its target with a group: seven dots.
    expect(screen.getByTestId("onboarding-dots").children).toHaveLength(7)

    // The overlay ignores pointer events during its short fade-in.
    await waitFor(() =>
      expect(screen.getByTestId("onboarding-tour").style.pointerEvents).not.toBe("none")
    )
    await user.click(screen.getByTestId("onboarding-next"))
    expect(screen.getByTestId("onboarding-card")).toHaveAttribute("data-step", "addItem")
    expect(await screen.findByTestId("onboarding-highlight")).toBeInTheDocument()

    await user.click(screen.getByTestId("onboarding-skip-icon"))
    await waitFor(() => expect(screen.queryByTestId("onboarding-tour")).not.toBeInTheDocument())
    expect(window.localStorage.getItem(SEEN_KEY)).toBe("1")
  })

  it("closes an open tour without marking it seen when the window stays narrow", async () => {
    // Under WebDriver the tour does not launch by itself, so a tour that is
    // gone after the window widens again was closed, not just hidden.
    Object.defineProperty(window.navigator, "webdriver", { configurable: true, value: true })
    vi.useFakeTimers({ shouldAdvanceTime: true })
    try {
      const screenWidth = mockMobileQuery()
      server.use(...authHandlers.signedIn(), ...groupHandlers.list())
      const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
      renderShell("/g/household")
      // restart() needs the user (the seen flag is per user) and the group
      // (the steps point at its sidebar entries), so wait for both first.
      await waitFor(() =>
        expect(screen.getByTestId("probe-user")).toHaveTextContent(
          String(authHandlers.fixtureUser.id)
        )
      )
      expect(await screen.findByTestId("sidebar-inventory-group")).toBeInTheDocument()
      await user.click(screen.getByTestId("probe-restart"))
      expect(await screen.findByTestId("onboarding-tour")).toBeInTheDocument()

      screenWidth.set("narrow")
      expect(screen.queryByTestId("onboarding-tour")).not.toBeInTheDocument()
      expect(screen.getByTestId("probe-blocker")).toHaveTextContent("narrowScreen")

      await act(() => vi.advanceTimersByTimeAsync(TOUR_BLOCKER_GRACE_MS + 100))
      screenWidth.set("wide")

      expect(screen.getByTestId("probe-blocker")).toHaveTextContent("none")
      expect(screen.queryByTestId("onboarding-tour")).not.toBeInTheDocument()
      expect(window.localStorage.getItem(SEEN_KEY)).toBeNull()
    } finally {
      vi.useRealTimers()
      Object.defineProperty(window.navigator, "webdriver", { configurable: true, value: false })
    }
  })

  it("keeps an open tour on its step when the window is narrow for a moment only", async () => {
    // Chromium shrinks the page to 1x1 while it takes a full-page
    // screenshot, and a window dragged across the breakpoint and back
    // passes it twice. Neither may close the tour or send it back to the
    // welcome card.
    const screenWidth = mockMobileQuery()
    server.use(...authHandlers.signedIn(), ...groupHandlers.list())
    const user = userEvent.setup()
    renderShell("/g/household")
    expect(await screen.findByTestId("onboarding-tour")).toBeInTheDocument()
    await waitFor(() =>
      expect(screen.getByTestId("onboarding-tour").style.pointerEvents).not.toBe("none")
    )
    await user.click(screen.getByTestId("onboarding-next"))
    expect(screen.getByTestId("onboarding-card")).toHaveAttribute("data-step", "addItem")

    screenWidth.set("narrow")
    expect(screen.queryByTestId("onboarding-tour")).not.toBeInTheDocument()
    screenWidth.set("wide")

    expect(screen.getByTestId("onboarding-card")).toHaveAttribute("data-step", "addItem")
    expect(screen.getByTestId("onboarding-dots").children).toHaveLength(7)
    expect(await screen.findByTestId("onboarding-highlight")).toBeInTheDocument()
    expect(window.localStorage.getItem(SEEN_KEY)).toBeNull()
  })

  it("offers Restart product tour in the user menu", async () => {
    window.localStorage.setItem(SEEN_KEY, "1")
    server.use(...authHandlers.signedIn(), ...groupHandlers.list())
    const user = userEvent.setup()
    renderShell("/g/household")
    await screen.findByTestId("sidebar-add-item")
    expect(screen.queryByTestId("onboarding-tour")).not.toBeInTheDocument()

    await user.click(screen.getByTestId("user-menu"))
    await user.click(await screen.findByTestId("restart-tour"))

    expect(await screen.findByTestId("onboarding-tour")).toBeInTheDocument()
    expect(screen.getByTestId("onboarding-card")).toHaveAttribute("data-step", "welcome")
    expect(window.localStorage.getItem(SEEN_KEY)).toBeNull()
  })
})
