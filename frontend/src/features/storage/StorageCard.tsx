import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { ArrowRight, HardDrive } from "lucide-react"

import { Skeleton } from "@/components/ui/skeleton"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { formatBytes } from "@/lib/intl"
import { cn } from "@/lib/utils"

import { useStorageUsage } from "./hooks"
import type { StorageBreakdown } from "./api"

// StorageCard renders the per-group storage usage panel under
// Settings → Data & storage (#1388). Shows headline used / quota,
// a progress bar (when a quota is set), and a per-category byte
// breakdown so users can see where their space went.
export function StorageCard() {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug
  const { data, isPending, isError } = useStorageUsage()

  if (!slug) {
    return (
      <div className="rounded-xl border border-border bg-card p-4 space-y-3">
        <Header />
        <p className="text-xs text-muted-foreground">{t("settings:storage.noGroup")}</p>
      </div>
    )
  }

  return (
    <div
      className="rounded-xl border border-border bg-card p-4 space-y-3"
      data-testid="storage-card"
    >
      <Header />

      {isError ? (
        <p className="text-xs text-destructive">{t("settings:storage.errorBody")}</p>
      ) : null}

      {isPending ? <UsageSkeleton /> : null}

      {data ? <UsageBody data={data} /> : null}

      {currentGroup?.slug ? (
        <Link
          to={`/g/${encodeURIComponent(currentGroup.slug)}/files`}
          data-testid="storage-card-manage-files"
          className="inline-flex items-center gap-1 text-xs font-medium text-primary hover:underline"
        >
          {t("settings:storage.manageFiles")}
          <ArrowRight className="size-3.5" aria-hidden="true" />
        </Link>
      ) : null}
    </div>
  )
}

function Header() {
  const { t } = useTranslation()
  return (
    <div className="flex items-center gap-2">
      <HardDrive className="size-4 text-muted-foreground" aria-hidden="true" />
      <p className="text-sm font-medium">{t("settings:storage.title")}</p>
    </div>
  )
}

function UsageSkeleton() {
  return (
    <div className="space-y-2" data-testid="storage-card-loading">
      <Skeleton className="h-4 w-32" />
      <Skeleton className="h-2 w-full rounded-full" />
      <div className="flex flex-wrap gap-1.5">
        <Skeleton className="h-5 w-16 rounded-full" />
        <Skeleton className="h-5 w-20 rounded-full" />
        <Skeleton className="h-5 w-14 rounded-full" />
      </div>
    </div>
  )
}

function UsageBody({
  data,
}: {
  data: { used_bytes: number; quota_bytes: number | null; breakdown: StorageBreakdown }
}) {
  const { t } = useTranslation()
  const used = formatBytes(data.used_bytes)
  const quota = data.quota_bytes != null ? formatBytes(data.quota_bytes) : null

  // Cap percent at 100 for the bar width but still surface the real
  // value in the label so over-quota groups read as e.g. "112%".
  const percentRaw =
    data.quota_bytes && data.quota_bytes > 0
      ? Math.round((data.used_bytes / data.quota_bytes) * 100)
      : null
  const barWidth = percentRaw == null ? 0 : Math.min(100, percentRaw)
  const overQuota = percentRaw != null && percentRaw > 100

  return (
    <div className="space-y-3" data-testid="storage-card-body">
      <div className="space-y-1">
        <p className="text-sm font-semibold tabular-nums" data-testid="storage-card-used">
          {quota
            ? t("settings:storage.usageOfQuota", { used, quota })
            : t("settings:storage.unlimited", { used })}
        </p>
        {percentRaw != null ? (
          <>
            <div
              className="h-2 w-full overflow-hidden rounded-full bg-muted"
              role="progressbar"
              aria-valuemin={0}
              aria-valuemax={100}
              aria-valuenow={Math.min(100, percentRaw)}
            >
              <div
                className={cn(
                  "h-full rounded-full transition-all",
                  overQuota ? "bg-destructive" : "bg-primary"
                )}
                style={{ width: `${barWidth}%` }}
              />
            </div>
            <p className="text-xs text-muted-foreground tabular-nums">
              {t("settings:storage.usagePercent", { percent: percentRaw })}
            </p>
          </>
        ) : null}
      </div>

      <BreakdownChips breakdown={data.breakdown} />
    </div>
  )
}

const BREAKDOWN_KEYS: Array<{ key: keyof StorageBreakdown; testid: string }> = [
  { key: "photos", testid: "storage-breakdown-photos" },
  { key: "invoices", testid: "storage-breakdown-invoices" },
  { key: "documents", testid: "storage-breakdown-documents" },
  { key: "exports", testid: "storage-breakdown-exports" },
  { key: "other", testid: "storage-breakdown-other" },
]

function BreakdownChips({ breakdown }: { breakdown: StorageBreakdown }) {
  const { t } = useTranslation()
  return (
    <ul
      className="flex flex-wrap gap-1.5"
      data-testid="storage-card-breakdown"
      aria-label={t("settings:storage.title")}
    >
      {BREAKDOWN_KEYS.map(({ key, testid }) => (
        <li key={key}>
          <span
            className="inline-flex items-center gap-1 rounded-full border border-border bg-muted/40 px-2 py-0.5 text-xs"
            data-testid={testid}
          >
            <span className="text-muted-foreground">
              {t(`settings:storage.breakdown.${key}`)}
            </span>
            <span className="font-medium tabular-nums">{formatBytes(breakdown[key])}</span>
          </span>
        </li>
      ))}
    </ul>
  )
}
