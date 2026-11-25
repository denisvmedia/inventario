import { useEffect, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Link, useLocation, useNavigate, useSearchParams } from "react-router-dom"
import { useQueryClient } from "@tanstack/react-query"
import {
  ChevronRight,
  CircleCheck,
  CircleAlert,
  FilePen,
  Inbox,
  MapPinOff,
  Package,
  Paperclip,
  ScanText,
  Tag,
  Trash2,
  type LucideIcon,
} from "lucide-react"

import { ListLoadError } from "@/components/common/ListLoadError"
import { DocumentSource } from "@/components/items/DocumentSource"
import { StateBadge } from "@/components/items/IntakeCard"
import { Pagination } from "@/components/common/Pagination"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Page, PageHeader } from "@/components/ui/page"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { commodityKeys } from "@/features/commodities/keys"
import { commodityListHref, DEFAULT_COMMODITY_QUERY } from "@/features/commodities/query"
import { useDiscardDraft } from "@/features/drafts/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useMyGroupRole } from "@/features/group/roles"
import {
  INBOX_REASONS,
  setCommodityDraft,
  type InboxCounts,
  type InboxEntry,
  type InboxReason,
} from "@/features/inbox/api"
import { useInbox, useInboxCount, useSetItemDraft } from "@/features/inbox/hooks"
import { reviewRefusal } from "@/features/inbox/reviewRefusal"
import { useDeleteIntakeSession } from "@/features/intake/hooks"
import { useIntakeSummaryText } from "@/features/intake/summary"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatRelative } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"
import { cn } from "@/lib/utils"

const PER_PAGE = 50

type ReasonFilter = "all" | InboxReason
const REASON_FILTERS: readonly ReasonFilter[] = ["all", ...INBOX_REASONS]

function parseReason(raw: string | null): ReasonFilter {
  return (REASON_FILTERS as readonly string[]).includes(raw ?? "") ? (raw as ReasonFilter) : "all"
}

function parsePage(raw: string | null): number {
  const n = Number(raw)
  return Number.isInteger(n) && n > 1 ? n : 1
}

type TFn = ReturnType<typeof useTranslation>["t"]

// InboxPage is `/g/:slug/inbox` (#2794): the group's unfinished work in one
// list. Each entry is one record with every reason it is listed for, and
// offers the step that clears it. The server merges the sources and counts
// each record once; the tabs filter by reason on the server too, so their
// counts stay right on every page.
//
// Missing details (no location, no tags) are not unfinished work, so they
// stay out of the list and its count. The section under the list links to
// the item filters that show them.
export function InboxPage() {
  const { t } = useTranslation(["inbox", "common"])
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const [searchParams, setSearchParams] = useSearchParams()
  const reason = parseReason(searchParams.get("reason"))
  const page = parsePage(searchParams.get("page"))
  const inbox = useInbox({
    reason: reason === "all" ? undefined : reason,
    page,
    perPage: PER_PAGE,
  })
  // Every page carries the counts of the whole inbox, so the tabs read
  // them from the page on screen. While the page of a newly selected tab
  // loads there is none, so the tabs keep the sidebar's count.
  const count = useInboxCount()
  const counts = inbox.data?.counts ?? count.data

  function setReason(next: string) {
    const params = new URLSearchParams(searchParams)
    if (next === "all") params.delete("reason")
    else params.set("reason", next)
    params.delete("page")
    setSearchParams(params, { replace: true })
  }

  function setPage(next: number) {
    const params = new URLSearchParams(searchParams)
    if (next <= 1) params.delete("page")
    else params.set("page", String(next))
    setSearchParams(params, { replace: true })
  }

  return (
    <Page width="wide" data-testid="page-inbox">
      <RouteTitle title={t("inbox:title")} />
      <PageHeader
        title={t("inbox:title")}
        subtitle={t("inbox:subtitle")}
        subtitleClassName="max-w-prose"
      />

      <Tabs value={reason} onValueChange={setReason} data-testid="inbox-tabs">
        <TabsList variant="line" className="justify-start">
          {REASON_FILTERS.map((r) => (
            <TabsTrigger key={r} value={r} data-testid={`inbox-tab-${r}`}>
              {tabLabel(t, r, countFor(counts, r))}
            </TabsTrigger>
          ))}
        </TabsList>
        {REASON_FILTERS.map((r) => (
          <TabsContent key={r} value={r} className="mt-4">
            <InboxList slug={slug} reason={r} page={page} onPage={setPage} inbox={inbox} />
          </TabsContent>
        ))}
      </Tabs>

      <MissingDetails slug={slug} />
    </Page>
  )
}

function countFor(counts: InboxCounts | undefined, r: ReasonFilter): number | null {
  if (!counts) return null
  return r === "all" ? counts.total : (counts.reasons[r] ?? 0)
}

