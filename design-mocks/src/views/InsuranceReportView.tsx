import { useState, useEffect } from "react"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Printer, ArrowLeft, Shield, MapPin, Calendar, DollarSign, Hash, Tag, Image as ImageIcon, Building2, CircleCheck as CheckCircle2, CircleAlert as AlertCircle, Clock, Package, LayoutGrid, TextAlignJustify as AlignJustify } from "lucide-react"
import {
  MOCK_ITEMS,
  MOCK_FILES,
  MOCK_LOCATIONS,
  MOCK_AREAS,
  MOCK_GROUPS,
  warrantyStatus,
  WARRANTY_STATUS_CONFIG,
  areaLabel,
  type InventoryItem,
} from "@/data/mock"
import { cn } from "@/lib/utils"

// ── Helpers ──────────────────────────────────────────────────────────────────

const DEMO_PHOTOS: Record<string, string> = {
  "1": "https://images.unsplash.com/photo-1626806787461-102c1bfaaea1?w=800&q=80",
  "2": "https://images.unsplash.com/photo-1517336714731-489689fd1ca8?w=800&q=80",
  "3": "https://images.unsplash.com/photo-1571175443880-49e1d25b2bc5?w=800&q=80",
  "4": "https://images.unsplash.com/photo-1558618666-fcd25c85cd64?w=800&q=80",
  "5": "https://images.unsplash.com/photo-1505740420928-5e560c06d30e?w=800&q=80",
  "6": "https://images.unsplash.com/photo-1585771724684-38269d6639fd?w=800&q=80",
  "7": "https://images.unsplash.com/photo-1593784991095-a205069470b6?w=800&q=80",
  "8": "https://images.unsplash.com/photo-1504148455328-c376907d081c?w=800&q=80",
  "9": "https://images.unsplash.com/photo-1607400201889-565b1ee75f8e?w=800&q=80",
}

function formatDate(iso: string | null) {
  if (!iso) return "—"
  return new Date(iso).toLocaleDateString("en-US", { year: "numeric", month: "long", day: "numeric" })
}

function formatCurrency(n: number | null) {
  if (n == null) return "—"
  return new Intl.NumberFormat("en-US", { style: "currency", currency: "USD" }).format(n)
}

function itemPhotos(itemId: string) {
  const attached = MOCK_FILES.filter(
    (f) => f.attachedTo.type === "commodity" && f.attachedTo.id === itemId && f.mimeType.startsWith("image/")
  )
  if (attached.length > 0) return attached.map((f) => ({ url: f.thumbnailUrl ?? DEMO_PHOTOS[itemId] ?? "", name: f.name }))
  if (DEMO_PHOTOS[itemId]) return [{ url: DEMO_PHOTOS[itemId], name: `${itemId}_photo.jpg` }]
  return []
}

// ── Shared photo section ──────────────────────────────────────────────────────

