// CommodityHistoryTimeline renders the append-only audit log returned by
// `GET /commodities/{id}/events` (issue #1450). Replaces the placeholder
// `StatusHistoryCard` that was scratched together from the commodity row's
// `registered_date` / `last_modified_date` columns.
//
// Each entry shows: actor name, kind icon, kind-aware copy, absolute
// timestamp. Long timelines collapse after `INITIAL_VISIBLE` rows behind
// a "Show more" toggle so the detail page doesn't grow unboundedly while
// the user scrolls items the system has had for years.

import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import {
  CheckCircle2,
  CircleDot,
  HandHelping,
  ImagePlus,
  MapPin,
  PackageCheck,
  Pencil,
  Plus,
  Tag,
  Trash2,
  Wrench,
} from "lucide-react"

import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { Alert, AlertDescription } from "@/components/ui/alert"
import { useAreas } from "@/features/areas/hooks"
import { useCommodityEvents } from "@/features/commodities/hooks"
import type { CommodityEvent, CommodityEventKind } from "@/features/commodities/api"
import { formatDateTime } from "@/lib/intl"

// INITIAL_VISIBLE is the rendered ceiling before the "Show more" toggle
// fires. Picked to match the design mock's ~10-row history rail; the
// toggle reveals the rest in one shot rather than paginating because
// audit logs read top-to-bottom and break the user's scan if they
// jump pages mid-read.
const INITIAL_VISIBLE = 10

interface Props {
  commodityId: string
}