function tabLabel(t: TFn, r: ReasonFilter, n: number | null): string {
  switch (r) {
    case "draft":
      return n === null ? t("inbox:tabs.draft") : t("inbox:tabs.draftCount", { n })
    case "item_draft":
      return n === null ? t("inbox:tabs.itemDraft") : t("inbox:tabs.itemDraftCount", { n })
    case "intake":
      return n === null ? t("inbox:tabs.intake") : t("inbox:tabs.intakeCount", { n })
    default:
      return n === null ? t("inbox:tabs.all") : t("inbox:tabs.allCount", { n })
  }
}

interface InboxListProps {
  slug: string
  reason: ReasonFilter
  page: number
  onPage: (page: number) => void
  inbox: ReturnType<typeof useInbox>
}

function InboxList({ slug, reason, page, onPage, inbox }: InboxListProps) {
  const { t } = useTranslation(["inbox"])
  const role = useMyGroupRole()
  // Members who may edit content; nobody while the role is still loading.
  const canWrite = role === "user" || role === "admin" || role === "owner"
  const total = inbox.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PER_PAGE))

  // Clearing the last entries of the last page leaves the page past the
  // end; step back to the page that now ends the list.
  useEffect(() => {
    if (inbox.data && !inbox.isPlaceholderData && page > totalPages) onPage(totalPages)
  }, [inbox.data, inbox.isPlaceholderData, page, totalPages, onPage])

  if (inbox.isPending) {
    return (
      <div className="flex flex-col gap-2" data-testid="inbox-loading">
        <Skeleton className="h-16" />
        <Skeleton className="h-16" />
        <Skeleton className="h-16" />
      </div>
    )
  }
  if (inbox.isError) {
    return <ListLoadError testId="inbox-error" onRetry={() => void inbox.refetch()} />
  }
  const entries = inbox.data?.entries ?? []
  if (entries.length === 0) {
    return (
      <div className="flex flex-col items-center gap-2 py-12 text-center" data-testid="inbox-empty">
        <CircleCheck className="size-8 text-muted-foreground/30" aria-hidden="true" />
        <p className="text-sm font-medium">
          {reason === "all" ? t("inbox:empty.title") : t("inbox:empty.filteredTitle")}
        </p>
        <p className="max-w-prose text-sm text-muted-foreground">{t("inbox:empty.description")}</p>
      </div>
    )
  }
  return (
    <div className="flex flex-col gap-4">
      <Card className="overflow-hidden p-0" data-testid="inbox-list">
        <ul>
          {entries.map((entry, i) => (
            <li key={`${entry.kind}-${entry.id}`}>
              {i > 0 ? <Separator /> : null}
              <InboxRow entry={entry} slug={slug} canWrite={canWrite} />
            </li>
          ))}
        </ul>
      </Card>
      {totalPages > 1 ? (
        <Pagination
          page={page}
          totalPages={totalPages}
          onChange={onPage}
          testId="inbox-pagination"
        />
      ) : null}
    </div>
  )
}

interface RowProps {
  entry: InboxEntry
  slug: string
  canWrite: boolean
}

function InboxRow(props: RowProps) {
  switch (props.entry.kind) {
    case "draft":
      return <DraftRow {...props} />
    case "commodity":
      return <ItemRow {...props} />
    case "intake_session":
      return <IntakeRow {...props} />
    default:
      // A kind a newer server added: shown with its reasons, without an
      // action this client does not know.
      return (
        <RowShell
          icon={CircleAlert}
          title={props.entry.title}
          reasons={props.entry.reasons}
          testId={`inbox-row-${props.entry.kind}-${props.entry.id}`}
        />
      )
  }
}

function reasonLabel(t: TFn, reason: string): string {
  switch (reason) {
    case "draft":
      return t("inbox:reason.draft")
    case "item_draft":
      return t("inbox:reason.itemDraft")
    case "intake":
      return t("inbox:reason.intake")
    default:
      return t("inbox:reason.other")
  }
}

interface RowShellProps {
  icon: LucideIcon
  title: ReactNode
  reasons: string[]
  meta?: ReactNode
  actions?: ReactNode
  testId: string
}

// One inbox row: what the record is, why it is listed, and the action.
function RowShell({ icon: Icon, title, reasons, meta, actions, testId }: RowShellProps) {
  const { t } = useTranslation(["inbox"])
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-2 px-5 py-4" data-testid={testId}>
      <Icon className="size-5 shrink-0 text-muted-foreground" aria-hidden="true" />
      <div className="min-w-0 flex-1 basis-48">
        <div className="truncate text-sm font-medium">{title}</div>
        <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          {reasons.map((r) => (
            <Badge key={r} variant="secondary" data-testid="inbox-reason">
              {reasonLabel(t, r)}
            </Badge>
          ))}
          {meta}
        </div>
      </div>
      {actions ? <div className="flex shrink-0 items-center gap-2">{actions}</div> : null}
    </div>
  )
}

