import { Check, Zap, Package, Crown } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"

interface PlansViewProps {
  onBack?: () => void
}

const PLANS = [
  {
    id: "free",
    name: "Free",
    icon: Package,
    price: 0,
    priceLabel: "$0",
    period: "forever",
    description: "For individuals getting started with home inventory.",
    current: false,
    features: [
      "Up to 50 items",
      "1 location group",
      "3 locations per group",
      "500 MB file storage",
      "Warranty tracking",
      "Basic reports",
    ],
    unavailable: [
      "Insurance export",
      "Multiple groups",
      "Priority support",
    ],
  },
  {
    id: "pro",
    name: "Pro",
    icon: Zap,
    price: 5,
    priceLabel: "$5",
    period: "per month",
    description: "For households that want the full picture.",
    current: true,
    highlight: true,
    features: [
      "Up to 500 items",
      "3 location groups",
      "20 locations per group",
      "10 GB file storage",
      "Warranty tracking",
      "Insurance export",
      "Multiple groups",
      "Email digests & alerts",
    ],
    unavailable: [
      "Priority support",
    ],
  },
  {
    id: "family",
    name: "Family",
    icon: Crown,
    price: 12,
    priceLabel: "$12",
    period: "per month",
    description: "Shared inventory for the whole household.",
    current: false,
    features: [
      "Unlimited items",
      "Unlimited groups",
      "Unlimited locations",
      "50 GB file storage",
      "Warranty tracking",
      "Insurance export",
      "Multiple groups",
      "Email digests & alerts",
      "Priority support",
      "Up to 6 members per group",
    ],
    unavailable: [],
  },
]

export function PlansView({ onBack }: PlansViewProps) {
  return (
    <div className="flex flex-col gap-6 p-6 max-w-4xl mx-auto w-full">
      <div>
        <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">Plans & Pricing</h1>
        <p className="mt-1 text-muted-foreground">Choose the plan that fits your household.</p>
      </div>

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        {PLANS.map((plan) => {
          const Icon = plan.icon
          return (
            <div
              key={plan.id}
              className={cn(
                "relative flex flex-col rounded-xl border bg-card p-5 gap-5",
                plan.highlight
                  ? "border-accent shadow-sm ring-1 ring-accent/30"
                  : "border-border",
              )}
            >
              {plan.highlight && (
                <div className="absolute -top-3 left-1/2 -translate-x-1/2">
                  <Badge className="bg-accent text-accent-foreground border-accent/30 text-xs px-2.5 shadow-sm">
                    Current plan
                  </Badge>
                </div>
              )}

              {/* Header */}
              <div className="space-y-3">
                <div className="flex items-center gap-2.5">
                  <div className={cn(
                    "flex size-8 items-center justify-center rounded-lg shrink-0",
                    plan.highlight ? "bg-accent/20" : "bg-muted",
                  )}>
                    <Icon className={cn("size-4", plan.highlight ? "text-accent-foreground" : "text-muted-foreground")} />
                  </div>
                  <span className="font-semibold text-sm">{plan.name}</span>
                  {plan.current && !plan.highlight && (
                    <Badge variant="secondary" className="text-xs ml-auto">Active</Badge>
                  )}
                </div>

                <div className="flex items-baseline gap-1">
                  <span className="text-3xl font-bold tracking-tight">{plan.priceLabel}</span>
                  <span className="text-sm text-muted-foreground">{plan.period}</span>
                </div>

                <p className="text-sm text-muted-foreground leading-relaxed">{plan.description}</p>
              </div>

              {/* CTA */}
              {plan.current ? (
                <Button variant="outline" size="sm" disabled className="w-full">
                  Current plan
                </Button>
              ) : (
                <Button
                  size="sm"
                  variant={plan.id === "family" ? "default" : "outline"}
                  className="w-full"
                >
                  {plan.price === 0 ? "Downgrade" : "Upgrade"}
                </Button>
              )}

              {/* Features */}
              <div className="space-y-2">
                <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">Includes</p>
                <ul className="space-y-1.5">
                  {plan.features.map((f) => (
                    <li key={f} className="flex items-start gap-2 text-sm">
                      <Check className="size-3.5 shrink-0 mt-0.5 text-status-active" />
                      <span>{f}</span>
                    </li>
                  ))}
                  {plan.unavailable.map((f) => (
                    <li key={f} className="flex items-start gap-2 text-sm text-muted-foreground/50 line-through">
                      <span className="size-3.5 shrink-0 mt-0.5 flex items-center justify-center text-muted-foreground/30 select-none">—</span>
                      <span>{f}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          )
        })}
      </div>

      {onBack && (
        <div>
          <Button variant="ghost" size="sm" onClick={onBack} className="text-muted-foreground">
            ← Back to settings
          </Button>
        </div>
      )}
    </div>
  )
}
