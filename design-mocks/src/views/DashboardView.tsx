import { Package, ShieldCheck, ShieldAlert, ShieldOff, TrendingUp, Plus, Sparkles } from "lucide-react"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { WarrantyBadge } from "@/components/WarrantyBadge"
import { MOCK_ITEMS, warrantyStatus, areaName, WARRANTY_STATUS_CONFIG } from "@/data/mock"
import { useIsMobile } from "@/hooks/use-mobile"

function formatCurrency(n: number | null) {
  if (n === null) return "—"
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD", maximumFractionDigits: 0 }).format(n)
}

function daysUntil(dateStr: string | null) {
  if (!dateStr) return null
  const diff = Math.ceil((new Date(dateStr).getTime() - Date.now()) / (1000 * 60 * 60 * 24))
  return diff
}

export function DashboardView({ onItemClick, onAddItem }: { onItemClick: (id: string) => void; onAddItem: () => void }) {
  const isMobile = useIsMobile()
  const statusCounts = {
    active: 0,
    expiring: 0,
    expired: 0,
    none: 0,
  }
  let totalValue = 0

  for (const item of MOCK_ITEMS) {
    const s = warrantyStatus(item)
    statusCounts[s]++
    if (item.currentValue) totalValue += item.currentValue
  }

  const expiringItems = MOCK_ITEMS.filter((i) => warrantyStatus(i) === "expiring")
    .sort((a, b) => {
      const da = daysUntil(a.warranty.expiresAt) ?? 9999
      const db = daysUntil(b.warranty.expiresAt) ?? 9999
      return da - db
    })

  const recentItems = [...MOCK_ITEMS]
    .sort((a, b) => (b.purchasedAt ?? "").localeCompare(a.purchasedAt ?? ""))
    .slice(0, 4)

  const STAT_CARDS = [
    {
      label: "Total Items",
      value: MOCK_ITEMS.length,
      icon: Package,
      sub: "across all locations",
      color: "text-foreground",
    },
    {
      label: "Active Warranties",
      value: statusCounts.active,
      icon: ShieldCheck,
      sub: `${statusCounts.expiring} expiring soon`,
      color: "text-status-active",
    },
    {
      label: "Expired Warranties",
      value: statusCounts.expired,
      icon: ShieldOff,
      sub: "consider renewal",
      color: "text-status-expired",
    },
    {
      label: "Est. Total Value",
      value: formatCurrency(totalValue),
      icon: TrendingUp,
      sub: "current resale value",
      color: "text-foreground",
    },
  ]

  return (
    <div className="flex flex-col gap-8 p-6 max-w-5xl mx-auto w-full">
      {/* Header */}
      <div>
        <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">Overview</h1>
        <p className="mt-1 text-muted-foreground leading-7">
          Everything you own, at a glance.
        </p>
      </div>

      {/* Mobile add-item CTA — only visible on small screens */}
      {isMobile && (
        <button
          onClick={onAddItem}
          className="group flex w-full items-center gap-4 rounded-2xl border border-border bg-card px-5 py-4 text-left transition-all active:scale-[0.98] hover:border-primary/30 hover:bg-muted/40 hover:shadow-sm md:hidden"
        >
          <div className="flex size-12 shrink-0 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-sm transition-transform group-active:scale-95">
            <Plus className="size-5" />
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-base font-semibold leading-tight">Add new item</p>
            <p className="mt-0.5 text-sm text-muted-foreground">Track a device, appliance, tool…</p>
          </div>
          <Sparkles className="size-4 text-muted-foreground/50 shrink-0" />
        </button>
      )}

      {/* Stat cards */}
      <div className="grid grid-cols-2 gap-4 lg:grid-cols-4">
        {STAT_CARDS.map((stat) => (
          <Card key={stat.label} className="gap-3">
            <CardHeader className="pb-2">
              <div className="flex items-center justify-between">
                <CardDescription className="text-xs font-medium uppercase tracking-wide">
                  {stat.label}
                </CardDescription>
                <stat.icon className={`size-4 ${stat.color}`} />
              </div>
            </CardHeader>
            <CardContent>
              <p className={`text-2xl font-bold tracking-tight ${stat.color}`}>
                {stat.value}
              </p>
              <p className="mt-0.5 text-xs text-muted-foreground">{stat.sub}</p>
            </CardContent>
          </Card>
        ))}
      </div>

      <div className="grid gap-6 lg:grid-cols-2">
        {/* Expiring warranties */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <ShieldAlert className="size-4 text-status-expiring" />
              Expiring Warranties
            </CardTitle>
            <CardDescription>Warranties expiring within 60 days</CardDescription>
          </CardHeader>
          <CardContent className="p-0">
            {expiringItems.length === 0 ? (
              <p className="px-6 pb-6 text-sm text-muted-foreground">
                All clear — no warranties expiring soon.
              </p>
            ) : (
              <ul>
                {expiringItems.map((item, i) => {
                  const days = daysUntil(item.warranty.expiresAt)
                  return (
                    <li key={item.id}>
                      {i > 0 && <Separator />}
                      <button
                        className="flex w-full items-center justify-between px-6 py-3.5 text-left transition-colors hover:bg-muted/50"
                        onClick={() => onItemClick(item.id)}
                      >
                        <div>
                          <p className="text-sm font-medium">{item.name}</p>
                          <p className="text-xs text-muted-foreground">{item.brand} · {areaName(item.areaId)}</p>
                        </div>
                        <Badge
                          variant="outline"
                          className="text-status-expiring bg-status-expiring/10 border-current/20 shrink-0 ml-4"
                        >
                          {days} days left
                        </Badge>
                      </button>
                    </li>
                  )
                })}
              </ul>
            )}
          </CardContent>
        </Card>

        {/* Recently added */}
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2 text-base">
              <Package className="size-4" />
              Recently Added
            </CardTitle>
            <CardDescription>Your latest inventory entries</CardDescription>
          </CardHeader>
          <CardContent className="p-0">
            <ul>
              {recentItems.map((item, i) => (
                <li key={item.id}>
                  {i > 0 && <Separator />}
                  <button
                    className="flex w-full items-center justify-between px-6 py-3.5 text-left transition-colors hover:bg-muted/50"
                    onClick={() => onItemClick(item.id)}
                  >
                    <div className="flex items-center gap-3 min-w-0">
                      <div className="flex size-8 shrink-0 items-center justify-center rounded-md bg-muted">
                        <Package className="size-4 text-muted-foreground" />
                      </div>
                      <div className="min-w-0">
                        <p className="truncate text-sm font-medium">{item.name}</p>
                        <p className="text-xs text-muted-foreground">{item.brand}</p>
                      </div>
                    </div>
                    <div className="ml-4 shrink-0">
                      <WarrantyBadge item={item} />
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          </CardContent>
        </Card>
      </div>

      {/* Warranty breakdown */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base">Warranty Health</CardTitle>
          <CardDescription>Status distribution across all items</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="space-y-3">
            {(["active", "expiring", "expired", "none"] as const).map((status) => {
              const config = WARRANTY_STATUS_CONFIG[status]
              const count = statusCounts[status]
              const pct = Math.round((count / MOCK_ITEMS.length) * 100)
              return (
                <div key={status} className="flex items-center gap-3">
                  <span className={`w-24 text-xs font-medium ${config.color}`}>
                    {config.label}
                  </span>
                  <div className="flex-1 h-2 rounded-full bg-muted overflow-hidden">
                    <div
                      className={`h-full rounded-full transition-all ${config.bg} border border-current/20`}
                      style={{ width: `${pct}%`, backgroundColor: `var(--status-${status})` }}
                    />
                  </div>
                  <span className="w-8 text-right text-xs text-muted-foreground">{count}</span>
                </div>
              )
            })}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
