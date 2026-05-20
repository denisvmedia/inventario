import { useState, useMemo, useEffect } from "react"
import {
  Package,
  LayoutGrid,
  List,
  Search,
  ListFilter as Filter,
  ChevronDown,
  ShieldCheck,
  TrendingUp,
  Plus,
  ArrowUpDown,
  Eye,
  EyeOff,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle, CardDescription } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuCheckboxItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
} from "@/components/ui/dropdown-menu"
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationPrevious,
  PaginationNext,
  PaginationEllipsis,
} from "@/components/ui/pagination"
import { WarrantyBadge } from "@/components/WarrantyBadge"
import {
  CATEGORIES,
  warrantyStatus,
  areaName,
  COMMODITY_STATUS_CONFIG,
  resolveTags,
  type InventoryItem,
  type ItemCategory,
  type WarrantyStatus,
  type CommodityStatus,
} from "@/data/mock"
import { TagPill } from "@/components/TagPill"
import { cn } from "@/lib/utils"

const PAGE_SIZE = 8

type SortKey = "name" | "purchasedAt" | "price" | "value"

const SORT_OPTIONS: { value: SortKey; label: string }[] = [
  { value: "name", label: "Name A–Z" },
  { value: "purchasedAt", label: "Purchase Date" },
  { value: "price", label: "Purchase Price" },
  { value: "value", label: "Current Value" },
]

const CATEGORY_ICONS: Record<ItemCategory | string, string> = {
  appliance: "🏠",
  electronics: "💻",
  tool: "🔧",
  furniture: "🪑",
  vehicle: "🚗",
  other: "📦",
}

function formatCurrency(n: number | null) {
  if (n === null) return "—"
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 0,
  }).format(n)
}

interface ItemsPanelProps {
  items: InventoryItem[]
  onItemClick: (id: string) => void
  onAddItem?: () => void
  /** Title shown above stats. If omitted, no heading section is rendered. */
  title?: string
  subtitle?: string
  showStats?: boolean
  defaultViewMode?: "grid" | "list"
}

