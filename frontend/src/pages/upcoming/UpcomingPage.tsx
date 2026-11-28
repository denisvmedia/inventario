import {
  BellRing,
  CalendarClock,
  CalendarDays,
  HandCoins,
  ShieldCheck,
  type LucideIcon,
} from "lucide-react"
import { useMemo } from "react"
import { useTranslation } from "react-i18next"
import { Link, useSearchParams } from "react-router-dom"

import { useReminderText } from "@/components/reminders/useReminderText"
import { ListLoadError } from "@/components/common/ListLoadError"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { Badge } from "@/components/ui/badge"
import { Card } from "@/components/ui/card"
import { Page, PageHeader } from "@/components/ui/page"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { isSectionHidden, type GroupSection } from "@/features/group/sections"
import { UPCOMING_KINDS, type UpcomingItem, type UpcomingKind } from "@/features/reminders/api"
import { useUpcoming } from "@/features/reminders/hooks"
import { formatDate } from "@/lib/intl"
import { cn } from "@/lib/utils"

type KindFilter = "all" | UpcomingKind
const KIND_FILTERS: readonly KindFilter[] = ["all", ...UPCOMING_KINDS]

const WINDOW_OPTIONS = [30, 90, 365] as const
type WindowDays = (typeof WINDOW_OPTIONS)[number]
const DEFAULT_WINDOW: WindowDays = 30

const KIND_ICONS: Record<UpcomingKind, LucideIcon> = {
  reminder: BellRing,
  warranty: ShieldCheck,
  loan: HandCoins,
  maintenance: CalendarClock,
}

// Item-page tab each kind deep-links to.
const KIND_TABS: Record<UpcomingKind, string | null> = {
  reminder: null,
  warranty: "warranty",
  loan: "lend",
  maintenance: "maintenance",
}

// The group section each kind belongs to. A group that hides the section
// (#1607) gets neither the kind's rows nor its tab.
const KIND_SECTIONS: Record<UpcomingKind, GroupSection | null> = {
  reminder: null,
  warranty: "warranties",
  loan: "loans",
  maintenance: "maintenance",
}

function parseKind(raw: string | null, filters: readonly KindFilter[]): KindFilter {
  return (filters as readonly string[]).includes(raw ?? "") ? (raw as KindFilter) : "all"
}

function parseWindow(raw: string | null): WindowDays {
  const n = Number(raw)
  return (WINDOW_OPTIONS as readonly number[]).includes(n) ? (n as WindowDays) : DEFAULT_WINDOW
}

