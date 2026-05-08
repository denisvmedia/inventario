import { ShieldCheck, ShieldAlert, ShieldOff, Shield } from "lucide-react"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { MOCK_ITEMS, warrantyStatus, areaName, WARRANTY_STATUS_CONFIG, type WarrantyStatus } from "@/data/mock"

interface WarrantiesViewProps {
  onItemClick: (id: string) => void
}

function formatDate(d: string | null) {
  if (!d) return "—"
  return new Date(d).toLocaleDateString("en-US", { year: "numeric", month: "short", day: "numeric" })
}

function daysUntil(dateStr: string | null) {
  if (!dateStr) return null
  return Math.ceil((new Date(dateStr).getTime() - Date.now()) / (1000 * 60 * 60 * 24))
}

const STATUS_ICONS = {
  active: ShieldCheck,
  expiring: ShieldAlert,
  expired: ShieldOff,
  none: Shield,
}

function WarrantyList({
  status,
  onItemClick,
}: {
  status: WarrantyStatus
  onItemClick: (id: string) => void
}) {
  const items = MOCK_ITEMS.filter((i) => warrantyStatus(i) === status).sort((a, b) => {
    const da = a.warranty.expiresAt ?? ""
    const db = b.warranty.expiresAt ?? ""
    return da.localeCompare(db)
  })

  const config = WARRANTY_STATUS_CONFIG[status]
  const Icon = STATUS_ICONS[status]

  if (items.length === 0) {
    return (
      <div className="py-12 text-center">
        <Icon className="size-8 mx-auto mb-2 text-muted-foreground/30" />
        <p className="text-sm text-muted-foreground">No items in this category.</p>
      </div>
    )
  }

  return (
    <Card className="overflow-hidden p-0 mt-4">
      <ul>
        {items.map((item, i) => {
          const days = daysUntil(item.warranty.expiresAt)
          return (
            <li key={item.id}>
              {i > 0 && <Separator />}
              <button
                className="flex w-full items-center gap-4 px-5 py-4 text-left transition-colors hover:bg-muted/50"
                onClick={() => onItemClick(item.id)}
              >
                <Icon className={`size-5 shrink-0 ${config.color}`} />
                <div className="flex-1 min-w-0">
                  <p className="truncate text-sm font-medium">{item.name}</p>
                  <p className="text-xs text-muted-foreground">{item.brand} · {areaName(item.areaId)}</p>
                </div>
                <div className="text-right shrink-0">
                  {item.warranty.expiresAt ? (
                    <>
                      <p className="text-sm font-medium">{formatDate(item.warranty.expiresAt)}</p>
                      {days !== null && (
                        <p className={`text-xs ${config.color}`}>
                          {days > 0 ? `${days} days left` : `${Math.abs(days)} days ago`}
                        </p>
                      )}
                    </>
                  ) : (
                    <p className="text-xs text-muted-foreground">No date</p>
                  )}
                </div>
              </button>
            </li>
          )
        })}
      </ul>
    </Card>
  )
}

export function WarrantiesView({ onItemClick }: WarrantiesViewProps) {
  const counts = {
    active: MOCK_ITEMS.filter((i) => warrantyStatus(i) === "active").length,
    expiring: MOCK_ITEMS.filter((i) => warrantyStatus(i) === "expiring").length,
    expired: MOCK_ITEMS.filter((i) => warrantyStatus(i) === "expired").length,
    none: MOCK_ITEMS.filter((i) => warrantyStatus(i) === "none").length,
  }

  return (
    <div className="flex flex-col gap-6 p-6 max-w-4xl mx-auto w-full">
      <div>
        <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">Warranties</h1>
        <p className="mt-1 text-muted-foreground">Track coverage across everything you own.</p>
      </div>

      {/* Status summary */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {(["active", "expiring", "expired", "none"] as WarrantyStatus[]).map((status) => {
          const config = WARRANTY_STATUS_CONFIG[status]
          const Icon = STATUS_ICONS[status]
          return (
            <Card key={status} className={`gap-3 border ${config.bg} border-current/10`}>
              <CardHeader className="pb-1">
                <Icon className={`size-5 ${config.color}`} />
              </CardHeader>
              <CardContent>
                <p className={`text-2xl font-bold ${config.color}`}>{counts[status]}</p>
                <p className="text-xs text-muted-foreground mt-0.5">{config.label}</p>
              </CardContent>
            </Card>
          )
        })}
      </div>

      <Tabs defaultValue="expiring">
        <TabsList variant="line" className="w-full justify-start">
          <TabsTrigger value="expiring" className="gap-1.5">
            Expiring
            {counts.expiring > 0 && (
              <Badge variant="outline" className="h-4 px-1 text-[10px] text-status-expiring border-status-expiring/30">
                {counts.expiring}
              </Badge>
            )}
          </TabsTrigger>
          <TabsTrigger value="active" className="gap-1.5">
            Active
            {counts.active > 0 && (
              <Badge variant="outline" className="h-4 px-1 text-[10px]">
                {counts.active}
              </Badge>
            )}
          </TabsTrigger>
          <TabsTrigger value="expired" className="gap-1.5">
            Expired
            {counts.expired > 0 && (
              <Badge variant="outline" className="h-4 px-1 text-[10px] text-status-expired border-status-expired/30">
                {counts.expired}
              </Badge>
            )}
          </TabsTrigger>
          <TabsTrigger value="none">No Warranty</TabsTrigger>
        </TabsList>
        <TabsContent value="expiring">
          <WarrantyList status="expiring" onItemClick={onItemClick} />
        </TabsContent>
        <TabsContent value="active">
          <WarrantyList status="active" onItemClick={onItemClick} />
        </TabsContent>
        <TabsContent value="expired">
          <WarrantyList status="expired" onItemClick={onItemClick} />
        </TabsContent>
        <TabsContent value="none">
          <WarrantyList status="none" onItemClick={onItemClick} />
        </TabsContent>
      </Tabs>
    </div>
  )
}