// A server draft of a new item (#2812): continue it in the item dialog over
// the inbox, or discard it.
function DraftRow({ entry, slug, canWrite }: RowProps) {
  const { t } = useTranslation(["inbox", "commodities"])
  const navigate = useNavigate()
  const location = useLocation()
  const toast = useAppToast()
  const confirm = useConfirm()
  const discard = useDiscardDraft()
  const title = entry.title || t("commodities:drafts.untitled")
  const saved = entry.updatedAt ? formatRelative(entry.updatedAt) : ""

  function resume() {
    navigate(
      `/g/${encodeURIComponent(slug)}/commodities/new?draft=${encodeURIComponent(entry.id)}`,
      { state: { background: location } }
    )
  }

  async function handleDiscard() {
    const ok = await confirm({
      title: t("commodities:drafts.discardTitle", { title }),
      description: t("commodities:drafts.discardDescription"),
      confirmLabel: t("commodities:drafts.discard"),
      destructive: true,
    })
    if (!ok) return
    try {
      await discard.mutateAsync(entry.id)
      toast.success(t("commodities:drafts.discarded", { title }))
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:drafts.discardFailed")))
    }
  }

  return (
    <RowShell
      icon={FilePen}
      testId={`inbox-row-draft-${entry.id}`}
      title={<span className={cn(!entry.title && "italic text-muted-foreground")}>{title}</span>}
      reasons={entry.reasons}
      meta={
        <>
          {entry.source ? (
            <DocumentSource source={entry.source} testId={`inbox-source-${entry.id}`} />
          ) : null}
          {saved ? <span>{t("commodities:drafts.savedAgo", { when: saved })}</span> : null}
          {entry.fileCount > 0 ? (
            <span className="inline-flex items-center gap-1">
              <Paperclip className="size-3" aria-hidden="true" />
              {t("commodities:drafts.fileCount", { count: entry.fileCount })}
            </span>
          ) : null}
        </>
      }
      actions={
        canWrite ? (
          <>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={resume}
              aria-label={t("inbox:continueLabel", { title })}
              data-testid={`inbox-resume-${entry.id}`}
            >
              {t("commodities:drafts.resume")}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="size-8 text-muted-foreground hover:text-destructive"
              onClick={() => void handleDiscard()}
              disabled={discard.isPending}
              aria-label={t("commodities:drafts.discardLabel", { title })}
              data-testid={`inbox-discard-${entry.id}`}
            >
              <Trash2 className="size-4" aria-hidden="true" />
            </Button>
          </>
        ) : null
      }
    />
  )
}

// An item marked as draft: its details were saved without the checks a
// reviewed item passes. The title opens the item to look it over; "Mark as
// reviewed" clears the mark, which the server accepts only when the item
// passes those checks. Unknown purchase data passes (#2813), so a reviewed
// item does not come back for it.
function ItemRow({ entry, slug, canWrite }: RowProps) {
  const { t } = useTranslation(["inbox"])
  const location = useLocation()
  const navigate = useNavigate()
  const toast = useAppToast()
  const qc = useQueryClient()
  const setDraft = useSetItemDraft()
  const href = `/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(entry.id)}`
  const title = entry.title

  function openItem() {
    navigate(href, { state: { background: location } })
  }

  async function markReviewed() {
    try {
      await setDraft.mutateAsync({ id: entry.id, draft: false })
    } catch (err) {
      const action = { label: t("inbox:openItem"), onClick: openItem }
      const problems = reviewRefusal(t, err)
      if (problems) toast.error(t("inbox:reviewFailed"), { description: problems, action })
      else toast.error(parseServerError(err, t("inbox:reviewFailed")), { action })
      return
    }
    toast.success(t("inbox:reviewed", { title }), {
      // The row is gone by the time the toast is clicked, so the undo
      // calls the API itself rather than through this row's mutation.
      action: {
        label: t("inbox:undo"),
        onClick: () => {
          setCommodityDraft(entry.id, true).then(
            () => void qc.invalidateQueries({ queryKey: commodityKeys.group(slug) }),
            (err: unknown) => toast.error(parseServerError(err, t("inbox:undoFailed")))
          )
        },
      },
    })
  }

  return (
    <RowShell
      icon={Package}
      testId={`inbox-row-commodity-${entry.id}`}
      title={
        <Link
          to={href}
          state={{ background: location }}
          className="hover:underline"
          data-testid={`inbox-open-${entry.id}`}
        >
          {title}
        </Link>
      }
      reasons={entry.reasons}
      meta={<span>{t("inbox:itemDraftHint")}</span>}
      actions={
        canWrite ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void markReviewed()}
            disabled={setDraft.isPending}
            aria-label={t("inbox:markReviewedLabel", { title })}
            data-testid={`inbox-review-${entry.id}`}
          >
            {t("inbox:markReviewed")}
          </Button>
        ) : null
      }
    />
  )
}