export function CommodityHistoryTimeline({ commodityId }: Props) {
  const { t } = useTranslation()
  const events = useCommodityEvents(commodityId)
  const areas = useAreas({ enabled: true })
  const [expanded, setExpanded] = useState(false)

  const areaName = useMemo(() => {
    const map = new Map<string, string>()
    for (const a of areas.data ?? []) {
      if (a.id) map.set(a.id, a.name ?? "")
    }
    return (id?: string) => (id ? (map.get(id) ?? id) : "")
  }, [areas.data])

  if (events.isLoading) {
    return (
      <Card data-testid="commodity-detail-history">
        <CardHeader>
          <CardTitle className="text-base">{t("commodities:detail.historyTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <ul className="space-y-3" aria-label={t("commodities:detail.historyTitle")}>
            {Array.from({ length: 3 }).map((_, i) => (
              <li key={i} className="flex gap-3 items-start">
                <Skeleton className="size-8 rounded-full" />
                <div className="flex-1">
                  <Skeleton className="h-4 w-48" />
                  <Skeleton className="mt-2 h-3 w-32" />
                </div>
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>
    )
  }

  if (events.isError) {
    return (
      <Card data-testid="commodity-detail-history">
        <CardHeader>
          <CardTitle className="text-base">{t("commodities:detail.historyTitle")}</CardTitle>
        </CardHeader>
        <CardContent>
          <Alert variant="destructive" data-testid="history-error">
            <AlertDescription>{t("commodities:detail.historyError")}</AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    )
  }

  const rows = events.data?.events ?? []
  const visible = expanded ? rows : rows.slice(0, INITIAL_VISIBLE)
  const hidden = Math.max(0, rows.length - visible.length)

  return (
    <Card data-testid="commodity-detail-history">
      <CardHeader>
        <CardTitle className="text-base">{t("commodities:detail.historyTitle")}</CardTitle>
        <CardDescription>{t("commodities:detail.historyDescription")}</CardDescription>
      </CardHeader>
      <CardContent>
        {rows.length === 0 ? (
          <p className="text-sm text-muted-foreground" data-testid="history-empty">
            {t("commodities:detail.historyEmpty")}
          </p>
        ) : (
          <>
            <ol className="relative ml-2 border-l border-border pl-4 space-y-3">
              {visible.map((ev) => (
                <TimelineRow key={ev.id} event={ev} areaName={areaName} />
              ))}
            </ol>
            {hidden > 0 && !expanded ? (
              <Button
                variant="ghost"
                size="sm"
                className="mt-3"
                onClick={() => setExpanded(true)}
                data-testid="history-show-more"
              >
                {t("commodities:detail.historyShowMore", { count: hidden })}
              </Button>
            ) : null}
            {expanded && rows.length > INITIAL_VISIBLE ? (
              <Button
                variant="ghost"
                size="sm"
                className="mt-3"
                onClick={() => setExpanded(false)}
                data-testid="history-show-less"
              >
                {t("commodities:detail.historyShowLess")}
              </Button>
            ) : null}
          </>
        )}
      </CardContent>
    </Card>
  )
}

interface RowProps {
  event: CommodityEvent
  areaName: (id?: string) => string
}

function TimelineRow({ event, areaName }: RowProps) {
  const { t } = useTranslation()
  const actor = event.actor?.name?.trim() || event.actor?.email?.trim() || ""
  // Pin to UTC so the rendered calendar day agrees with the meta-grid
  // "Date added" field (`formatDate` UTC-pins YYYY-MM-DD strings like
  // `registered_date`). Rendering this row's instant in the viewer's
  // local TZ would let it straddle the UTC midnight boundary and
  // disagree with the sibling date-only column (issue #1680). The
  // " UTC" suffix is appended manually because Intl rejects combining
  // `dateStyle`/`timeStyle` with `timeZoneName`, and we want users to
  // realise the time isn't on their wall clock.
  const occurred = event.occurredAt
    ? `${formatDateTime(event.occurredAt, { timeZone: "UTC" })} UTC`
    : ""
  return (
    <li className="text-sm" data-testid={`history-row-${event.kind}`}>
      <span className="absolute -ml-[26px] mt-0.5 grid size-5 place-items-center rounded-full bg-background border border-border text-muted-foreground">
        {renderEventIcon(event.kind, "size-3")}
      </span>
      <div className="flex flex-col gap-0.5">
        <span className="font-medium">{labelFor(event, t, areaName)}</span>
        <span className="text-xs text-muted-foreground">
          {occurred}
          {actor ? ` · ${t("commodities:detail.historyEvent.by", { name: actor })}` : ""}
        </span>
      </div>
    </li>
  )
}

// renderEventIcon emits the per-kind icon as JSX. Returning the element
// (rather than the component reference) sidesteps react-hooks's
// "Cannot create components during render" rule, which flags PascalCase
// locals coming back from a switch.
function renderEventIcon(kind: CommodityEventKind, className: string) {
  const props = { className, "aria-hidden": true } as const
  switch (kind) {
    case "created":
      return <Plus {...props} />
    case "deleted":
      return <Trash2 {...props} />
    case "status_changed":
      return <CheckCircle2 {...props} />
    case "moved":
      return <MapPin {...props} />
    case "price_changed":
      return <Tag {...props} />
    case "cover_changed":
      return <ImagePlus {...props} />
    case "lent_out":
      return <HandHelping {...props} />
    case "returned":
      return <PackageCheck {...props} />
    case "loan_updated":
      return <Pencil {...props} />
    case "sent_for_service":
      return <Wrench {...props} />
    case "back_from_service":
      return <PackageCheck {...props} />
    case "service_updated":
      return <Pencil {...props} />
    case "updated":
      return <Pencil {...props} />
    default:
      // Forwards-compat: unknown kinds get a neutral dot icon and the
      // generic "edited" label so a server that ships a new kind before
      // the FE catches up doesn't crash the timeline.
      return <CircleDot {...props} />
  }
}

// labelFor produces the kind-aware copy. Each branch reads ev.before /
// ev.after sparsely — the BE only persists the fields that changed, so
// `before.area_id` is present on `moved` and absent on others. Never
// throws on missing keys; falls back to a generic message instead.
function labelFor(
  ev: CommodityEvent,
  t: (key: string, opts?: Record<string, unknown>) => string,
  areaName: (id?: string) => string
): string {
  switch (ev.kind) {
    case "created":
      return t("commodities:detail.historyEvent.createdLabel")
    case "deleted":
      return t("commodities:detail.historyEvent.deletedLabel")
    case "status_changed": {
      const fromStatus = stringField(ev.before, "status")
      const toStatus = stringField(ev.after, "status")
      const friendlyTo = toStatus
        ? t(`commodities:status.${toStatus}`, { defaultValue: toStatus })
        : ""
      if (!fromStatus) {
        return t("commodities:detail.historyEvent.statusChangedLabel", { to: friendlyTo })
      }
      const friendlyFrom = t(`commodities:status.${fromStatus}`, { defaultValue: fromStatus })
      return t("commodities:detail.historyEvent.statusChangedFromLabel", {
        from: friendlyFrom,
        to: friendlyTo,
      })
    }
    case "moved": {
      const from = areaName(stringField(ev.before, "area_id"))
      const to = areaName(stringField(ev.after, "area_id"))
      if (from && to) return `${t("commodities:detail.historyEvent.movedLabel")}: ${from} → ${to}`
      if (to) return `${t("commodities:detail.historyEvent.movedLabel")}: ${to}`
      return t("commodities:detail.historyEvent.movedLabel")
    }
    case "price_changed":
      return t("commodities:detail.historyEvent.priceChangedLabel")
    case "cover_changed": {
      const after = stringField(ev.after, "cover_file_id")
      return after
        ? t("commodities:detail.historyEvent.coverChangedLabelSet")
        : t("commodities:detail.historyEvent.coverChangedLabelCleared")
    }
    case "lent_out": {
      const borrower = stringField(ev.after, "borrower_name")
      const dueBack = stringField(ev.after, "due_back_at")
      if (borrower && dueBack) {
        return t("commodities:detail.historyEvent.lentOutLabelDue", {
          borrower,
          dueBack,
        })
      }
      if (borrower) {
        return t("commodities:detail.historyEvent.lentOutLabel", { borrower })
      }
      return t("commodities:detail.historyEvent.lentOutLabelGeneric")
    }
    case "returned": {
      const returnedAt = stringField(ev.after, "returned_at")
      if (returnedAt) {
        return t("commodities:detail.historyEvent.returnedLabelOn", {
          returnedAt,
        })
      }
      return t("commodities:detail.historyEvent.returnedLabel")
    }
    case "loan_updated": {
      const fields = changedLoanFields(ev.before, ev.after)
      if (fields.length === 0) {
        // Defensive: BE skips no-op patches, but if a row sneaks
        // through (older client / hand-edited DB), show the generic
        // copy rather than an empty diff.
        return t("commodities:detail.historyEvent.loanUpdatedLabel")
      }
      const labels = fields
        .map((key) => t(`commodities:detail.historyEvent.loanField.${key}`, { defaultValue: key }))
        .join(", ")
      return t("commodities:detail.historyEvent.loanUpdatedLabelFields", {
        fields: labels,
      })
    }
    case "sent_for_service": {
      const provider = stringField(ev.after, "provider_name")
      const reason = stringField(ev.after, "reason")
      if (provider && reason) {
        return t("commodities:detail.historyEvent.sentForServiceLabelReason", {
          provider,
          reason,
        })
      }
      if (provider) {
        return t("commodities:detail.historyEvent.sentForServiceLabel", { provider })
      }
      return t("commodities:detail.historyEvent.sentForServiceLabelGeneric")
    }
    case "back_from_service": {
      const returnedAt = stringField(ev.after, "returned_at")
      if (returnedAt) {
        return t("commodities:detail.historyEvent.backFromServiceLabelOn", {
          returnedAt,
        })
      }
      return t("commodities:detail.historyEvent.backFromServiceLabel")
    }
    case "service_updated": {
      const fields = changedServiceFields(ev.before, ev.after)
      if (fields.length === 0) {
        return t("commodities:detail.historyEvent.serviceUpdatedLabel")
      }
      const labels = fields
        .map((key) =>
          t(`commodities:detail.historyEvent.serviceField.${key}`, { defaultValue: key })
        )
        .join(", ")
      return t("commodities:detail.historyEvent.serviceUpdatedLabelFields", {
        fields: labels,
      })
    }
    case "updated":
      return t("commodities:detail.historyEvent.updatedLabel")
    default:
      return t("commodities:detail.historyEvent.updatedLabel")
  }
}

// changedLoanFields lists the keys whose values differ between a
// loan_updated event's before and after payloads. The fixed `keys`
// array fixes the rendered order — the BE payload is a Go map (no
// guaranteed key order), so we anchor stability here rather than
// relying on insertion order on the wire.
function changedLoanFields(
  before: Record<string, unknown> | undefined,
  after: Record<string, unknown> | undefined
): string[] {
  if (!before || !after) return []
  const keys = ["borrower_name", "borrower_contact", "borrower_note", "due_back_at"]
  const out: string[] = []
  for (const k of keys) {
    if (before[k] !== after[k]) out.push(k)
  }
  return out
}

// changedServiceFields mirrors changedLoanFields for service_updated
// events — same fixed-order rendering rationale. The cost pair shows
// up as a single "cost" entry rather than two separate rows; the BE
// skips no-op patches, so a single field shifting in either direction
// (amount or currency) still surfaces here.
function changedServiceFields(
  before: Record<string, unknown> | undefined,
  after: Record<string, unknown> | undefined
): string[] {
  if (!before || !after) return []
  const keys = ["provider_name", "provider_contact", "reason", "expected_return_at"]
  const out: string[] = []
  for (const k of keys) {
    if (before[k] !== after[k]) out.push(k)
  }
  if (before.cost_amount !== after.cost_amount || before.cost_currency !== after.cost_currency) {
    out.push("cost")
  }
  return out
}

// stringField pulls a string-typed field from a sparse before/after
// payload. Anything non-string (numbers, nulls, missing) collapses to ""
// so callers can treat the absent + the empty case identically.
function stringField(payload: Record<string, unknown> | undefined, key: string): string {
  if (!payload) return ""
  const v = payload[key]
  return typeof v === "string" ? v : ""
}