// UpcomingPage is the group-wide `/g/:slug/upcoming` list (#2805): item
// reminders, warranty expiries, loan due dates and maintenance due dates in
// one date-ordered list. The server merges and sorts; the page filters by
// kind client-side so the per-kind counters stay accurate.
export function UpcomingPage() {
  const { t } = useTranslation(["reminders", "common"])
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const [searchParams, setSearchParams] = useSearchParams()
  const kindShown = (k: UpcomingKind) => {
    const section = KIND_SECTIONS[k]
    return !section || !isSectionHidden(currentGroup, section)
  }
  const kindFilters = KIND_FILTERS.filter((k) => k === "all" || kindShown(k))
  const kind = parseKind(searchParams.get("kind"), kindFilters)
  const windowDays = parseWindow(searchParams.get("days"))
  const upcoming = useUpcoming(windowDays)
  const hidden = currentGroup?.hidden_sections
  const items = useMemo(
    () =>
      (upcoming.data?.items ?? []).filter((it) => {
        const section = it.kind ? KIND_SECTIONS[it.kind] : null
        return !section || !hidden?.includes(section)
      }),
    [upcoming.data?.items, hidden]
  )

  const counts = useMemo(() => {
    const out: Record<KindFilter, number> = {
      all: items.length,
      reminder: 0,
      warranty: 0,
      loan: 0,
      maintenance: 0,
    }
    for (const it of items) {
      if (it.kind) out[it.kind] += 1
    }
    return out
  }, [items])

  const rows = kind === "all" ? items : items.filter((it) => it.kind === kind)

  function setParam(name: string, value: string | null) {
    const params = new URLSearchParams(searchParams)
    if (value === null) params.delete(name)
    else params.set(name, value)
    setSearchParams(params, { replace: true })
  }

  return (
    <Page width="wide" data-testid="page-upcoming">
      <RouteTitle title={t("reminders:upcoming.title")} />
      <PageHeader
        title={t("reminders:upcoming.title")}
        subtitle={t("reminders:upcoming.subtitle")}
        actions={
          <Select
            value={String(windowDays)}
            onValueChange={(v) => setParam("days", v === String(DEFAULT_WINDOW) ? null : v)}
          >
            <SelectTrigger
              className="w-40"
              aria-label={t("reminders:upcoming.windowLabel")}
              data-testid="upcoming-window"
            >
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {WINDOW_OPTIONS.map((d) => (
                <SelectItem key={d} value={String(d)}>
                  {d === 365
                    ? t("reminders:upcoming.windowYear")
                    : t("reminders:upcoming.windowDays", { count: d })}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        }
      />

      <div
        role="tablist"
        className="flex gap-1 overflow-x-auto border-b border-border"
        data-testid="upcoming-tabs"
      >
        {kindFilters.map((k) => (
          <button
            key={k}
            role="tab"
            type="button"
            aria-selected={kind === k}
            onClick={() => setParam("kind", k === "all" ? null : k)}
            data-testid={`upcoming-tab-${k}`}
            className={cn(
              "-mb-px inline-flex shrink-0 items-center gap-1.5 border-b-2 px-3 py-2 text-sm",
              kind === k
                ? "border-primary text-foreground"
                : "border-transparent text-muted-foreground hover:text-foreground"
            )}
          >
            {kindFilterLabel(t, k)}
            {counts[k] > 0 ? (
              <Badge variant="outline" className="h-4 px-1 text-[10px]">
                {counts[k]}
              </Badge>
            ) : null}
          </button>
        ))}
      </div>

      {upcoming.isPending ? (
        <div className="flex flex-col gap-2" data-testid="upcoming-loading">
          <Skeleton className="h-14" />
          <Skeleton className="h-14" />
          <Skeleton className="h-14" />
        </div>
      ) : upcoming.isError ? (
        <ListLoadError testId="upcoming-error" onRetry={() => void upcoming.refetch()} />
      ) : rows.length === 0 ? (
        <div
          className="flex flex-col items-center gap-2 py-12 text-center"
          data-testid="upcoming-empty"
        >
          <CalendarDays className="size-8 text-muted-foreground/30" aria-hidden="true" />
          <p className="text-sm text-muted-foreground">{t("reminders:upcoming.empty")}</p>
        </div>
      ) : (
        <Card className="overflow-hidden p-0" data-testid="upcoming-list">
          <ul>
            {rows.map((it, i) => (
              <li key={`${it.kind}-${it.source_id}-${it.date}`}>
                {i > 0 ? <Separator /> : null}
                <UpcomingRow item={it} slug={slug} />
              </li>
            ))}
          </ul>
        </Card>
      )}
    </Page>
  )
}

function UpcomingRow({ item, slug }: { item: UpcomingItem; slug: string }) {
  const { t } = useTranslation(["reminders"])
  const text = useReminderText()
  const kind = item.kind ?? "reminder"
  const Icon = KIND_ICONS[kind]
  const days = item.days_left ?? 0
  const overdue = days < 0
  const soon = !overdue && days <= 7
  const commodity = item.commodity
  const subtitle = [kindLabel(t, kind), item.title].filter(Boolean).join(" · ")
  const tab = KIND_TABS[kind]
  const tone = overdue ? "text-destructive" : soon ? "text-primary" : "text-muted-foreground"

  const inner = (
    <>
      <Icon className={cn("size-5 shrink-0", tone)} aria-hidden="true" />
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">
          {commodity?.name ?? t("reminders:upcoming.unknownItem")}
        </p>
        <p className="truncate text-xs text-muted-foreground">{subtitle}</p>
      </div>
      <div className="shrink-0 text-right">
        <p className="text-sm font-medium">{item.date ? formatDate(item.date) : "—"}</p>
        <p className={cn("text-xs", tone)}>
          {overdue ? t("reminders:days.overdue", { count: -days }) : text.daysLeft(days)}
        </p>
      </div>
    </>
  )
  const className =
    "flex w-full items-center gap-4 px-5 py-4 text-left transition-colors hover:bg-muted/50"
  const testId = `upcoming-row-${kind}-${item.source_id ?? "unknown"}`
  if (!commodity?.id) {
    return (
      <div className={className} data-testid={testId}>
        {inner}
      </div>
    )
  }
  const href = `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(commodity.id)}${
    tab ? `?tab=${tab}` : ""
  }`
  return (
    <Link to={href} className={className} data-testid={testId}>
      {inner}
    </Link>
  )
}

type TFn = ReturnType<typeof useTranslation>["t"]

function kindLabel(t: TFn, kind: UpcomingKind): string {
  switch (kind) {
    case "warranty":
      return t("reminders:kind.warranty")
    case "loan":
      return t("reminders:kind.loan")
    case "maintenance":
      return t("reminders:kind.maintenance")
    default:
      return t("reminders:kind.reminder")
  }
}

function kindFilterLabel(t: TFn, kind: KindFilter): string {
  switch (kind) {
    case "all":
      return t("reminders:upcoming.tabAll")
    case "reminder":
      return t("reminders:upcoming.tabReminders")
    case "warranty":
      return t("reminders:upcoming.tabWarranties")
    case "loan":
      return t("reminders:upcoming.tabLoans")
    default:
      return t("reminders:upcoming.tabMaintenance")
  }
}