// A batch of files (#2810) that is not finished: a file waits for its scan
// or could not be scanned, or a product waits for a decision or its draft.
// Review opens the batch, where each of these is cleared; a batch that is
// not being scanned can be removed here, which keeps the drafts made from it.
function IntakeRow({ entry, slug, canWrite }: RowProps) {
  const { t } = useTranslation(["inbox", "commodities"])
  const navigate = useNavigate()
  const toast = useAppToast()
  const confirm = useConfirm()
  const summaryText = useIntakeSummaryText()
  const remove = useDeleteIntakeSession()
  const state = entry.intake?.state ?? "processing"
  const title = t("inbox:intakeTitle", { count: entry.fileCount })
  const added = entry.updatedAt ? formatRelative(entry.updatedAt) : ""
  const summary = entry.intake ? summaryText(entry.intake.summary) : ""
  const action =
    state === "review" && canWrite ? t("commodities:intake.review") : t("commodities:intake.open")

  async function handleRemove() {
    const ok = await confirm({
      title: t("commodities:intake.removeTitle"),
      description: t("commodities:intake.removeDescription"),
      confirmLabel: t("commodities:intake.remove"),
      destructive: true,
    })
    if (!ok) return
    try {
      await remove.mutateAsync(entry.id)
      toast.success(t("commodities:intake.removed"))
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:intake.removeFailed")))
    }
  }

  return (
    <RowShell
      icon={ScanText}
      testId={`inbox-row-intake_session-${entry.id}`}
      title={
        <span className="inline-flex flex-wrap items-center gap-2">
          <span>{title}</span>
          <StateBadge state={state} />
        </span>
      }
      reasons={entry.reasons}
      meta={
        <span data-testid={`inbox-intake-summary-${entry.id}`}>
          {[added ? t("commodities:intake.addedAgo", { when: added }) : "", summary]
            .filter(Boolean)
            .join(" · ")}
        </span>
      }
      actions={
        <>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() =>
              navigate(`/g/${encodeURIComponent(slug)}/intake/${encodeURIComponent(entry.id)}`)
            }
            aria-label={t("inbox:intakeActionLabel", { action, title })}
            data-testid={`inbox-intake-open-${entry.id}`}
          >
            {action}
          </Button>
          {canWrite && state !== "processing" ? (
            <Button
              type="button"
              variant="ghost"
              size="icon"
              className="size-8 text-muted-foreground hover:text-destructive"
              onClick={() => void handleRemove()}
              disabled={remove.isPending}
              aria-label={t("commodities:intake.removeLabel")}
              data-testid={`inbox-intake-remove-${entry.id}`}
            >
              <Trash2 className="size-4" aria-hidden="true" />
            </Button>
          ) : null}
        </>
      }
    />
  )
}

// Optional checks on item details. An unknown location or a missing tag is
// not unfinished work, so these open the items list with the matching
// filter instead of adding to the inbox.
function MissingDetails({ slug }: { slug: string }) {
  const { t } = useTranslation(["inbox"])
  if (!slug) return null
  const links: { key: string; icon: LucideIcon; label: string; href: string }[] = [
    {
      key: "no-location",
      icon: MapPinOff,
      label: t("inbox:missing.noLocation"),
      href: commodityListHref(slug, { unassigned: true }),
    },
    {
      key: "untagged",
      icon: Tag,
      label: t("inbox:missing.untagged"),
      href: commodityListHref(slug, {
        tags: { ...DEFAULT_COMMODITY_QUERY.tags, untagged: true },
      }),
    },
  ]
  return (
    <Card className="gap-3 py-4" data-testid="inbox-missing">
      <CardHeader className="px-4">
        <CardTitle className="flex items-center gap-2 text-base">
          <Inbox className="size-4 text-muted-foreground" aria-hidden="true" />
          {t("inbox:missing.title")}
        </CardTitle>
        <CardDescription>{t("inbox:missing.description")}</CardDescription>
      </CardHeader>
      <CardContent className="px-4">
        <ul className="divide-y divide-border">
          {links.map((l) => {
            const Icon = l.icon
            return (
              <li key={l.key}>
                <Link
                  to={l.href}
                  className="flex items-center gap-3 py-2 text-sm hover:text-primary"
                  data-testid={`inbox-missing-${l.key}`}
                >
                  <Icon className="size-4 text-muted-foreground" aria-hidden="true" />
                  <span className="flex-1">{l.label}</span>
                  <ChevronRight className="size-4 text-muted-foreground" aria-hidden="true" />
                </Link>
              </li>
            )
          })}
        </ul>
      </CardContent>
    </Card>
  )
}
