import { Skeleton } from "@/components/ui/skeleton"
import { Separator } from "@/components/ui/separator"

// ─── Stat card skeleton ───────────────────────────────────────────────────────
// Used in: Dashboard stat row, Items header stats
export function StatCardSkeleton() {
  return (
    <div className="rounded-xl border border-border bg-card px-4 py-3 flex items-center gap-3">
      <Skeleton className="size-8 rounded-lg shrink-0" />
      <div className="space-y-1.5 flex-1">
        <Skeleton className="h-3 w-16 rounded" />
        <Skeleton className="h-5 w-10 rounded" />
      </div>
    </div>
  )
}

export function StatCardSkeletonRow({ count = 3 }: { count?: number }) {
  return (
    <div className="grid grid-cols-3 gap-3">
      {Array.from({ length: count }).map((_, i) => (
        <StatCardSkeleton key={i} />
      ))}
    </div>
  )
}

// ─── Table row skeleton ───────────────────────────────────────────────────────
// Used in: Items list, Warranties list, Members list
export function TableRowSkeleton() {
  return (
    <div className="flex items-center gap-3 px-4 py-3 border-b border-border last:border-0">
      <Skeleton className="size-9 rounded-lg shrink-0" />
      <div className="flex-1 space-y-1.5">
        <Skeleton className="h-3.5 w-40 rounded" />
        <Skeleton className="h-3 w-24 rounded" />
      </div>
      <Skeleton className="h-5 w-16 rounded-full shrink-0" />
      <Skeleton className="h-5 w-14 rounded-full shrink-0" />
      <Skeleton className="size-7 rounded-md shrink-0" />
    </div>
  )
}

export function TableSkeleton({ rows = 6 }: { rows?: number }) {
  return (
    <div className="rounded-xl border border-border bg-card overflow-hidden">
      {/* fake header */}
      <div className="flex items-center gap-3 px-4 py-2.5 bg-muted/50 border-b border-border">
        <Skeleton className="h-3 w-6 rounded" />
        <Skeleton className="h-3 w-24 rounded" />
        <div className="flex-1" />
        <Skeleton className="h-3 w-16 rounded" />
        <Skeleton className="h-3 w-14 rounded" />
        <Skeleton className="size-5 rounded-md" />
      </div>
      {Array.from({ length: rows }).map((_, i) => (
        <TableRowSkeleton key={i} />
      ))}
    </div>
  )
}

// ─── Tile / card skeleton ─────────────────────────────────────────────────────
// Used in: Items grid view, Files grid view
export function TileSkeleton() {
  return (
    <div className="rounded-xl border border-border bg-card overflow-hidden">
      {/* image area */}
      <Skeleton className="w-full aspect-[4/3] rounded-none" />
      <div className="p-3 space-y-2">
        <Skeleton className="h-4 w-3/4 rounded" />
        <Skeleton className="h-3 w-1/2 rounded" />
        <div className="flex items-center gap-1.5 pt-0.5">
          <Skeleton className="h-4 w-12 rounded-full" />
          <Skeleton className="h-4 w-14 rounded-full" />
        </div>
      </div>
    </div>
  )
}

export function TileGridSkeleton({ count = 8 }: { count?: number }) {
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4">
      {Array.from({ length: count }).map((_, i) => (
        <TileSkeleton key={i} />
      ))}
    </div>
  )
}

// ─── Detail panel skeleton ────────────────────────────────────────────────────
// Used in: ItemDetail (Sheet), FileDetail (Sheet)
export function DetailPanelSkeleton() {
  return (
    <div className="flex flex-col gap-5 p-6">
      {/* header: image + title */}
      <div className="flex items-start gap-4">
        <Skeleton className="size-16 rounded-xl shrink-0" />
        <div className="flex-1 space-y-2 pt-1">
          <Skeleton className="h-5 w-40 rounded" />
          <Skeleton className="h-3.5 w-28 rounded" />
          <div className="flex gap-1.5 pt-1">
            <Skeleton className="h-5 w-16 rounded-full" />
            <Skeleton className="h-5 w-20 rounded-full" />
          </div>
        </div>
      </div>

      <Separator />

      {/* stat row */}
      <div className="grid grid-cols-2 gap-2">
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className="rounded-lg border border-border p-3 space-y-1.5">
            <Skeleton className="h-3 w-16 rounded" />
            <Skeleton className="h-4 w-20 rounded" />
          </div>
        ))}
      </div>

      <Separator />

      {/* tab bar */}
      <div className="flex gap-1">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-8 w-20 rounded-md" />
        ))}
      </div>

      {/* body lines */}
      <div className="space-y-3">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="flex justify-between items-center py-2 border-b border-border last:border-0">
            <Skeleton className="h-3.5 w-24 rounded" />
            <Skeleton className={`h-3.5 rounded ${i % 2 === 0 ? "w-32" : "w-20"}`} />
          </div>
        ))}
      </div>

      {/* action buttons */}
      <div className="flex gap-2 pt-2">
        <Skeleton className="h-9 flex-1 rounded-md" />
        <Skeleton className="size-9 rounded-md shrink-0" />
      </div>
    </div>
  )
}

// ─── Showcase wrapper used in UIShowcaseView ──────────────────────────────────
export function SkeletonShowcase() {
  return (
    <div className="space-y-10">

      {/* Stat cards */}
      <div>
        <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground mb-3">
          Stat card skeleton
        </p>
        <StatCardSkeletonRow count={3} />
      </div>

      <Separator />

      {/* Table rows */}
      <div>
        <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground mb-3">
          Table / row skeleton (6 rows)
        </p>
        <TableSkeleton rows={6} />
      </div>

      <Separator />

      {/* Tiles */}
      <div>
        <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground mb-3">
          Tile / grid skeleton (8 cards)
        </p>
        <TileGridSkeleton count={8} />
      </div>

      <Separator />

      {/* Detail panel */}
      <div>
        <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground mb-3">
          Detail-panel skeleton (Sheet / slide-over)
        </p>
        <div className="rounded-xl border border-border bg-card max-w-sm overflow-hidden">
          <DetailPanelSkeleton />
        </div>
      </div>

    </div>
  )
}
