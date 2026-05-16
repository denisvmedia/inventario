import { describe, expect, it, vi } from "vitest"
import { render, screen, fireEvent } from "@testing-library/react"

import { OnboardingTour, TOUR_STEPS } from "@/components/OnboardingTour"

function renderTour(stepIndex: number, handlers: Partial<Record<string, () => void>> = {}) {
  const onNext = vi.fn()
  const onPrev = vi.fn()
  const onFinish = vi.fn()
  const onSkip = vi.fn()
  render(
    <OnboardingTour
      step={stepIndex}
      totalSteps={TOUR_STEPS.length}
      onNext={handlers.onNext ?? onNext}
      onPrev={handlers.onPrev ?? onPrev}
      onFinish={handlers.onFinish ?? onFinish}
      onSkip={handlers.onSkip ?? onSkip}
    />
  )
  return { onNext, onPrev, onFinish, onSkip }
}

describe("OnboardingTour", () => {
  it("renders the welcome step with title, description, skip + next CTAs", () => {
    renderTour(0)
    expect(screen.getByTestId("onboarding-tour")).toBeInTheDocument()
    expect(screen.getByTestId("onboarding-title")).toHaveTextContent(/welcome/i)
    expect(screen.getByTestId("onboarding-skip")).toBeInTheDocument()
    expect(screen.getByTestId("onboarding-next")).toBeInTheDocument()
    expect(screen.queryByTestId("onboarding-prev")).not.toBeInTheDocument()
    expect(screen.queryByTestId("onboarding-done")).not.toBeInTheDocument()
  })

  it("renders the final step with Back + Done (no Next)", () => {
    renderTour(TOUR_STEPS.length - 1)
    expect(screen.getByTestId("onboarding-prev")).toBeInTheDocument()
    expect(screen.getByTestId("onboarding-done")).toBeInTheDocument()
    expect(screen.queryByTestId("onboarding-next")).not.toBeInTheDocument()
    expect(screen.queryByTestId("onboarding-skip")).not.toBeInTheDocument()
  })

  it("calls onNext when Next is clicked", () => {
    const { onNext } = renderTour(1)
    fireEvent.click(screen.getByTestId("onboarding-next"))
    expect(onNext).toHaveBeenCalledOnce()
  })

  it("calls onPrev when Back is clicked", () => {
    const { onPrev } = renderTour(2)
    fireEvent.click(screen.getByTestId("onboarding-prev"))
    expect(onPrev).toHaveBeenCalledOnce()
  })

  it("calls onFinish when Done is clicked on the final step", () => {
    const { onFinish } = renderTour(TOUR_STEPS.length - 1)
    fireEvent.click(screen.getByTestId("onboarding-done"))
    expect(onFinish).toHaveBeenCalledOnce()
  })

  it("calls onSkip when Escape is pressed", () => {
    const { onSkip } = renderTour(0)
    fireEvent.keyDown(document, { key: "Escape" })
    expect(onSkip).toHaveBeenCalledOnce()
  })

  it("calls onNext when ArrowRight is pressed", () => {
    const { onNext } = renderTour(1)
    fireEvent.keyDown(document, { key: "ArrowRight" })
    expect(onNext).toHaveBeenCalledOnce()
  })

  it("calls onFinish on ArrowRight on the final step", () => {
    const { onFinish, onNext } = renderTour(TOUR_STEPS.length - 1)
    fireEvent.keyDown(document, { key: "ArrowRight" })
    expect(onFinish).toHaveBeenCalledOnce()
    expect(onNext).not.toHaveBeenCalled()
  })

  it("calls onPrev when ArrowLeft is pressed (but not on first step)", () => {
    const { onPrev } = renderTour(0)
    fireEvent.keyDown(document, { key: "ArrowLeft" })
    expect(onPrev).not.toHaveBeenCalled()
  })

  it("progress bar grows proportionally with step", () => {
    const { unmount } = render(
      <OnboardingTour
        step={0}
        totalSteps={TOUR_STEPS.length}
        onNext={() => {}}
        onPrev={() => {}}
        onFinish={() => {}}
        onSkip={() => {}}
      />
    )
    const first = screen.getByTestId("onboarding-progress").getAttribute("style") ?? ""
    expect(first).toMatch(/width:\s*[\d.]+%/)
    unmount()

    render(
      <OnboardingTour
        step={TOUR_STEPS.length - 1}
        totalSteps={TOUR_STEPS.length}
        onNext={() => {}}
        onPrev={() => {}}
        onFinish={() => {}}
        onSkip={() => {}}
      />
    )
    const last = screen.getByTestId("onboarding-progress").getAttribute("style") ?? ""
    // Final step → 100%.
    expect(last).toMatch(/width:\s*100%/)
  })
})
