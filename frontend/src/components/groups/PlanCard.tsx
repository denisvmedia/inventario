import { useTranslation } from "react-i18next"
import { FileText, Info, MapPin, Package, Zap } from "lucide-react"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { type Plan, type PlanUsage } from "@/features/plan/api"
import { useGroupPlan } from "@/features/plan/hooks"
import { cn } from "@/lib/utils"

// PlanCard renders the GroupSettings Plan & quota surface (mock parity
// with `design-mocks/.../GroupSettingsView.tsx` lines 31-79):
// - Icon + plan name + Active badge + owner line on the left.
// - Upgrade CTA on the right (placeholder — billing flow lives elsewhere).
// - Three usage chips (items / locations / storage) under the header.
// - Info banner about owner-tied capabilities at the bottom.
//
// Resolves the plan + per-group usage via `useGroupPlan(slug)`; ownerName
// is passed in from the parent so the card stays decoupled from the
// members query (the parent already fetches members for other reasons).
interface PlanCardProps {
  groupSlug: string | null
  ownerName: string | null
  className?: string
}

export function PlanCard({ groupSlug, ownerName, className }: PlanCardProps) {
  const { t } = useTranslation()
  const planQuery = useGroupPlan(groupSlug)

  // Order matters: when the request fails, React Query keeps `data`
  // undefined, so an `isLoading || !data` check would mask the error
  // forever (`isLoading` flips to false but `data` never arrives, and
  // the skeleton sticks). Check `isError` first so failures surface.
  if (planQuery.isError) {
    return (
      <Alert variant="destructive" className={className} data-testid="plan-card-error">
        <AlertDescription>{t("groups:settings.plan.errorGeneric")}</AlertDescription>
      </Alert>
    )
  }
  if (!planQuery.data) {
    return <PlanCardSkeleton className={className} />
  }

  const { plan, usage } = planQuery.data
  if (!plan || !usage) {
    return <PlanCardSkeleton className={className} />
  }

  return (
    <div
      className={cn("rounded-xl border border-border bg-card p-6 space-y-4", className)}
      data-testid="plan-card"
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-3 min-w-0">
          <div className="flex size-9 items-center justify-center rounded-lg bg-accent/20 shrink-0">
            <Zap className="size-4 text-accent-foreground" aria-hidden="true" />
          </div>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              {/* h3 not h2: the enclosing Info section already renders the
                  section-level h2 (#1887). PlanCard's card title is the
                  next level down. */}
              <h3 className="text-base font-semibold truncate" data-testid="plan-card-name">
                {t("groups:settings.plan.title", {
                  name: plan.name ?? "—",
                })}
              </h3>
              <Badge variant="secondary" className="text-xs">
                {t("groups:settings.plan.activeBadge")}
              </Badge>
            </div>
            <p className="text-sm text-muted-foreground mt-0.5">
              {ownerName
                ? t("groups:settings.plan.ownerLine", { name: ownerName })
                : t("groups:settings.plan.ownerLineUnknown")}
            </p>
          </div>
        </div>
        {/* Upgrade is a placeholder CTA until the plan-catalogue page
            lands (#1389 follow-up). Kept visible to mirror the mock and
            signal that an upgrade path exists; disabled so users don't
            navigate into a 404. The button keeps its testid so the e2e
            harness can assert it's mounted regardless. */}
        <Button
          variant="outline"
          size="sm"
          disabled
          title={t("groups:settings.plan.upgrade")}
          data-testid="plan-card-upgrade"
        >
          {t("groups:settings.plan.upgrade")}
        </Button>
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <UsageChip
          icon={Package}
          label={t("groups:settings.plan.chips.items")}
          value={usage.items ?? 0}
          limit={plan.max_items}
          testId="plan-card-chip-items"
        />
        <UsageChip
          icon={MapPin}
          label={t("groups:settings.plan.chips.locations")}
          value={usage.locations ?? 0}
          limit={plan.max_locations}
          testId="plan-card-chip-locations"
        />
        <UsageChip
          icon={FileText}
          label={t("groups:settings.plan.chips.storage")}
          value={formatBytes(usage.storage_bytes ?? 0)}
          limit={plan.max_storage_bytes != null ? formatBytes(plan.max_storage_bytes) : null}
          testId="plan-card-chip-storage"
        />
      </div>

      <div className="flex items-start gap-3 rounded-lg border border-border bg-muted/40 px-4 py-3">
        <Info className="size-4 text-muted-foreground shrink-0 mt-0.5" aria-hidden="true" />
        <p className="text-sm text-muted-foreground leading-relaxed">
          {t("groups:settings.plan.infoBanner")}
        </p>
      </div>
    </div>
  )
}

interface UsageChipProps {
  icon: typeof Package
  label: string
  // Counts (items / locations) come in as numbers; storage is pre-formatted
  // to "1.2 GB" by the caller because the X / Y display needs the same
  // unit on both sides.
  value: number | string
  // null = "Unlimited" — both for plans that uncap the resource and for
  // the storage chip when `max_storage_bytes` is null in the API payload.
  limit: number | string | null | undefined
  testId: string
}

function UsageChip({ icon: Icon, label, value, limit, testId }: UsageChipProps) {
  const { t } = useTranslation()
  return (
    <div className="rounded-lg border border-border bg-muted/30 px-3 py-2.5" data-testid={testId}>
      <div className="flex items-center gap-1.5 mb-1">
        <Icon className="size-3.5 text-muted-foreground" aria-hidden="true" />
        <p className="text-xs text-muted-foreground font-medium">{label}</p>
      </div>
      <p className="text-sm font-semibold">
        {value}{" "}
        <span className="text-muted-foreground font-normal">
          / {limit ?? t("groups:settings.plan.chips.unlimited")}
        </span>
      </p>
    </div>
  )
}

function PlanCardSkeleton({ className }: { className?: string }) {
  return (
    <div
      className={cn("rounded-xl border border-border bg-card p-6 space-y-4", className)}
      data-testid="plan-card-skeleton"
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-3 min-w-0 flex-1">
          <Skeleton className="size-9 rounded-lg shrink-0" />
          <div className="space-y-2 flex-1">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-3 w-44" />
          </div>
        </div>
        <Skeleton className="h-8 w-20" />
      </div>
      <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
        <Skeleton className="h-14 w-full rounded-lg" />
        <Skeleton className="h-14 w-full rounded-lg" />
        <Skeleton className="h-14 w-full rounded-lg" />
      </div>
      <Skeleton className="h-12 w-full rounded-lg" />
    </div>
  )
}

// formatBytes renders a byte count as a short human string ("1.2 GB",
// "300 MB", "12 KB"). Kept local because the only usage right now is
// inside the Plan card; if a second caller appears, move it to
// `@/lib/format`.
function formatBytes(bytes: number): string {
  if (bytes <= 0) return "0 B"
  const units = ["B", "KB", "MB", "GB", "TB"]
  const exp = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  const value = bytes / Math.pow(1024, exp)
  // Whole bytes / KB / MB look weird with a trailing ".0", so trim it.
  const formatted = value >= 100 || exp === 0 ? value.toFixed(0) : value.toFixed(1)
  return `${formatted} ${units[exp]}`
}

// Helper types re-exported so consumers can type ownerName resolvers
// without importing from features/plan directly.
export type { Plan, PlanUsage }