function PhotoSection({ photos, imageSize }: { photos: { url: string; name: string }[]; imageSize: "thumb" | "full" }) {
  if (photos.length === 0) return null
  return (
    <div>
      <h3 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground mb-3 flex items-center gap-2">
        <ImageIcon className="size-3.5" />
        Photographs ({photos.length})
      </h3>
      {imageSize === "thumb" ? (
        <div className="grid grid-cols-3 gap-2">
          {photos.map((p, i) => (
            <div key={i} className="overflow-hidden rounded-lg border border-border bg-muted aspect-square">
              <img src={p.url} alt={p.name} className="w-full h-full object-cover" />
            </div>
          ))}
        </div>
      ) : (
        <div className="space-y-3">
          {photos.map((p, i) => (
            <div key={i} className="overflow-hidden rounded-xl border border-border bg-muted">
              <img src={p.url} alt={p.name} className="w-full object-contain max-h-[480px]" />
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ── Single-item report ────────────────────────────────────────────────────────

function ItemReport({ item, imageSize }: { item: InventoryItem; imageSize: "thumb" | "full" }) {
  const wStatus = warrantyStatus(item)
  const wConfig = WARRANTY_STATUS_CONFIG[wStatus]
  const photos = itemPhotos(item.id)

  return (
    <div className="space-y-8">
      {/* Header */}
      <div className="bg-primary text-primary-foreground px-10 py-8 -mx-10 -mt-8 print:-mx-0">
        <div className="flex items-start justify-between gap-6">
          <div>
            <div className="flex items-center gap-2 mb-3">
              <Package className="size-5 opacity-70" />
              <span className="text-sm font-medium uppercase tracking-widest opacity-70">Item Insurance Report</span>
            </div>
            <h1 className="text-3xl font-bold tracking-tight leading-tight">{item.brand} {item.name}</h1>
            <p className="mt-1 text-sm opacity-70">{item.model}</p>
          </div>
          <div className="text-right shrink-0">
            <div className="text-xs uppercase tracking-widest opacity-60 mb-1">Generated</div>
            <div className="text-sm font-medium">
              {new Date().toLocaleDateString("en-US", { year: "numeric", month: "long", day: "numeric" })}
            </div>
          </div>
        </div>
      </div>

      {/* Value cards */}
      <div className="grid grid-cols-2 gap-4">
        <div className="rounded-xl border border-border bg-muted/30 p-5">
          <div className="flex items-center gap-2 mb-2">
            <DollarSign className="size-4 text-muted-foreground" />
            <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Purchase Price</span>
          </div>
          <p className="text-2xl font-bold tabular-nums">{formatCurrency(item.purchasePrice)}</p>
          {item.purchasedAt && <p className="text-xs text-muted-foreground mt-1">{formatDate(item.purchasedAt)}</p>}
        </div>
        <div className="rounded-xl border border-border bg-muted/30 p-5">
          <div className="flex items-center gap-2 mb-2">
            <DollarSign className="size-4 text-muted-foreground" />
            <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Estimated Value</span>
          </div>
          <p className="text-2xl font-bold tabular-nums">{formatCurrency(item.currentValue)}</p>
          <p className="text-xs text-muted-foreground mt-1">Current replacement estimate</p>
        </div>
      </div>

      {/* Details */}
      <div>
        <h2 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground mb-4">Item Details</h2>
        <div className="grid grid-cols-2 gap-x-8 gap-y-4">
          {[
            { icon: Tag, label: "Category", value: item.category.charAt(0).toUpperCase() + item.category.slice(1) },
            { icon: Hash, label: "Serial Number", value: item.serialNumber || "—" },
            { icon: Calendar, label: "Purchase Date", value: formatDate(item.purchasedAt) },
            { icon: MapPin, label: "Location", value: areaLabel(item.areaId) },
          ].map(({ icon: Icon, label, value }) => (
            <div key={label} className="flex items-start gap-3">
              <div className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-muted mt-0.5">
                <Icon className="size-3.5 text-muted-foreground" />
              </div>
              <div>
                <p className="text-xs text-muted-foreground">{label}</p>
                <p className="text-sm font-medium mt-0.5">{value}</p>
              </div>
            </div>
          ))}
        </div>
      </div>

      <Separator />

      {/* Warranty */}
      <div>
        <h2 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground mb-4">Warranty Status</h2>
        <div className={cn("flex items-start gap-3 rounded-xl border p-4", wConfig.bg, "border-border")}>
          <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-background border border-border">
            {wStatus === "active"   && <CheckCircle2 className="size-4 text-status-active" />}
            {wStatus === "expiring" && <Clock         className="size-4 text-status-expiring" />}
            {wStatus === "expired"  && <AlertCircle   className="size-4 text-status-expired" />}
            {wStatus === "none"     && <Shield        className="size-4 text-muted-foreground" />}
          </div>
          <div className="flex-1">
            <div className="flex items-center gap-2">
              <span className={cn("text-sm font-semibold", wConfig.color)}>{wConfig.label}</span>
              {item.warranty.expiresAt && (
                <span className="text-xs text-muted-foreground">— expires {formatDate(item.warranty.expiresAt)}</span>
              )}
            </div>
            {item.warranty.notes && <p className="text-sm text-muted-foreground mt-1">{item.warranty.notes}</p>}
          </div>
        </div>
      </div>

      {/* Photos */}
      {photos.length > 0 && (
        <>
          <Separator />
          <PhotoSection photos={photos} imageSize={imageSize} />
        </>
      )}

      {/* Notes */}
      {item.notes && (
        <>
          <Separator />
          <div>
            <h2 className="text-xs font-semibold uppercase tracking-widest text-muted-foreground mb-3">Notes</h2>
            <p className="text-sm text-foreground leading-relaxed bg-muted/30 rounded-xl px-4 py-3 border border-border">{item.notes}</p>
          </div>
        </>
      )}
    </div>
  )
}

// ── Location report ───────────────────────────────────────────────────────────

function LocationReport({
  locationId,
  imageSize,
}: {
  locationId: string
  imageSize: "thumb" | "full"
}) {
  // Resolve location and areas
  const location = MOCK_LOCATIONS.find((l) => l.id === locationId)
  const group = location ? MOCK_GROUPS.find((g) => g.id === location.groupId) : undefined
  const areas = MOCK_AREAS.filter((a) => a.locationId === locationId)
  const areaIds = new Set(areas.map((a) => a.id))
  const items = MOCK_ITEMS.filter((i) => areaIds.has(i.areaId))

  const totalPurchase = items.reduce((s, i) => s + (i.purchasePrice ?? 0), 0)
  const totalEstimated = items.reduce((s, i) => s + (i.currentValue ?? 0), 0)

  const generatedAt = new Date().toLocaleDateString("en-US", {
    year: "numeric", month: "long", day: "numeric",
  })

  return (
    <div className="space-y-10">
      {/* Header */}
      <div className="bg-primary text-primary-foreground px-10 py-8 -mx-10 -mt-8 print:-mx-0">
        <div className="flex items-start justify-between gap-6">
          <div>
            <div className="flex items-center gap-2 mb-3">
              <Building2 className="size-5 opacity-70" />
              <span className="text-sm font-medium uppercase tracking-widest opacity-70">
                Location Insurance Report
              </span>
            </div>
            <h1 className="text-3xl font-bold tracking-tight leading-tight">
              {location?.icon} {location?.name}
            </h1>
            {group && <p className="mt-1 text-sm opacity-70">{group.name}</p>}
          </div>
          <div className="text-right shrink-0">
            <div className="text-xs uppercase tracking-widest opacity-60 mb-1">Generated</div>
            <div className="text-sm font-medium">{generatedAt}</div>
            <div className="text-xs opacity-60 mt-1">{items.length} items</div>
          </div>
        </div>
      </div>

      {/* Summary */}
      <div className="grid grid-cols-3 gap-4">
        <div className="rounded-xl border border-border bg-muted/30 p-5">
          <div className="flex items-center gap-2 mb-2">
            <Package className="size-4 text-muted-foreground" />
            <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Total Items</span>
          </div>
          <p className="text-2xl font-bold tabular-nums">{items.length}</p>
        </div>
        <div className="rounded-xl border border-border bg-muted/30 p-5">
          <div className="flex items-center gap-2 mb-2">
            <DollarSign className="size-4 text-muted-foreground" />
            <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Total Purchase</span>
          </div>
          <p className="text-2xl font-bold tabular-nums">{formatCurrency(totalPurchase)}</p>
        </div>
        <div className="rounded-xl border border-border bg-muted/30 p-5">
          <div className="flex items-center gap-2 mb-2">
            <DollarSign className="size-4 text-muted-foreground" />
            <span className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">Est. Total Value</span>
          </div>
          <p className="text-2xl font-bold tabular-nums">{formatCurrency(totalEstimated)}</p>
        </div>
      </div>

      {/* Per-item sections */}
      {items.map((item, idx) => {
        const wStatus = warrantyStatus(item)
        const wConfig = WARRANTY_STATUS_CONFIG[wStatus]
        const photos = itemPhotos(item.id)

        return (
          <div key={item.id} className="print:break-inside-avoid">
            {idx > 0 && <Separator className="mb-10" />}

            {/* Item heading */}
            <div className="flex items-start justify-between gap-4 mb-6">
              <div>
                <div className="flex items-center gap-2 mb-1">
                  <span className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
                    {item.category.charAt(0).toUpperCase() + item.category.slice(1)}
                  </span>
                  <span className="text-xs text-muted-foreground">·</span>
                  <span className="text-xs text-muted-foreground">{areaLabel(item.areaId)}</span>
                </div>
                <h2 className="text-xl font-bold tracking-tight">{item.brand} {item.name}</h2>
                <p className="text-sm text-muted-foreground">{item.model}</p>
              </div>
              <div className={cn(
                "flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-semibold shrink-0 border border-border",
                wConfig.bg, wConfig.color
              )}>
                {wStatus === "active"   && <CheckCircle2 className="size-3" />}
                {wStatus === "expiring" && <Clock         className="size-3" />}
                {wStatus === "expired"  && <AlertCircle   className="size-3" />}
                {wStatus === "none"     && <Shield        className="size-3" />}
                {wConfig.label}
              </div>
            </div>

            {/* Financials + details */}
            <div className="grid grid-cols-2 gap-6 mb-6">
              <div className="space-y-3">
                <div className="flex items-start gap-3">
                  <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted mt-0.5">
                    <DollarSign className="size-3 text-muted-foreground" />
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground">Purchase Price</p>
                    <p className="text-sm font-semibold tabular-nums">{formatCurrency(item.purchasePrice)}</p>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted mt-0.5">
                    <DollarSign className="size-3 text-muted-foreground" />
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground">Estimated Value</p>
                    <p className="text-sm font-semibold tabular-nums">{formatCurrency(item.currentValue)}</p>
                  </div>
                </div>
              </div>
              <div className="space-y-3">
                <div className="flex items-start gap-3">
                  <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted mt-0.5">
                    <Hash className="size-3 text-muted-foreground" />
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground">Serial Number</p>
                    <p className="text-sm font-medium font-mono">{item.serialNumber || "—"}</p>
                  </div>
                </div>
                <div className="flex items-start gap-3">
                  <div className="flex size-7 shrink-0 items-center justify-center rounded-md bg-muted mt-0.5">
                    <Calendar className="size-3 text-muted-foreground" />
                  </div>
                  <div>
                    <p className="text-xs text-muted-foreground">Purchase Date</p>
                    <p className="text-sm font-medium">{formatDate(item.purchasedAt)}</p>
                  </div>
                </div>
              </div>
            </div>

            {/* Warranty note */}
            {(item.warranty.expiresAt || item.warranty.notes) && (
              <div className={cn("rounded-lg border px-4 py-3 mb-6 text-sm", wConfig.bg, "border-border")}>
                <span className={cn("font-medium", wConfig.color)}>Warranty: </span>
                {item.warranty.expiresAt && (
                  <span className="text-muted-foreground">expires {formatDate(item.warranty.expiresAt)}. </span>
                )}
                {item.warranty.notes && <span className="text-muted-foreground">{item.warranty.notes}</span>}
              </div>
            )}

            {/* Photos */}
            <PhotoSection photos={photos} imageSize={imageSize} />
          </div>
        )
      })}

      {items.length === 0 && (
        <div className="rounded-xl border border-dashed border-border p-10 text-center">
          <Package className="size-10 text-muted-foreground/30 mx-auto mb-3" />
          <p className="text-sm text-muted-foreground">No items found for this location.</p>
        </div>
      )}
    </div>
  )
}

// ── Main view ─────────────────────────────────────────────────────────────────

export type InsuranceReportMode = "item" | "location"

interface InsuranceReportViewProps {
  mode?: InsuranceReportMode
  initialItemId?: string
  initialLocationId?: string
  onBack?: () => void
}

export function InsuranceReportView({
  mode: initialMode = "item",
  initialItemId,
  initialLocationId,
  onBack,
}: InsuranceReportViewProps) {
  const [mode, setMode] = useState<InsuranceReportMode>(initialMode)
  const [selectedItemId, setSelectedItemId] = useState(initialItemId ?? MOCK_ITEMS[0].id)
  const [selectedLocationId, setSelectedLocationId] = useState(initialLocationId ?? MOCK_LOCATIONS[0].id)
  const [imageSize, setImageSize] = useState<"thumb" | "full">("thumb")

  const item = MOCK_ITEMS.find((i) => i.id === selectedItemId) ?? MOCK_ITEMS[0]

  const generatedAt = new Date().toLocaleDateString("en-US", {
    year: "numeric", month: "long", day: "numeric",
    hour: "2-digit", minute: "2-digit",
  } as Intl.DateTimeFormatOptions)

  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "Escape") onBack?.()
    }
    document.addEventListener("keydown", handleKeyDown)
    return () => document.removeEventListener("keydown", handleKeyDown)
  }, [onBack])

  return (
    <div className="flex flex-col min-h-screen bg-background">
      {/* Toolbar */}
      <div className="print:hidden sticky top-0 z-10 flex h-12 items-center gap-2 border-b border-border bg-background px-4">
        {onBack && (
          <Button variant="ghost" size="icon" className="size-8 shrink-0" onClick={onBack}>
            <ArrowLeft className="size-4" />
          </Button>
        )}
        <Separator orientation="vertical" className="h-4 shrink-0" />

        {/* Report mode toggle */}
        <ToggleGroup
          type="single"
          value={mode}
          onValueChange={(v) => v && setMode(v as InsuranceReportMode)}
          className="h-8"
        >
          <ToggleGroupItem value="item" className="h-7 px-3 text-xs gap-1.5">
            <Package className="size-3.5" />
            Item
          </ToggleGroupItem>
          <ToggleGroupItem value="location" className="h-7 px-3 text-xs gap-1.5">
            <MapPin className="size-3.5" />
            Location
          </ToggleGroupItem>
        </ToggleGroup>

        <Separator orientation="vertical" className="h-4 shrink-0" />

        {/* Selector */}
        {mode === "item" ? (
          <Select value={selectedItemId} onValueChange={setSelectedItemId}>
            <SelectTrigger className="h-8 w-52 text-xs">
              <SelectValue placeholder="Select item…" />
            </SelectTrigger>
            <SelectContent>
              {MOCK_ITEMS.map((i) => (
                <SelectItem key={i.id} value={i.id} className="text-xs">
                  {i.brand} {i.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        ) : (
          <Select value={selectedLocationId} onValueChange={setSelectedLocationId}>
            <SelectTrigger className="h-8 w-52 text-xs">
              <SelectValue placeholder="Select location…" />
            </SelectTrigger>
            <SelectContent>
              {MOCK_LOCATIONS.map((l) => (
                <SelectItem key={l.id} value={l.id} className="text-xs">
                  {l.icon} {l.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}

        {/* Image size toggle */}
        <div className="ml-auto flex items-center gap-2">
          <span className="text-xs text-muted-foreground hidden sm:block">Photos:</span>
          <ToggleGroup
            type="single"
            value={imageSize}
            onValueChange={(v) => v && setImageSize(v as "thumb" | "full")}
            className="h-8"
          >
            <ToggleGroupItem value="thumb" className="h-7 px-2.5 text-xs gap-1.5" title="Thumbnails">
              <LayoutGrid className="size-3.5" />
              <span className="hidden sm:inline">Thumbnails</span>
            </ToggleGroupItem>
            <ToggleGroupItem value="full" className="h-7 px-2.5 text-xs gap-1.5" title="Full size">
              <AlignJustify className="size-3.5" />
              <span className="hidden sm:inline">Full size</span>
            </ToggleGroupItem>
          </ToggleGroup>

          <Separator orientation="vertical" className="h-4 shrink-0" />

          <Button size="sm" className="h-8 gap-1.5 text-xs" onClick={() => window.print()}>
            <Printer className="size-3.5" />
            <span className="hidden sm:inline">Print / Save PDF</span>
            <span className="sm:hidden">Print</span>
          </Button>
        </div>
      </div>

      {/* Report body */}
      <div className="flex-1 overflow-y-auto py-8 px-4 bg-muted/30 print:bg-white print:p-0">
        <div className={cn(
          "mx-auto bg-card border border-border rounded-2xl overflow-hidden shadow-sm",
          "print:shadow-none print:rounded-none print:border-0 print:max-w-none",
          "max-w-3xl w-full"
        )}>
          <div className="px-10 py-8">
            {mode === "item" ? (
              <ItemReport item={item} imageSize={imageSize} />
            ) : (
              <LocationReport locationId={selectedLocationId} imageSize={imageSize} />
            )}

            {/* Footer */}
            <Separator className="mt-10 mb-6" />
            <div className="flex items-center justify-between text-xs text-muted-foreground pb-2">
              <div className="flex items-center gap-2">
                <Building2 className="size-3.5" />
                <span>Home Inventory — Insurance Report</span>
              </div>
              <span>Generated {generatedAt}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}
