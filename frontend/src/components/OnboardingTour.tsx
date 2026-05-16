import { useCallback, useEffect, useRef, useState, type CSSProperties } from "react"
import { createPortal } from "react-dom"
import { useTranslation } from "react-i18next"
import {
  ArrowLeft,
  ArrowRight,
  FolderOpen,
  LayoutDashboard,
  MapPin,
  Package,
  Plus,
  ShieldCheck,
  Sparkles,
  X,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"

// OnboardingTour — 7-step guided product tour ported from
// `design-mocks/src/components/OnboardingTour.tsx`. Step copy reads from
// the `common:onboarding.steps.*` i18n keys so en/cs/ru can diverge; the
// fallback chain still resolves to en when a locale leaves them blank.
//
// Step targets are `[data-tour='<key>']` selectors. The Welcome step uses
// `placement: "center"`, so the target lookup is allowed to miss without
// breaking the geometry — the overlay falls back to a full-screen dim.
// Every other step needs the matching attribute on a visible element in
// `AppSidebar` (or wherever else).
//
// State (open/skipped/current step / per-user persistence) lives in
// `useOnboardingTour` — this component is a pure renderer driven by the
// step index + callbacks.
// #1543 / design-audit #1527.

type StepKey =
  | "welcome"
  | "addItem"
  | "navDashboard"
  | "navLocations"
  | "navItems"
  | "navWarranties"
  | "navFiles"

export interface TourStep {
  key: StepKey
  // CSS selector for the highlighted target. `[data-tour='welcome']` doesn't
  // need to exist — the welcome step uses placement: "center".
  target: string
  icon: typeof Sparkles
  placement: "top" | "bottom" | "left" | "right" | "center"
  highlightPadding?: number
}

export const TOUR_STEPS: ReadonlyArray<TourStep> = [
  { key: "welcome", target: "[data-tour='welcome']", icon: Sparkles, placement: "center" },
  {
    key: "addItem",
    target: "[data-tour='add-item']",
    icon: Plus,
    placement: "bottom",
    highlightPadding: 6,
  },
  {
    key: "navDashboard",
    target: "[data-tour='nav-dashboard']",
    icon: LayoutDashboard,
    placement: "right",
    highlightPadding: 4,
  },
  {
    key: "navLocations",
    target: "[data-tour='nav-locations']",
    icon: MapPin,
    placement: "right",
    highlightPadding: 4,
  },
  {
    key: "navItems",
    target: "[data-tour='nav-items']",
    icon: Package,
    placement: "right",
    highlightPadding: 4,
  },
  {
    key: "navWarranties",
    target: "[data-tour='nav-warranties']",
    icon: ShieldCheck,
    placement: "right",
    highlightPadding: 4,
  },
  {
    key: "navFiles",
    target: "[data-tour='nav-files']",
    icon: FolderOpen,
    placement: "right",
    highlightPadding: 4,
  },
]

interface SpotRect {
  top: number
  left: number
  width: number
  height: number
}

function measureTarget(selector: string, padding = 8): SpotRect | null {
  if (typeof document === "undefined") return null
  const el = document.querySelector(selector)
  if (!el) return null
  const r = el.getBoundingClientRect()
  return {
    top: r.top - padding,
    left: r.left - padding,
    width: r.width + padding * 2,
    height: r.height + padding * 2,
  }
}

function tooltipPosition(
  spot: SpotRect | null,
  placement: TourStep["placement"],
  tooltipW = 320,
  tooltipH = 220
): CSSProperties {
  if (typeof window === "undefined") return { position: "fixed" }
  const vw = window.innerWidth
  const vh = window.innerHeight
  const gap = 14
  const margin = 12

  if (!spot || placement === "center") {
    return {
      position: "fixed",
      top: vh / 2 - tooltipH / 2,
      left: vw / 2 - tooltipW / 2,
    }
  }

  let top: number
  let left: number

  if (placement === "right") {
    top = spot.top + spot.height / 2 - tooltipH / 2
    left = spot.left + spot.width + gap
    if (left + tooltipW > vw - margin) left = spot.left - tooltipW - gap
  } else if (placement === "left") {
    top = spot.top + spot.height / 2 - tooltipH / 2
    left = spot.left - tooltipW - gap
    if (left < margin) left = spot.left + spot.width + gap
  } else if (placement === "bottom") {
    top = spot.top + spot.height + gap
    left = spot.left + spot.width / 2 - tooltipW / 2
    if (top + tooltipH > vh - margin) top = spot.top - tooltipH - gap
  } else {
    top = spot.top - tooltipH - gap
    left = spot.left + spot.width / 2 - tooltipW / 2
    if (top < margin) top = spot.top + spot.height + gap
  }

  top = Math.max(margin, Math.min(vh - tooltipH - margin, top))
  left = Math.max(margin, Math.min(vw - tooltipW - margin, left))

  return { position: "fixed", top, left }
}

interface OverlayProps {
  spot: SpotRect | null
  isCenter: boolean
  onSkip: () => void
}

function Overlay({ spot, isCenter, onSkip }: OverlayProps) {
  if (typeof window === "undefined") return null
  const vw = window.innerWidth
  const vh = window.innerHeight

  // The overlay panes are *clickable* (clicking outside the spotlight
  // dismisses the tour) but they're visual chrome rather than primary
  // controls — keyboard skip is wired in the parent via Escape. Render
  // them as <button> so jsx-a11y/click-events-have-key-events passes
  // without re-binding the global Escape handler on every pane.
  const skipLabel = "Skip tour overlay"

  if (!spot || isCenter) {
    return (
      <button
        type="button"
        className="fixed inset-0 cursor-pointer bg-black/55"
        onClick={onSkip}
        aria-label={skipLabel}
        data-testid="onboarding-overlay-full"
      />
    )
  }

  const top = Math.max(0, spot.top)
  const left = Math.max(0, spot.left)
  const right = Math.min(vw, spot.left + spot.width)
  const bottom = Math.min(vh, spot.top + spot.height)

  return (
    <>
      <button
        type="button"
        className="fixed cursor-pointer bg-black/55"
        style={{ top: 0, left: 0, right: 0, height: top }}
        onClick={onSkip}
        aria-label={skipLabel}
      />
      <button
        type="button"
        className="fixed cursor-pointer bg-black/55"
        style={{ top: bottom, left: 0, right: 0, bottom: 0 }}
        onClick={onSkip}
        aria-label={skipLabel}
      />
      <button
        type="button"
        className="fixed cursor-pointer bg-black/55"
        style={{ top, left: 0, width: left, height: bottom - top }}
        onClick={onSkip}
        aria-label={skipLabel}
      />
      <button
        type="button"
        className="fixed cursor-pointer bg-black/55"
        style={{ top, left: right, right: 0, height: bottom - top }}
        onClick={onSkip}
        aria-label={skipLabel}
      />
    </>
  )
}

interface OnboardingTourProps {
  step: number
  totalSteps: number
  onNext: () => void
  onPrev: () => void
  onFinish: () => void
  onSkip: () => void
}

export function OnboardingTour({
  step,
  totalSteps,
  onNext,
  onPrev,
  onFinish,
  onSkip,
}: OnboardingTourProps) {
  const { t } = useTranslation()
  const currentStep = TOUR_STEPS[step]
  const [spot, setSpot] = useState<SpotRect | null>(null)
  const [mounted, setMounted] = useState(false)
  const measureRef = useRef<() => void>(() => {})

  const measure = useCallback(() => {
    if (!currentStep) return
    const r = measureTarget(currentStep.target, currentStep.highlightPadding ?? 8)
    setSpot(r)
  }, [currentStep])

  // Mirror `measure` into a ref inside an effect (not during render) so
  // the resize/scroll listeners below can call the latest closure
  // without re-binding on every step change. Direct assignment in the
  // render body trips `react-hooks/refs-in-render`.
  useEffect(() => {
    measureRef.current = measure
  }, [measure])

  useEffect(() => {
    const t = window.setTimeout(() => setMounted(true), 80)
    return () => window.clearTimeout(t)
  }, [])

  useEffect(() => {
    if (!currentStep) return
    const el = document.querySelector(currentStep.target)
    if (el && "scrollIntoView" in el) {
      ;(el as Element).scrollIntoView({ behavior: "smooth", block: "nearest" })
    }
    const t = window.setTimeout(() => measure(), 50)
    const onResize = () => measureRef.current()
    const onScroll = () => measureRef.current()
    window.addEventListener("resize", onResize)
    window.addEventListener("scroll", onScroll, true)
    return () => {
      window.clearTimeout(t)
      window.removeEventListener("resize", onResize)
      window.removeEventListener("scroll", onScroll, true)
    }
  }, [measure, currentStep])

  // Keyboard: Esc skips, ArrowLeft/Right navigates, Enter advances /
  // finishes. Bound at the document level so users don't need to click
  // the card first.
  useEffect(() => {
    function handleKey(e: KeyboardEvent) {
      if (e.key === "Escape") {
        e.preventDefault()
        onSkip()
      } else if (e.key === "ArrowRight" || e.key === "Enter") {
        e.preventDefault()
        if (step === totalSteps - 1) onFinish()
        else onNext()
      } else if (e.key === "ArrowLeft") {
        e.preventDefault()
        if (step > 0) onPrev()
      }
    }
    document.addEventListener("keydown", handleKey)
    return () => document.removeEventListener("keydown", handleKey)
  }, [step, totalSteps, onNext, onPrev, onSkip, onFinish])

  if (!currentStep || typeof document === "undefined") return null

  const isCenter = currentStep.placement === "center"
  const isLast = step === totalSteps - 1
  const isFirst = step === 0
  const Icon = currentStep.icon

  const cardStyle = tooltipPosition(spot, currentStep.placement)
  const title = t(`common:onboarding.steps.${currentStep.key}.title`, {
    brand: t("common:brand"),
  })
  const description = t(`common:onboarding.steps.${currentStep.key}.description`)

  return createPortal(
    <div
      className="fixed inset-0 z-[9999]"
      aria-modal="true"
      role="dialog"
      aria-label={t("common:onboarding.ariaLabel")}
      data-testid="onboarding-tour"
      style={{
        opacity: mounted ? 1 : 0,
        transition: "opacity 200ms ease",
        pointerEvents: mounted ? undefined : "none",
      }}
    >
      <Overlay spot={spot} isCenter={isCenter} onSkip={onSkip} />

      {spot && !isCenter ? (
        <div
          className="pointer-events-none fixed rounded-[10px]"
          style={{
            top: spot.top,
            left: spot.left,
            width: spot.width,
            height: spot.height,
            boxShadow:
              "0 0 0 2px var(--primary), 0 0 0 5px color-mix(in oklch, var(--primary) 30%, transparent), 0 8px 40px rgba(0,0,0,0.4)",
            transition:
              "top 220ms cubic-bezier(0.4,0,0.2,1), left 220ms cubic-bezier(0.4,0,0.2,1), width 220ms cubic-bezier(0.4,0,0.2,1), height 220ms cubic-bezier(0.4,0,0.2,1)",
          }}
          aria-hidden="true"
          data-testid="onboarding-highlight"
        />
      ) : null}

      {/* The card itself isn't clickable — clicks on its background are
          a no-op via stopPropagation so they don't bubble to the
          overlay (which would skip the tour). The interactive controls
          inside (Back / Next / Done / X) handle their own keyboard
          activation, and Escape/Arrow keys are wired on document. */}
      <div
        className="pointer-events-auto w-[320px] rounded-2xl border border-border bg-card shadow-2xl"
        data-testid="onboarding-card"
        role="presentation"
        style={{
          ...cardStyle,
          transition:
            "top 220ms cubic-bezier(0.4,0,0.2,1), left 220ms cubic-bezier(0.4,0,0.2,1), transform 200ms ease, opacity 200ms ease",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="h-1 overflow-hidden rounded-t-2xl bg-muted">
          <div
            className="h-full bg-primary transition-all duration-500 ease-out"
            data-testid="onboarding-progress"
            style={{ width: `${((step + 1) / totalSteps) * 100}%` }}
          />
        </div>

        <div className="flex flex-col gap-4 p-5">
          <div className="flex items-start gap-3">
            <div className="mt-0.5 flex size-9 shrink-0 items-center justify-center rounded-xl bg-primary/10">
              <Icon className="size-4 text-primary" aria-hidden="true" />
            </div>
            <div className="min-w-0 flex-1">
              <div className="mb-1 flex items-start justify-between gap-2">
                <p className="text-sm font-semibold leading-snug" data-testid="onboarding-title">
                  {title}
                </p>
                <button
                  type="button"
                  onClick={onSkip}
                  className="mt-0.5 shrink-0 rounded p-0.5 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                  aria-label={t("common:onboarding.skipLabel")}
                  data-testid="onboarding-skip-icon"
                >
                  <X className="size-3.5" aria-hidden="true" />
                </button>
              </div>
              <p
                className="text-xs leading-relaxed text-muted-foreground"
                data-testid="onboarding-description"
              >
                {description}
              </p>
            </div>
          </div>

          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1" aria-hidden="true">
              {Array.from({ length: totalSteps }, (_, i) => (
                <div
                  key={i}
                  className={cn(
                    "rounded-full transition-all duration-300",
                    i === step
                      ? "h-1.5 w-4 bg-primary"
                      : i < step
                        ? "h-1.5 w-1.5 bg-primary/40"
                        : "h-1.5 w-1.5 bg-border"
                  )}
                />
              ))}
            </div>

            <div className="flex items-center gap-1.5">
              {!isFirst ? (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 gap-1 px-2 text-xs"
                  onClick={onPrev}
                  data-testid="onboarding-prev"
                >
                  <ArrowLeft className="size-3" aria-hidden="true" />
                  {t("common:onboarding.back")}
                </Button>
              ) : null}
              {isFirst ? (
                <Button
                  variant="ghost"
                  size="sm"
                  className="h-7 px-2 text-xs text-muted-foreground"
                  onClick={onSkip}
                  data-testid="onboarding-skip"
                >
                  {t("common:onboarding.skip")}
                </Button>
              ) : null}
              <Button
                size="sm"
                className="h-7 gap-1.5 px-3 text-xs"
                onClick={isLast ? onFinish : onNext}
                data-testid={isLast ? "onboarding-done" : "onboarding-next"}
              >
                {isLast ? t("common:onboarding.done") : t("common:onboarding.next")}
                {!isLast ? <ArrowRight className="size-3" aria-hidden="true" /> : null}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>,
    document.body
  )
}

interface RestartTourButtonProps {
  onRestart: () => void
  className?: string
}

export function RestartTourButton({ onRestart, className }: RestartTourButtonProps) {
  const { t } = useTranslation()
  return (
    <button
      type="button"
      onClick={onRestart}
      className={cn(
        "flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs text-muted-foreground transition-colors hover:bg-muted hover:text-foreground",
        className
      )}
      title={t("common:onboarding.restartTitle")}
      data-testid="onboarding-restart"
    >
      <Sparkles className="size-3.5" aria-hidden="true" />
      {t("common:onboarding.restart")}
    </button>
  )
}
