import { useState } from "react"
import { afterEach, describe, expect, it, vi } from "vitest"
import { render, screen, fireEvent, waitFor } from "@testing-library/react"

import { OnboardingTour, TOUR_STEPS, resolveShownSteps } from "@/components/OnboardingTour"

// The tour shows a step only when its `[data-tour=…]` target is in the
// DOM (the centered welcome step needs none), so each test mounts the
// targets it wants the tour to find. In the app they live in AppSidebar.
const ALL_TARGETS = [
  "welcome",
  "add-item",
  "nav-dashboard",
  "nav-locations",
  "nav-items",
  "nav-warranties",
  "nav-files",
]

function mountTargets(keys: string[] = ALL_TARGETS) {
  const container = document.createElement("div")
  container.setAttribute("data-testid", "tour-targets")
  for (const key of keys) {
    const el = document.createElement("button")
    el.setAttribute("data-tour", key)
    el.textContent = key
    container.appendChild(el)
  }
  document.body.appendChild(container)
}

afterEach(() => {
  document.querySelectorAll("[data-testid='tour-targets']").forEach((el) => el.remove())
})

function renderTour(stepIndex: number, handlers: Partial<Record<string, () => void>> = {}) {
  const onNext = vi.fn()
  const onPrev = vi.fn()
  const onFinish = vi.fn()
  const onSkip = vi.fn()
  const onClose = vi.fn()
  render(
    <OnboardingTour
      step={stepIndex}
      onNext={handlers.onNext ?? onNext}
      onPrev={handlers.onPrev ?? onPrev}
      onFinish={handlers.onFinish ?? onFinish}
      onSkip={handlers.onSkip ?? onSkip}
      onClose={handlers.onClose ?? onClose}
    />
  )
  return { onNext, onPrev, onFinish, onSkip, onClose }
}

// Drives the tour the way Shell does (step state + next/prev), so a test
// can click through it and see which steps it shows.
function StatefulTour({ onFinish, onClose }: { onFinish: () => void; onClose: () => void }) {
  const [step, setStep] = useState(0)
  return (
    <OnboardingTour
      step={step}
      onNext={() => setStep((s) => s + 1)}
      onPrev={() => setStep((s) => Math.max(0, s - 1))}
      onFinish={onFinish}
      onSkip={() => {}}
      onClose={onClose}
    />
  )
}

