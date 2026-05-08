import { useEffect, useRef, useState, useCallback } from "react"
import { createPortal } from "react-dom"
import { Button } from "@/components/ui/button"
import { ArrowRight, ArrowLeft, X, Sparkles, MapPin, Plus, LayoutDashboard, Package, ShieldCheck, FolderOpen } from "lucide-react"
import { cn } from "@/lib/utils"

// ── Step definitions ──────────────────────────────────────────────────────────

export interface TourStep {
  target: string
  title: string
  description: string
  icon: React.ElementType
  placement?: "top" | "bottom" | "left" | "right" | "center"
  highlightPadding?: number
}

export const TOUR_STEPS: TourStep[] = [
  {
    target: "[data-tour='welcome']",
    title: "Welcome to Homelog",
    description: "Your personal home inventory system. Track every item, warranty, file, and receipt — all in one place. Let's take a quick tour.",
    icon: Sparkles,
    placement: "center",
  },
  {
    target: "[data-tour='add-item']",
    title: "Add your first item",
    description: "Tap here to add any item to your inventory — appliances, electronics, furniture, tools. Capture purchase price, serial number, and photos.",
    icon: Plus,
    placement: "bottom",
    highlightPadding: 6,
  },
  {
    target: "[data-tour='nav-dashboard']",
    title: "Dashboard overview",
    description: "Your home base. See a summary of total inventory value, warranty statuses, recent items, and key stats at a glance.",
    icon: LayoutDashboard,
    placement: "right",
    highlightPadding: 4,
  },
  {
    target: "[data-tour='nav-locations']",
    title: "Organize by location",
    description: "Structure your inventory by rooms and zones. Create locations like \"Kitchen\" or \"Garage\", then divide them into areas.",
    icon: MapPin,
    placement: "right",
    highlightPadding: 4,
  },
  {
    target: "[data-tour='nav-items']",
    title: "Browse all items",
    description: "See every item in your inventory. Filter by category, search by name, sort by value — and open any item for full details.",
    icon: Package,
    placement: "right",
    highlightPadding: 4,
  },
  {
    target: "[data-tour='nav-warranties']",
    title: "Track warranties",
    description: "Never miss an expiring warranty again. Homelog tracks every warranty and alerts you before they run out.",
    icon: ShieldCheck,
    placement: "right",
    highlightPadding: 4,
  },
  {
    target: "[data-tour='nav-files']",
    title: "Attach files",
    description: "Upload receipts, manuals, warranty certificates, and photos — all attached directly to items, locations, or areas.",
    icon: FolderOpen,
    placement: "right",
    highlightPadding: 4,
  },
]

// ── Geometry helpers ──────────────────────────────────────────────────────────

interface SpotRect {
  top: number
  left: number
  width: number
  height: number
}