export function ItemsPanel({
  items,
  onItemClick,
  onAddItem,
  title,
  subtitle,
  showStats = true,
  defaultViewMode = "grid",
}: ItemsPanelProps) {
  const [query, setQuery] = useState("")
  const [viewMode, setViewMode] = useState<"grid" | "list">(defaultViewMode)
  const [activeCategories, setActiveCategories] = useState<Set<ItemCategory>>(new Set())
  const [activeWarrantyStatuses, setActiveWarrantyStatuses] = useState<Set<WarrantyStatus>>(new Set())
  const [activeCommodityStatuses, setActiveCommodityStatuses] = useState<Set<CommodityStatus>>(new Set())
  const [showInactive, setShowInactive] = useState(false)
  const [sortKey, setSortKey] = useState<SortKey>("name")
  const [currentPage, setCurrentPage] = useState(1)

  const filtered = useMemo(() => {
    let result = items.filter((item) => {
      // Hide draft + non-in_use unless showInactive
      if (!showInactive && (item.draft || item.status !== "in_use")) return false

      if (
        query &&
        !item.name.toLowerCase().includes(query.toLowerCase()) &&
        !item.brand.toLowerCase().includes(query.toLowerCase()) &&
        !areaName(item.areaId).toLowerCase().includes(query.toLowerCase())
      )
        return false
      if (activeCategories.size > 0 && !activeCategories.has(item.category)) return false
      if (activeWarrantyStatuses.size > 0 && !activeWarrantyStatuses.has(warrantyStatus(item))) return false
      if (activeCommodityStatuses.size > 0 && !activeCommodityStatuses.has(item.status)) return false
      return true
    })

    result = [...result].sort((a, b) => {
      if (sortKey === "name") return a.name.localeCompare(b.name)
      if (sortKey === "purchasedAt") {
        return (b.purchasedAt ?? "").localeCompare(a.purchasedAt ?? "")
      }
      if (sortKey === "price") return (b.purchasePrice ?? 0) - (a.purchasePrice ?? 0)
      if (sortKey === "value") return (b.currentValue ?? 0) - (a.currentValue ?? 0)
      return 0
    })

    return result
  }, [items, query, activeCategories, activeWarrantyStatuses, activeCommodityStatuses, showInactive, sortKey])

  useEffect(() => {
    setCurrentPage(1)
  }, [query, activeCategories, activeWarrantyStatuses, activeCommodityStatuses, showInactive, sortKey])

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const paginated = filtered.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE)

  function toggleCategory(cat: ItemCategory) {
    setActiveCategories((prev) => {
      const next = new Set(prev)
      next.has(cat) ? next.delete(cat) : next.add(cat)
      return next
    })
  }

  function toggleWarrantyStatus(s: WarrantyStatus) {
    setActiveWarrantyStatuses((prev) => {
      const next = new Set(prev)
      next.has(s) ? next.delete(s) : next.add(s)
      return next
    })
  }

  function toggleCommodityStatus(s: CommodityStatus) {
    setActiveCommodityStatuses((prev) => {
      const next = new Set(prev)
      next.has(s) ? next.delete(s) : next.add(s)
      return next
    })
  }

  const hasFilters = activeCategories.size > 0 || activeWarrantyStatuses.size > 0 || activeCommodityStatuses.size > 0

  const activeWarranties = items.filter((i) => warrantyStatus(i) === "active").length
  const totalValue = items.reduce((s, i) => s + (i.currentValue ?? 0), 0)

  const inactiveCount = items.filter((i) => i.draft || i.status !== "in_use").length

  return (
    <div className="flex flex-col gap-4">
      {/* Optional header */}
      {(title || onAddItem) && (
        <div className="flex items-start justify-between gap-4">
          {title && (
            <div>
              <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">{title}</h1>
              {subtitle && <p className="mt-1 text-muted-foreground">{subtitle}</p>}
            </div>
          )}
          {onAddItem && (
            <Button onClick={onAddItem} size="sm" className={cn(!title && "ml-auto")}>
              <Plus className="size-4 mr-1.5" />
              Add Item
            </Button>
          )}
        </div>
      )}

      {/* Stats strip */}
      {showStats && (
        <div className="grid grid-cols-3 gap-3">
          {[
            { label: "Items", value: items.length, icon: Package, color: "text-foreground" as const },
            { label: "Active warranties", value: activeWarranties, icon: ShieldCheck, color: "text-status-active" as const },
            { label: "Est. value", value: formatCurrency(totalValue), icon: TrendingUp, color: "text-foreground" as const },
          ].map((s) => (
            <div key={s.label} className="flex items-center gap-3 rounded-xl border border-border bg-card px-4 py-3">
              <s.icon className={cn("size-4 shrink-0", s.color)} />
              <div>
                <p className={cn("text-sm font-semibold", s.color)}>{s.value}</p>
                <p className="text-xs text-muted-foreground">{s.label}</p>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Toolbar */}
      <div className="flex items-center gap-2 flex-wrap">
        <div className="relative flex-1 min-w-48">
          <Search className="absolute left-2.5 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search items, brands…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            className="pl-8"
          />
        </div>

        {/* Category filter */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant={activeCategories.size > 0 ? "default" : "outline"}
              size="sm"
              className="gap-1.5"
            >
              <Filter className="size-3.5" />
              Category
              {activeCategories.size > 0 && (
                <Badge variant="secondary" className="ml-0.5 h-4 px-1 text-xs">
                  {activeCategories.size}
                </Badge>
              )}
              <ChevronDown className="size-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-44">
            <DropdownMenuLabel>Category</DropdownMenuLabel>
            <DropdownMenuSeparator />
            {CATEGORIES.map((cat) => (
              <DropdownMenuCheckboxItem
                key={cat.value}
                checked={activeCategories.has(cat.value)}
                onCheckedChange={() => toggleCategory(cat.value)}
              >
                {CATEGORY_ICONS[cat.value]} {cat.label}
              </DropdownMenuCheckboxItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>

        {/* Status filter */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant={activeCommodityStatuses.size > 0 ? "default" : "outline"}
              size="sm"
              className="gap-1.5"
            >
              <Filter className="size-3.5" />
              Status
              {activeCommodityStatuses.size > 0 && (
                <Badge variant="secondary" className="ml-0.5 h-4 px-1 text-xs">
                  {activeCommodityStatuses.size}
                </Badge>
              )}
              <ChevronDown className="size-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-48">
            <DropdownMenuLabel>Commodity Status</DropdownMenuLabel>
            <DropdownMenuSeparator />
            {(Object.keys(COMMODITY_STATUS_CONFIG) as CommodityStatus[]).map((s) => (
              <DropdownMenuCheckboxItem
                key={s}
                checked={activeCommodityStatuses.has(s)}
                onCheckedChange={() => toggleCommodityStatus(s)}
              >
                {COMMODITY_STATUS_CONFIG[s].label}
              </DropdownMenuCheckboxItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>

        {/* Warranty filter */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant={activeWarrantyStatuses.size > 0 ? "default" : "outline"}
              size="sm"
              className="gap-1.5"
            >
              <Filter className="size-3.5" />
              Warranty
              {activeWarrantyStatuses.size > 0 && (
                <Badge variant="secondary" className="ml-0.5 h-4 px-1 text-xs">
                  {activeWarrantyStatuses.size}
                </Badge>
              )}
              <ChevronDown className="size-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-44">
            <DropdownMenuLabel>Warranty Status</DropdownMenuLabel>
            <DropdownMenuSeparator />
            {(["active", "expiring", "expired", "none"] as WarrantyStatus[]).map((s) => (
              <DropdownMenuCheckboxItem
                key={s}
                checked={activeWarrantyStatuses.has(s)}
                onCheckedChange={() => toggleWarrantyStatus(s)}
              >
                {s === "active" && "✓ Active"}
                {s === "expiring" && "⚠ Expiring"}
                {s === "expired" && "✗ Expired"}
                {s === "none" && "— None"}
              </DropdownMenuCheckboxItem>
            ))}
          </DropdownMenuContent>
        </DropdownMenu>

        {/* Sort */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="outline" size="sm" className="gap-1.5">
              <ArrowUpDown className="size-3.5" />
              {SORT_OPTIONS.find((o) => o.value === sortKey)?.label}
              <ChevronDown className="size-3.5" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-44">
            <DropdownMenuLabel>Sort by</DropdownMenuLabel>
            <DropdownMenuSeparator />
            <DropdownMenuRadioGroup value={sortKey} onValueChange={(v) => setSortKey(v as SortKey)}>
              {SORT_OPTIONS.map((o) => (
                <DropdownMenuRadioItem key={o.value} value={o.value}>
                  {o.label}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>

        {hasFilters && (
          <Button
            variant="ghost"
            size="sm"
            onClick={() => {
              setActiveCategories(new Set())
              setActiveWarrantyStatuses(new Set())
              setActiveCommodityStatuses(new Set())
            }}
          >
            Clear filters
          </Button>
        )}

        {/* Show inactive toggle */}
        {inactiveCount > 0 && (
          <Button
            variant={showInactive ? "secondary" : "ghost"}
            size="sm"
            className="gap-1.5"
            onClick={() => setShowInactive((v) => !v)}
          >
            {showInactive ? <Eye className="size-3.5" /> : <EyeOff className="size-3.5" />}
            {showInactive ? "Showing inactive" : `${inactiveCount} hidden`}
          </Button>
        )}

        <div className="ml-auto flex gap-1">
          <Button
            variant={viewMode === "grid" ? "secondary" : "ghost"}
            size="icon"
            className="size-8"
            onClick={() => setViewMode("grid")}
          >
            <LayoutGrid className="size-4" />
          </Button>
          <Button
            variant={viewMode === "list" ? "secondary" : "ghost"}
            size="icon"
            className="size-8"
            onClick={() => setViewMode("list")}
          >
            <List className="size-4" />
          </Button>
        </div>
      </div>

      {/* Items */}
      {filtered.length === 0 ? (
        <div className="flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border py-20">
          <Package className="size-10 text-muted-foreground/40" />
          <p className="text-sm text-muted-foreground">No items match your filters.</p>
          <Button
            variant="outline"
            size="sm"
            onClick={() => {
              setQuery("")
              setActiveCategories(new Set())
              setActiveWarrantyStatuses(new Set())
              setActiveCommodityStatuses(new Set())
            }}
          >
            Clear filters
          </Button>
        </div>
      ) : viewMode === "grid" ? (
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {paginated.map((item) => {
            const statusCfg = COMMODITY_STATUS_CONFIG[item.status]
            return (
              <Card
                key={item.id}
                className={cn(
                  "cursor-pointer gap-3 transition-all hover:shadow-md hover:-translate-y-0.5",
                  item.draft && "opacity-70 border-dashed"
                )}
                onClick={() => onItemClick(item.id)}
              >
                <CardHeader className="pb-2">
                  <div className="flex items-start justify-between gap-2">
                    <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-lg">
                      {CATEGORY_ICONS[item.category]}
                    </div>
                    <div className="flex items-center gap-1.5 flex-wrap justify-end">
                      {item.draft && (
                        <Badge variant="outline" className="text-[10px] h-4 px-1.5 border-dashed text-muted-foreground">
                          Draft
                        </Badge>
                      )}
                      {item.status !== "in_use" && (
                        <span className={cn("text-[10px] font-medium px-1.5 py-0.5 rounded-full border border-current/20", statusCfg.color, statusCfg.bg)}>
                          {statusCfg.label}
                        </span>
                      )}
                      <WarrantyBadge item={item} showIcon={false} />
                    </div>
                  </div>
                  <CardTitle className="mt-2 text-sm font-semibold leading-tight">
                    {item.name}
                  </CardTitle>
                  <CardDescription className="text-xs">
                    {item.brand} · {item.model}
                  </CardDescription>
                </CardHeader>
                <CardContent>
                  <div className="flex items-center justify-between text-xs text-muted-foreground">
                    <span>{areaName(item.areaId)}</span>
                    <span className="font-medium text-foreground">
                      {formatCurrency(item.currentValue)}
                    </span>
                  </div>
                  {item.tags.length > 0 && (
                    <div className="mt-2 flex flex-wrap gap-1">
                      {resolveTags(item.tags).slice(0, 3).map((tag) => (
                        <TagPill key={tag.id} tag={tag} size="xs" />
                      ))}
                    </div>
                  )}
                </CardContent>
              </Card>
            )
          })}
        </div>
      ) : (
        <Card className="overflow-hidden p-0">
          <ul>
            {paginated.map((item, i) => {
              const statusCfg = COMMODITY_STATUS_CONFIG[item.status]
              return (
                <li key={item.id}>
                  {i > 0 && <Separator />}
                  <button
                    className={cn(
                      "flex w-full items-center gap-4 px-5 py-3.5 text-left transition-colors hover:bg-muted/50",
                      item.draft && "opacity-70"
                    )}
                    onClick={() => onItemClick(item.id)}
                  >
                    <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-lg">
                      {CATEGORY_ICONS[item.category]}
                    </div>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-1.5">
                        <p className="truncate text-sm font-medium">{item.name}</p>
                        {item.draft && (
                          <Badge variant="outline" className="text-[10px] h-4 px-1 border-dashed text-muted-foreground shrink-0">
                            Draft
                          </Badge>
                        )}
                      </div>
                      <p className="text-xs text-muted-foreground">
                        {item.brand} · {areaName(item.areaId)}
                      </p>
                    </div>
                    {item.status !== "in_use" ? (
                      <span className={cn("text-xs font-medium px-2 py-0.5 rounded-full border border-current/20 shrink-0", statusCfg.color, statusCfg.bg)}>
                        {statusCfg.label}
                      </span>
                    ) : (
                      <WarrantyBadge item={item} showIcon={false} />
                    )}
                    <p className="hidden text-sm font-medium sm:block w-20 text-right">
                      {formatCurrency(item.currentValue)}
                    </p>
                  </button>
                </li>
              )
            })}
          </ul>
        </Card>
      )}

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex flex-col items-center gap-3 sm:flex-row sm:justify-between">
          <p className="text-sm text-muted-foreground">
            Showing {(currentPage - 1) * PAGE_SIZE + 1}–{Math.min(currentPage * PAGE_SIZE, filtered.length)} of {filtered.length} items
          </p>
          <Pagination className="w-auto mx-0">
            <PaginationContent>
              <PaginationItem>
                <PaginationPrevious
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  aria-disabled={currentPage === 1}
                  className={cn(currentPage === 1 && "pointer-events-none opacity-50")}
                />
              </PaginationItem>
              {Array.from({ length: totalPages }, (_, i) => i + 1).map((page) => {
                const showPage =
                  page === 1 ||
                  page === totalPages ||
                  Math.abs(page - currentPage) <= 1
                const showEllipsisBefore = page === currentPage - 2 && currentPage - 2 > 1
                const showEllipsisAfter = page === currentPage + 2 && currentPage + 2 < totalPages
                if (showEllipsisBefore) {
                  return (
                    <PaginationItem key={`ellipsis-before-${page}`}>
                      <PaginationEllipsis />
                    </PaginationItem>
                  )
                }
                if (showEllipsisAfter) {
                  return (
                    <PaginationItem key={`ellipsis-after-${page}`}>
                      <PaginationEllipsis />
                    </PaginationItem>
                  )
                }
                if (!showPage) return null
                return (
                  <PaginationItem key={page}>
                    <PaginationLink
                      isActive={page === currentPage}
                      onClick={() => setCurrentPage(page)}
                    >
                      {page}
                    </PaginationLink>
                  </PaginationItem>
                )
              })}
              <PaginationItem>
                <PaginationNext
                  onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                  aria-disabled={currentPage === totalPages}
                  className={cn(currentPage === totalPages && "pointer-events-none opacity-50")}
                />
              </PaginationItem>
            </PaginationContent>
          </Pagination>
        </div>
      )}
    </div>
  )
}