describe("OnboardingTour", () => {
  it("renders the welcome step with title, description, skip + next CTAs", () => {
    mountTargets()
    renderTour(0)
    expect(screen.getByTestId("onboarding-tour")).toBeInTheDocument()
    expect(screen.getByTestId("onboarding-title")).toHaveTextContent(/welcome/i)
    expect(screen.getByTestId("onboarding-skip")).toBeInTheDocument()
    expect(screen.getByTestId("onboarding-next")).toBeInTheDocument()
    expect(screen.queryByTestId("onboarding-prev")).not.toBeInTheDocument()
    expect(screen.queryByTestId("onboarding-done")).not.toBeInTheDocument()
  })

  it("renders the final step with Back + Done (no Next)", () => {
    mountTargets()
    renderTour(TOUR_STEPS.length - 1)
    expect(screen.getByTestId("onboarding-prev")).toBeInTheDocument()
    expect(screen.getByTestId("onboarding-done")).toBeInTheDocument()
    expect(screen.queryByTestId("onboarding-next")).not.toBeInTheDocument()
    expect(screen.queryByTestId("onboarding-skip")).not.toBeInTheDocument()
  })

  it("calls onNext when Next is clicked", () => {
    mountTargets()
    const { onNext } = renderTour(1)
    fireEvent.click(screen.getByTestId("onboarding-next"))
    expect(onNext).toHaveBeenCalledOnce()
  })

  it("calls onPrev when Back is clicked", () => {
    mountTargets()
    const { onPrev } = renderTour(2)
    fireEvent.click(screen.getByTestId("onboarding-prev"))
    expect(onPrev).toHaveBeenCalledOnce()
  })

  it("calls onFinish when Done is clicked on the final step", () => {
    mountTargets()
    const { onFinish } = renderTour(TOUR_STEPS.length - 1)
    fireEvent.click(screen.getByTestId("onboarding-done"))
    expect(onFinish).toHaveBeenCalledOnce()
  })

  it("calls onSkip when Escape is pressed", () => {
    mountTargets()
    const { onSkip } = renderTour(0)
    fireEvent.keyDown(document, { key: "Escape" })
    expect(onSkip).toHaveBeenCalledOnce()
  })

  it("calls onNext when ArrowRight is pressed", () => {
    mountTargets()
    const { onNext } = renderTour(1)
    fireEvent.keyDown(document, { key: "ArrowRight" })
    expect(onNext).toHaveBeenCalledOnce()
  })

  it("calls onFinish on ArrowRight on the final step", () => {
    mountTargets()
    const { onFinish, onNext } = renderTour(TOUR_STEPS.length - 1)
    fireEvent.keyDown(document, { key: "ArrowRight" })
    expect(onFinish).toHaveBeenCalledOnce()
    expect(onNext).not.toHaveBeenCalled()
  })

  it("calls onPrev when ArrowLeft is pressed (but not on first step)", () => {
    mountTargets()
    const { onPrev } = renderTour(0)
    fireEvent.keyDown(document, { key: "ArrowLeft" })
    expect(onPrev).not.toHaveBeenCalled()
  })

  it("progress bar grows proportionally with step", () => {
    mountTargets()
    const { unmount } = render(
      <OnboardingTour
        step={0}
        onNext={() => {}}
        onPrev={() => {}}
        onFinish={() => {}}
        onSkip={() => {}}
        onClose={() => {}}
      />
    )
    const first = screen.getByTestId("onboarding-progress").getAttribute("style") ?? ""
    expect(first).toMatch(/width:\s*[\d.]+%/)
    unmount()

    render(
      <OnboardingTour
        step={TOUR_STEPS.length - 1}
        onNext={() => {}}
        onPrev={() => {}}
        onFinish={() => {}}
        onSkip={() => {}}
        onClose={() => {}}
      />
    )
    const last = screen.getByTestId("onboarding-progress").getAttribute("style") ?? ""
    // Final step → 100%.
    expect(last).toMatch(/width:\s*100%/)
  })

  it("highlights the target of an anchored step", async () => {
    mountTargets()
    renderTour(1)
    expect(screen.getByTestId("onboarding-card")).toHaveAttribute("data-step", "addItem")
    expect(await screen.findByTestId("onboarding-highlight")).toBeInTheDocument()
  })

  // #2787 — without a group the sidebar has no Add item button and no group
  // sections. A step whose target is missing is skipped instead of showing
  // a centered card about an element the user cannot see.
  it("skips a step whose target is not in the DOM and counts only the shown steps", () => {
    mountTargets(ALL_TARGETS.filter((k) => k !== "nav-locations"))
    const onFinish = vi.fn()
    render(<StatefulTour onFinish={onFinish} onClose={() => {}} />)

    const shown: string[] = []
    for (let i = 0; i < TOUR_STEPS.length; i += 1) {
      shown.push(screen.getByTestId("onboarding-card").getAttribute("data-step") ?? "")
      expect(screen.getByTestId("onboarding-dots").children).toHaveLength(TOUR_STEPS.length - 1)
      const done = screen.queryByTestId("onboarding-done")
      if (done) {
        fireEvent.click(done)
        break
      }
      fireEvent.click(screen.getByTestId("onboarding-next"))
    }

    expect(shown).toEqual([
      "welcome",
      "addItem",
      "navDashboard",
      "navItems",
      "navWarranties",
      "navFiles",
    ])
    expect(onFinish).toHaveBeenCalledOnce()
  })

  it("Back returns to the previous shown step, skipping the missing one", () => {
    mountTargets(ALL_TARGETS.filter((k) => k !== "nav-locations"))
    render(<StatefulTour onFinish={() => {}} onClose={() => {}} />)
    // welcome → addItem → navDashboard → navItems
    for (let i = 0; i < 3; i += 1) fireEvent.click(screen.getByTestId("onboarding-next"))
    expect(screen.getByTestId("onboarding-card")).toHaveAttribute("data-step", "navItems")
    fireEvent.click(screen.getByTestId("onboarding-prev"))
    expect(screen.getByTestId("onboarding-card")).toHaveAttribute("data-step", "navDashboard")
  })

  it("keeps its steps while hidden, even when the targets go away for a moment", async () => {
    // Shell hides the tour while the window is below the mobile breakpoint,
    // where the sidebar the steps point at is not rendered. When it comes
    // back, it must show the same steps and the same position.
    mountTargets()
    const props = {
      step: 1,
      onNext: vi.fn(),
      onPrev: vi.fn(),
      onFinish: vi.fn(),
      onSkip: vi.fn(),
      onClose: vi.fn(),
    }
    const { rerender } = render(<OnboardingTour {...props} />)
    expect(screen.getByTestId("onboarding-card")).toHaveAttribute("data-step", "addItem")

    rerender(<OnboardingTour {...props} hidden />)
    expect(screen.queryByTestId("onboarding-tour")).not.toBeInTheDocument()
    fireEvent.keyDown(document, { key: "Escape" })
    fireEvent.keyDown(document, { key: "ArrowRight" })
    expect(props.onSkip).not.toHaveBeenCalled()
    expect(props.onNext).not.toHaveBeenCalled()

    document.querySelectorAll("[data-testid='tour-targets']").forEach((el) => el.remove())
    mountTargets()
    rerender(<OnboardingTour {...props} hidden={false} />)
    expect(screen.getByTestId("onboarding-card")).toHaveAttribute("data-step", "addItem")
    expect(screen.getByTestId("onboarding-dots").children).toHaveLength(TOUR_STEPS.length)
    expect(await screen.findByTestId("onboarding-highlight")).toBeInTheDocument()
    expect(props.onClose).not.toHaveBeenCalled()
  })

  it("renders nothing and calls onClose when no step has a target", async () => {
    // The welcome target alone (the sidebar header, which renders for a
    // user without a group too) is not enough: the welcome card followed
    // by Done would announce a tour that is not there.
    mountTargets(["welcome"])
    const { onClose, onSkip, onFinish } = renderTour(0)
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
    expect(screen.queryByTestId("onboarding-tour")).not.toBeInTheDocument()
    // Not a skip or a finish: those mark the tour as seen.
    expect(onSkip).not.toHaveBeenCalled()
    expect(onFinish).not.toHaveBeenCalled()
  })
})

describe("resolveShownSteps", () => {
  it("returns every step when all targets are present", () => {
    mountTargets()
    expect(resolveShownSteps().map((s) => s.key)).toEqual(TOUR_STEPS.map((s) => s.key))
  })

  it("keeps the centered welcome step without its target", () => {
    mountTargets(["add-item"])
    expect(resolveShownSteps().map((s) => s.key)).toEqual(["welcome", "addItem"])
  })

  it("returns nothing when no anchored step has a target", () => {
    mountTargets(["welcome"])
    expect(resolveShownSteps()).toEqual([])
  })
})
