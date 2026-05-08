import {
  Camera,
  Package,
  ShieldCheck,
  TrendingUp,
  Calendar,
  Pencil,
  MapPin,
  Globe,
  Zap,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { MOCK_ITEMS, warrantyStatus } from "@/data/mock"

interface UserProfileViewProps {
  onItemClick?: (id: string) => void
  onEditProfile?: () => void
  onUpgrade?: () => void
}

export function UserProfileView({ onItemClick, onEditProfile, onUpgrade }: UserProfileViewProps) {
  const displayName = "Alex Johnson"
  const bio = "Home owner, gadget collector. Tracking everything I own so I never lose a warranty again."
  const location = "San Francisco, CA"
  const website = "alex.example.com"

  const activeWarranties = MOCK_ITEMS.filter((i) => warrantyStatus(i) === "active").length
  const expiringWarranties = MOCK_ITEMS.filter((i) => warrantyStatus(i) === "expiring").length
  const totalValue = MOCK_ITEMS.reduce((s, i) => s + (i.currentValue ?? 0), 0)
  const recentItems = [...MOCK_ITEMS]
    .sort((a, b) => (b.purchasedAt ?? "").localeCompare(a.purchasedAt ?? ""))
    .slice(0, 4)

  const STATS = [
    { label: "Items", value: MOCK_ITEMS.length, icon: Package, color: "text-foreground" },
    { label: "Active warranties", value: activeWarranties, icon: ShieldCheck, color: "text-status-active" },
    { label: "Expiring soon", value: expiringWarranties, icon: ShieldCheck, color: "text-status-expiring" },
    { label: "Est. value", value: `$${(totalValue / 1000).toFixed(1)}k`, icon: TrendingUp, color: "text-foreground" },
  ]

  const CATEGORY_ICONS: Record<string, string> = {
    appliance: "🏠", electronics: "💻", tool: "🔧",
    furniture: "🪑", vehicle: "🚗", other: "📦",
  }

  return (
    <div className="flex flex-col gap-8 p-6 max-w-3xl mx-auto w-full">
      {/* Cover + avatar + identity */}
      <div className="rounded-2xl border border-border overflow-hidden">
        {/* Cover banner */}
        <div className="relative h-28 bg-primary overflow-hidden">
          {/* Subtle diagonal stripe texture */}
          <div
            className="absolute inset-0 opacity-[0.07]"
            style={{
              backgroundImage:
                "repeating-linear-gradient(-45deg, currentColor 0, currentColor 1px, transparent 1px, transparent 10px)",
            }}
          />
          {/* Radial glow */}
          <div className="absolute -bottom-6 -left-6 size-36 rounded-full bg-primary-foreground/10 blur-2xl" />
          <div className="absolute -top-4 right-12 size-24 rounded-full bg-primary-foreground/5 blur-xl" />

          {/* Edit profile button — top-right of banner */}
          <Button
            variant="secondary"
            size="sm"
            className="absolute top-3 right-3 gap-1.5 bg-background/80 hover:bg-background/95 backdrop-blur-sm text-foreground shadow-sm"
            onClick={onEditProfile}
          >
            <Pencil className="size-3.5" />
            Edit profile
          </Button>
        </div>

        {/* Avatar + name row */}
        <div className="px-5 pb-5">
          {/* Avatar overlapping the banner */}
          <div className="flex items-end justify-between -mt-9 mb-3">
            <div className="relative group">
              <div className="flex size-[72px] items-center justify-center rounded-2xl bg-card border-4 border-background text-xl font-bold text-primary shadow-md">
                AJ
              </div>
              <button className="absolute inset-0 flex items-center justify-center rounded-[14px] bg-black/40 opacity-0 group-hover:opacity-100 transition-opacity">
                <Camera className="size-4 text-white" />
              </button>
            </div>
              <div className="flex items-center gap-2 mb-1">
                <Badge variant="outline" className="text-xs font-medium">Free plan</Badge>
                <Button variant="outline" size="sm" className="gap-1.5 h-7 px-2.5 text-xs" onClick={onUpgrade}>
                  <Zap className="size-3" />
                  Upgrade
                </Button>
              </div>
          </div>

          {/* Name + email */}
          <div className="space-y-0.5 mb-3">
            <h1 className="text-xl font-bold tracking-tight">{displayName}</h1>
            <p className="text-sm text-muted-foreground">alex@example.com</p>
          </div>

          {/* Bio */}
          <p className="text-sm text-muted-foreground leading-relaxed mb-4">{bio}</p>

          {/* Meta row */}
          <div className="flex flex-wrap gap-x-4 gap-y-1.5">
            {[
              { icon: MapPin, value: location },
              { icon: Calendar, value: "Joined January 2024" },
              ...(website ? [{ icon: Globe, value: website }] : []),
            ].map(({ icon: Icon, value }) => (
              <div key={value} className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <Icon className="size-3.5 shrink-0" />
                <span>{value}</span>
              </div>
            ))}
          </div>
        </div>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-4 gap-3">
        {STATS.map((s) => (
          <Card key={s.label} className="gap-2 py-4">
            <CardHeader className="pb-0 px-4">
              <s.icon className={`size-4 ${s.color}`} />
            </CardHeader>
            <CardContent className="px-4">
              <p className={`text-xl font-bold ${s.color}`}>{s.value}</p>
              <p className="text-xs text-muted-foreground mt-0.5">{s.label}</p>
            </CardContent>
          </Card>
        ))}
      </div>

      {/* Tabs */}
      <Tabs defaultValue="inventory">
        <TabsList variant="line" className="justify-start">
          <TabsTrigger value="inventory">Inventory</TabsTrigger>
          <TabsTrigger value="activity">Activity</TabsTrigger>
        </TabsList>

        <TabsContent value="inventory" className="mt-4">
          <div className="grid gap-3 sm:grid-cols-2">
            {recentItems.map((item) => (
              <button
                key={item.id}
                className="flex items-center gap-3 rounded-xl border border-border bg-card p-3 text-left transition-all hover:shadow-sm hover:-translate-y-0.5"
                onClick={() => onItemClick?.(item.id)}
              >
                <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-lg">
                  {CATEGORY_ICONS[item.category]}
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium truncate">{item.name}</p>
                  <p className="text-xs text-muted-foreground">{item.brand}</p>
                </div>
                <p className="text-sm font-medium shrink-0 text-muted-foreground">
                  {item.currentValue ? `$${item.currentValue.toLocaleString()}` : "—"}
                </p>
              </button>
            ))}
          </div>
          {MOCK_ITEMS.length > 4 && (
            <p className="mt-3 text-center text-xs text-muted-foreground">
              +{MOCK_ITEMS.length - 4} more items in inventory
            </p>
          )}
        </TabsContent>

        <TabsContent value="activity" className="mt-4">
          <div className="space-y-0 rounded-xl border border-border overflow-hidden bg-card">
            {[
              { text: "Added Bosch Dishwasher", time: "3 months ago", type: "add" },
              { text: "Updated Washing Machine warranty", time: "4 months ago", type: "edit" },
              { text: "Added 4K TV to Living Room", time: "5 months ago", type: "add" },
              { text: "Marked Sony WH-1000XM5 warranty as expired", time: "7 months ago", type: "warn" },
              { text: "Added Dyson V15 Detect", time: "1 year ago", type: "add" },
            ].map((ev, i) => (
              <div key={i}>
                {i > 0 && <Separator />}
                <div className="flex items-start gap-3 px-4 py-3">
                  <div className={`mt-0.5 size-2 rounded-full shrink-0 ${
                    ev.type === "add" ? "bg-status-active" :
                    ev.type === "warn" ? "bg-status-expiring" : "bg-muted-foreground"
                  }`} />
                  <div className="flex-1">
                    <p className="text-sm">{ev.text}</p>
                    <p className="text-xs text-muted-foreground mt-0.5">{ev.time}</p>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </TabsContent>
      </Tabs>
    </div>
  )
}