function measureTarget(selector: string, padding = 8): SpotRect | null {
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
  tooltipH = 220,
): React.CSSProperties {
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

  let top: number, left: number

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

// ── Overlay: 4 rects with cutout ─────────────────────────────────────────────

interface OverlayProps {
  spot: SpotRect | null
  isCenter: boolean
  onSkip: () => void
}

function Overlay({ spot, isCenter, onSkip }: OverlayProps) {
  const vw = window.innerWidth
  const vh = window.innerHeight

  if (!spot || isCenter) {
    return (
      <div className="fixed inset-0 bg-black/55 cursor-pointer" onClick={onSkip} />
    )
  }

  const top    = Math.max(0, spot.top)
  const left   = Math.max(0, spot.left)
  const right  = Math.min(vw, spot.left + spot.width)
  const bottom = Math.min(vh, spot.top + spot.height)

  return (
    <>
      <div className="fixed bg-black/55 cursor-pointer" style={{ top: 0, left: 0, right: 0, height: top }} onClick={onSkip} />
      <div className="fixed bg-black/55 cursor-pointer" style={{ top: bottom, left: 0, right: 0, bottom: 0 }} onClick={onSkip} />
      <div className="fixed bg-black/55 cursor-pointer" style={{ top, left: 0, width: left, height: bottom - top }} onClick={onSkip} />
      <div className="fixed bg-black/55 cursor-pointer" style={{ top, left: right, right: 0, height: bottom - top }} onClick={onSkip} />
    </>
  )
}

// ── Main OnboardingTour ───────────────────────────────────────────────────────

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
  const currentStep = TOUR_STEPS[step]
  const [spot, setSpot] = useState<SpotRect | null>(null)
  // mounted tracks first render to fade in the whole tour; never goes back to false
  const [mounted, setMounted] = useState(false)
  const measureRef = useRef<() => void>(() => {})

  const measure = useCallback(() => {
    const r = measureTarget(currentStep.target, currentStep.highlightPadding ?? 8)
    setSpot(r)
  }, [currentStep])

  measureRef.current = measure

  // On first mount: small delay then show
  useEffect(() => {
    const t = setTimeout(() => setMounted(true), 80)
    return () => clearTimeout(t)
  }, [])

  // On step change: re-measure immediately (no opacity reset)
  useEffect(() => {
    // Scroll target into view first
    const el = document.querySelector(currentStep.target)
    if (el) el.scrollIntoView({ behavior: "smooth", block: "nearest" })

    // Measure after a tiny tick so scroll has started
    const t = setTimeout(() => measure(), 50)

    const onResize = () => measureRef.current()
    const onScroll = () => measureRef.current()
    window.addEventListener("resize", onResize)
    window.addEventListener("scroll", onScroll, true)

    return () => {
      clearTimeout(t)
      window.removeEventListener("resize", onResize)
      window.removeEventListener("scroll", onScroll, true)
    }
  }, [measure, currentStep.target])

  const isCenter = currentStep.placement === "center"
  const isLast = step === totalSteps - 1
  const isFirst = step === 0
  const Icon = currentStep.icon

  const cardStyle = tooltipPosition(spot, currentStep.placement)

  return createPortal(
    <div
      className="fixed inset-0 z-[9999]"
      aria-modal="true"
      role="dialog"
      aria-label="Product tour"
      style={{
        opacity: mounted ? 1 : 0,
        transition: "opacity 200ms ease",
        // Pointer events only after mounted to avoid eating clicks during fade-in
        pointerEvents: mounted ? undefined : "none",
      }}
    >
      {/* Dimmed overlay with cutout — transitions its own geometry via CSS */}
      <Overlay spot={spot} isCenter={isCenter} onSkip={onSkip} />

      {/* Highlight ring — transitions position/size smoothly */}
      {spot && !isCenter && (
        <div
          className="fixed pointer-events-none rounded-[10px]"
          style={{
            top: spot.top,
            left: spot.left,
            width: spot.width,
            height: spot.height,
            boxShadow: "0 0 0 2px var(--primary), 0 0 0 5px color-mix(in oklch, var(--primary) 30%, transparent), 0 8px 40px rgba(0,0,0,0.4)",
            transition: "top 220ms cubic-bezier(0.4,0,0.2,1), left 220ms cubic-bezier(0.4,0,0.2,1), width 220ms cubic-bezier(0.4,0,0.2,1), height 220ms cubic-bezier(0.4,0,0.2,1)",
          }}
        />
      )}

      {/* Tooltip card — slides position, content cross-fades */}
      <div
        className="w-[320px] rounded-2xl border border-border bg-card shadow-2xl pointer-events-auto"
        style={{
          ...cardStyle,
          transition: "top 220ms cubic-bezier(0.4,0,0.2,1), left 220ms cubic-bezier(0.4,0,0.2,1), transform 200ms ease, opacity 200ms ease",
        }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Progress bar */}
        <div className="h-1 rounded-t-2xl overflow-hidden bg-muted">
          <div
            className="h-full bg-primary transition-all duration-500 ease-out"
            style={{ width: `${((step + 1) / totalSteps) * 100}%` }}
          />
        </div>

        <div className="p-5 flex flex-col gap-4">
          {/* Step content */}
          <div className="flex items-start gap-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 mt-0.5">
              <Icon className="size-4 text-primary" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="flex items-start justify-between gap-2 mb-1">
                <p className="text-sm font-semibold leading-snug">{currentStep.title}</p>
                <button
                  onClick={onSkip}
                  className="shrink-0 mt-0.5 rounded p-0.5 text-muted-foreground hover:text-foreground hover:bg-muted transition-colors"
                  aria-label="Skip tour"
                >
                  <X className="size-3.5" />
                </button>
              </div>
              <p className="text-xs text-muted-foreground leading-relaxed">
                {currentStep.description}
              </p>
            </div>
          </div>

          {/* Footer */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1">
              {Array.from({ length: totalSteps }, (_, i) => (
                <div
                  key={i}
                  className={cn(
                    "rounded-full transition-all duration-300",
                    i === step
                      ? "w-4 h-1.5 bg-primary"
                      : i < step
                      ? "w-1.5 h-1.5 bg-primary/40"
                      : "w-1.5 h-1.5 bg-border"
                  )}
                />
              ))}
            </div>

            <div className="flex items-center gap-1.5">
              {!isFirst && (
                <Button variant="ghost" size="sm" className="h-7 px-2 gap-1 text-xs" onClick={onPrev}>
                  <ArrowLeft className="size-3" />
                  Back
                </Button>
              )}
              {isFirst && (
                <Button variant="ghost" size="sm" className="h-7 px-2 text-xs text-muted-foreground" onClick={onSkip}>
                  Skip
                </Button>
              )}
              <Button size="sm" className="h-7 px-3 gap-1.5 text-xs" onClick={isLast ? onFinish : onNext}>
                {isLast ? "Done" : "Next"}
                {!isLast && <ArrowRight className="size-3" />}
              </Button>
            </div>
          </div>
        </div>
      </div>
    </div>,
    document.body
  )
}

// ── Restart tour button ───────────────────────────────────────────────────────

export function RestartTourButton({ onRestart, className }: { onRestart: () => void; className?: string }) {
  return (
    <button
      onClick={onRestart}
      className={cn(
        "flex items-center gap-1.5 rounded-md px-2 py-0.5 text-xs text-muted-foreground hover:text-foreground hover:bg-muted transition-colors",
        className
      )}
      title="Restart product tour"
    >
      <Sparkles className="size-3.5" />
      Tour
    </button>
  )
}
