import { useState, type ReactNode } from "react"
import { useTranslation } from "react-i18next"
import { Link, useNavigate, useParams } from "react-router-dom"
import { useQueryClient } from "@tanstack/react-query"
import {
  AlertCircle,
  ArrowLeft,
  ChevronDown,
  ChevronUp,
  FileText,
  Loader2,
  RotateCcw,
  Square,
  Trash2,
} from "lucide-react"

import { StateBadge } from "@/components/items/IntakeCard"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardAction, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Input } from "@/components/ui/input"
import { Page, PageHeader } from "@/components/ui/page"
import { Skeleton } from "@/components/ui/skeleton"
import type { CustomField } from "@/features/customFields/api"
import { useCustomFields } from "@/features/customFields/hooks"
import { customFieldGuessText, customFieldGuessUsable } from "@/features/customFields/values"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useMyGroupRole } from "@/features/group/roles"
import {
  candidatePrice,
  candidateText,
  INTAKE_MAX_QUANTITY,
  INTAKE_REVIEW_FIELDS,
  INTAKE_SOURCE_BUSY,
  INTAKE_SOURCE_HAS_DRAFTS,
  isIntakeError,
  type IntakeCandidate,
  type IntakeReviewField,
  type IntakeSessionDetail,
  type IntakeSource,
} from "@/features/intake/api"
import { useIntakeDraftErrorText, useIntakeSourceErrorText } from "@/features/intake/errors"
import { useIntakeFieldLabel } from "@/features/intake/fields"
import {
  useCancelIntakeSession,
  useCreateIntakeDrafts,
  useDeleteIntakeSession,
  reviewsSettled,
  useIntakeSession,
  useRescanIntakeSource,
  useResumeIntakeSession,
  useReviewIntakeCandidate,
} from "@/features/intake/hooks"
import { useIntakeSummaryText } from "@/features/intake/summary"
import { useItemTypeLookup } from "@/features/itemTypes/hooks"
import { useGroupMigrationLock } from "@/features/currency-migration/lock"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatCurrency, formatDateTime } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"
import { cn } from "@/lib/utils"

// IntakeSessionPage is the review of one batch of dropped files (#2810):
// each purchase with its scan status and the products the scan found. The
// member keeps some of them and creates the drafts. The page reads the batch
// from the server, so it shows the same state on any device.
export function IntakeSessionPage() {
  const { t } = useTranslation()
  const { id = "" } = useParams<{ id: string }>()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const role = useMyGroupRole()
  const migrationLock = useGroupMigrationLock()
  const canWrite =
    (role === "user" || role === "admin" || role === "owner") && !migrationLock.locked
  const session = useIntakeSession(id)
  const itemsPath = `/g/${encodeURIComponent(slug)}/commodities`

  const backLink = (
    <Link
      to={itemsPath}
      className="inline-flex items-center gap-1.5 text-muted-foreground transition-colors hover:text-foreground"
    >
      <ArrowLeft className="size-4" aria-hidden="true" />
      {t("commodities:list.heading")}
    </Link>
  )

  if (session.isPending) {
    return (
      <Page width="narrow" className="gap-4" data-testid="page-intake-loading">
        <Skeleton className="h-8 w-1/2" />
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-32 w-full" />
      </Page>
    )
  }

  if (session.isError || !session.data) {
    return (
      <Page width="narrow" className="gap-4" data-testid="page-intake-not-found">
        <RouteTitle title={t("commodities:intake.page.title")} />
        <Alert variant="destructive">
          <AlertCircle aria-hidden="true" />
          <AlertTitle>{t("commodities:intake.page.notFound")}</AlertTitle>
          <AlertDescription>{t("commodities:intake.page.notFoundBody")}</AlertDescription>
        </Alert>
        <div>{backLink}</div>
      </Page>
    )
  }

  return <SessionView session={session.data} canWrite={canWrite} backLink={backLink} />
}

interface SessionViewProps {
  session: IntakeSessionDetail
  canWrite: boolean
  backLink: ReactNode
}

function SessionView({ session, canWrite, backLink }: SessionViewProps) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const toast = useAppToast()
  const confirm = useConfirm()
  const summaryText = useIntakeSummaryText()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const qc = useQueryClient()
  const createDrafts = useCreateIntakeDrafts()
  const cancel = useCancelIntakeSession()
  const resume = useResumeIntakeSession()
  const remove = useDeleteIntakeSession()
  const [creating, setCreating] = useState(false)

  const s = session.summary
  const toCreate = session.candidates.filter(
    (c) => c.selected && (c.outcome === "pending" || c.outcome === "failed")
  ).length
  const undecided = s.undecided + s.unfinished
  const canResume = s.failed + s.canceled > 0
  const busy =
    creating || createDrafts.isPending || cancel.isPending || resume.isPending || remove.isPending

  async function handleCreateDrafts() {
    setCreating(true)
    try {
      // A choice still being saved would reach the server after the drafts
      // were made from what it held before. The button stays enabled while
      // a choice is saved: the quantity field saves when it loses focus,
      // which the press on the button does, and a button disabled between
      // the press and the release never gets the click.
      await reviewsSettled(qc, slug)
      const result = await createDrafts.mutateAsync(session.id)
      const text = summaryText(result.session.summary)
      if (result.failed > 0) toast.error(text)
      else toast.success(text)
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:intake.page.draftsFailed")))
    } finally {
      setCreating(false)
    }
  }

  async function handleCancel() {
    try {
      await cancel.mutateAsync(session.id)
      toast.success(t("commodities:intake.page.canceled"))
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:intake.page.actionFailed")))
    }
  }

  async function handleResume() {
    try {
      await resume.mutateAsync(session.id)
      toast.success(t("commodities:intake.page.resumed"))
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:intake.page.actionFailed")))
    }
  }

  async function handleRemove() {
    const ok = await confirm({
      title: t("commodities:intake.removeTitle"),
      description: t("commodities:intake.removeDescription"),
      confirmLabel: t("commodities:intake.remove"),
      destructive: true,
    })
    if (!ok) return
    try {
      await remove.mutateAsync(session.id)
      toast.success(t("commodities:intake.removed"))
      navigate(`/g/${encodeURIComponent(slug)}/commodities`)
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:intake.removeFailed")))
    }
  }

  return (
    <Page width="narrow" data-testid="page-intake">
      <RouteTitle title={t("commodities:intake.page.title")} />
      <PageHeader
        size="detail"
        backLink={backLink}
        title={
          <span className="inline-flex flex-wrap items-center gap-2">
            <span>{t("commodities:intake.page.title")}</span>
            <StateBadge state={session.state} />
          </span>
        }
        subtitle={
          <span data-testid="intake-page-summary">
            {[
              session.createdAt
                ? t("commodities:intake.page.added", { when: formatDateTime(session.createdAt) })
                : "",
              summaryText(s),
            ]
              .filter(Boolean)
              .join(" · ")}
          </span>
        }
        subtitleClassName="text-sm"
        actions={
          canWrite ? (
            <>
              {session.state === "processing" ? (
                <Button
                  type="button"
                  variant="outline"
                  className="gap-2"
                  onClick={() => void handleCancel()}
                  disabled={busy}
                  data-testid="intake-cancel"
                >
                  <Square className="size-4" aria-hidden="true" />
                  {t("commodities:intake.page.cancel")}
                </Button>
              ) : null}
              {canResume ? (
                <Button
                  type="button"
                  variant="outline"
                  className="gap-2"
                  onClick={() => void handleResume()}
                  disabled={busy}
                  data-testid="intake-resume"
                >
                  <RotateCcw className="size-4" aria-hidden="true" />
                  {t("commodities:intake.page.resume")}
                </Button>
              ) : null}
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="text-muted-foreground hover:text-destructive"
                onClick={() => void handleRemove()}
                disabled={busy}
                aria-label={t("commodities:intake.removeLabel")}
                data-testid="intake-delete"
              >
                <Trash2 className="size-4" aria-hidden="true" />
              </Button>
            </>
          ) : null
        }
      />

      {session.state === "processing" ? (
        <Alert data-testid="intake-processing-note">
          <Loader2 className="animate-spin" aria-hidden="true" />
          <AlertTitle>{t("commodities:intake.page.processingTitle")}</AlertTitle>
          <AlertDescription>{t("commodities:intake.page.processingBody")}</AlertDescription>
        </Alert>
      ) : null}

      <div className="flex flex-col gap-4" data-testid="intake-sources">
        {session.sources.map((source, index) => (
          <SourceCard
            key={source.id}
            index={index}
            sessionId={session.id}
            source={source}
            candidates={session.candidates
              .filter((c) => c.sourceId === source.id)
              .sort((a, b) => a.sortOrder - b.sortOrder)}
            canWrite={canWrite}
            busy={busy}
          />
        ))}
      </div>

      {canWrite && undecided > 0 ? (
        <div
          className="sticky bottom-0 -mx-6 flex flex-wrap items-center justify-between gap-3 border-t bg-background px-6 py-3"
          data-testid="intake-footer"
        >
          <p className="text-sm text-muted-foreground">
            {toCreate > 0
              ? t("commodities:intake.page.footer", { count: toCreate })
              : t("commodities:intake.page.footerNone")}
          </p>
          <Button
            type="button"
            onClick={() => void handleCreateDrafts()}
            disabled={busy}
            className="gap-2"
            data-testid="intake-create-drafts"
          >
            {creating ? <Loader2 className="size-4 animate-spin" aria-hidden="true" /> : null}
            {toCreate > 0
              ? t("commodities:intake.page.createDrafts", { count: toCreate })
              : t("commodities:intake.page.finish")}
          </Button>
        </div>
      ) : null}
    </Page>
  )
}

interface SourceCardProps {
  index: number
  sessionId: string
  source: IntakeSource
  candidates: IntakeCandidate[]
  canWrite: boolean
  // busy: the session's drafts are being made, or it is being canceled,
  // resumed or deleted. A rescan then waits, so it cannot replace the
  // products the drafts are being made from.
  busy: boolean
}

function SourceCard({ index, sessionId, source, candidates, canWrite, busy }: SourceCardProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const sourceError = useIntakeSourceErrorText()
  const rescan = useRescanIntakeSource()
  const hasDrafts = candidates.some((c) => c.outcome === "created")
  const canRescan =
    canWrite &&
    !hasDrafts &&
    (source.status === "scanned" || source.status === "failed" || source.status === "canceled")
  const first = source.files[0]
  const title = first
    ? source.files.length > 1
      ? t("commodities:intake.source.titleMore", {
          name: first.title,
          count: source.files.length - 1,
        })
      : first.title
    : t("commodities:intake.source.untitled", { index: index + 1 })

  async function handleRescan() {
    try {
      await rescan.mutateAsync({ sessionId, sourceId: source.id })
      toast.success(t("commodities:intake.source.rescanQueued"))
    } catch (err) {
      if (isIntakeError(err, INTAKE_SOURCE_HAS_DRAFTS)) {
        toast.error(t("commodities:intake.source.rescanHasDrafts"))
      } else if (isIntakeError(err, INTAKE_SOURCE_BUSY)) {
        toast.error(t("commodities:intake.source.rescanBusy"))
      } else {
        toast.error(parseServerError(err, t("commodities:intake.page.actionFailed")))
      }
    }
  }

  return (
    <Card className="gap-3 py-4" data-testid={`intake-source-${source.id}`}>
      <CardHeader className="px-4">
        <CardTitle className="flex min-w-0 items-center gap-3 text-base">
          <SourceThumb source={source} />
          <span className="min-w-0 truncate" title={title}>
            {title}
          </span>
          {source.oneItem ? (
            <Badge variant="outline">{t("commodities:intake.source.oneItem")}</Badge>
          ) : null}
        </CardTitle>
        <p
          className={cn(
            "text-sm",
            source.status === "failed" ? "text-destructive" : "text-muted-foreground"
          )}
          data-testid={`intake-source-status-${source.id}`}
        >
          {source.status === "failed" ? (
            sourceError(source.errorCode)
          ) : (
            <SourceStatus source={source} found={candidates.length} />
          )}
        </p>
        {canRescan ? (
          <CardAction>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="gap-2"
              onClick={() => void handleRescan()}
              disabled={busy || rescan.isPending}
              data-testid={`intake-rescan-${source.id}`}
            >
              <RotateCcw className="size-4" aria-hidden="true" />
              {t("commodities:intake.source.rescan")}
            </Button>
          </CardAction>
        ) : null}
      </CardHeader>
      {candidates.length > 0 ? (
        <CardContent className="px-4">
          <ul className="divide-y divide-border" data-testid={`intake-candidates-${source.id}`}>
            {candidates.map((c) => (
              <CandidateRow key={c.id} sessionId={sessionId} candidate={c} canWrite={canWrite} />
            ))}
          </ul>
        </CardContent>
      ) : null}
    </Card>
  )
}

function SourceThumb({ source }: { source: IntakeSource }) {
  const thumb = source.files.find((f) => f.thumbnail)?.thumbnail
  if (thumb) {
    return (
      <img
        src={thumb}
        alt=""
        className="size-10 shrink-0 rounded-md border object-cover"
        loading="lazy"
      />
    )
  }
  return (
    <span className="flex size-10 shrink-0 items-center justify-center rounded-md border bg-muted">
      <FileText className="size-5 text-muted-foreground" aria-hidden="true" />
    </span>
  )
}

function SourceStatus({ source, found }: { source: IntakeSource; found: number }) {
  const { t } = useTranslation()
  switch (source.status) {
    case "pending":
      if (source.waitReason === "rate_limited") {
        return <>{t("commodities:intake.source.rateLimited")}</>
      }
      if (source.waitReason === "retry") return <>{t("commodities:intake.source.retry")}</>
      return <>{t("commodities:intake.source.queued")}</>
    case "scanning":
      return <>{t("commodities:intake.source.scanning")}</>
    case "canceled":
      return <>{t("commodities:intake.source.canceled")}</>
    default:
      return found > 0 ? (
        <>{t("commodities:intake.source.found", { count: found })}</>
      ) : (
        <>{t("commodities:intake.source.nothingFound")}</>
      )
  }
}

interface CandidateRowProps {
  sessionId: string
  candidate: IntakeCandidate
  canWrite: boolean
}

function CandidateRow({ sessionId, candidate, canWrite }: CandidateRowProps) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const review = useReviewIntakeCandidate()
  const draftError = useIntakeDraftErrorText()
  const [open, setOpen] = useState(false)
  // The quantity being typed; null shows the saved one, so a change made on
  // another device shows up once the field is not being edited.
  const [quantity, setQuantity] = useState<string | null>(null)
  const created = candidate.outcome === "created"
  const editable = canWrite && !created
  const name = candidateText(candidate, "name")
  const detail = [candidateText(candidate, "brand"), candidateText(candidate, "model")]
    .filter(Boolean)
    .join(" ")
  const price = candidatePrice(candidate)
  const id = candidate.id
  // The custom fields the scan read a value for, in the group's order. A
  // field deleted since the scan, or a select option removed since, is not
  // shown, and its value goes nowhere.
  const customFields = (useCustomFields().data ?? []).filter((f) => {
    const guess = candidate.customFields[f.id]
    return guess !== undefined && customFieldGuessUsable(f, guess.value)
  })
  const acceptedCount =
    candidate.acceptedFields.length +
    customFields.filter((f) => candidate.acceptedCustomFields.includes(f.id)).length

  async function save(patch: {
    selected?: boolean
    acceptedFields?: string[]
    acceptedCustomFields?: string[]
    quantity?: number
  }) {
    try {
      await review.mutateAsync({ sessionId, candidateId: id, ...patch })
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:intake.candidate.saveFailed")))
    }
  }

  function commitQuantity() {
    if (quantity === null) return
    const n = Number(quantity)
    setQuantity(null)
    if (!Number.isInteger(n) || n < 1 || n > INTAKE_MAX_QUANTITY) return
    if (n !== candidate.quantity) void save({ quantity: n })
  }

  return (
    <li className="flex flex-col gap-2 py-3" data-testid={`intake-candidate-${id}`}>
      <div className="flex min-w-0 items-start gap-3">
        <Checkbox
          id={`intake-keep-${id}`}
          className="mt-0.5"
          checked={candidate.selected}
          disabled={!editable || review.isPending}
          onCheckedChange={(v) => void save({ selected: v === true })}
          aria-label={t("commodities:intake.candidate.keep", {
            name: name || t("commodities:intake.candidate.unnamed"),
          })}
          data-testid={`intake-keep-${id}`}
        />
        <div className="min-w-0 flex-1">
          <label
            htmlFor={`intake-keep-${id}`}
            className={cn(
              "block truncate text-sm font-medium",
              !name && "italic text-muted-foreground"
            )}
          >
            {name || t("commodities:intake.candidate.unnamed")}
          </label>
          <p className="truncate text-xs text-muted-foreground">
            {[
              detail,
              price
                ? price.currency
                  ? formatCurrency(price.amount, price.currency)
                  : String(price.amount)
                : t("commodities:intake.candidate.noPrice"),
            ]
              .filter(Boolean)
              .join(" · ")}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-2">
          <OutcomeBadge candidate={candidate} />
          {created && candidate.draftId && canWrite ? (
            <Button asChild variant="outline" size="sm" data-testid={`intake-continue-${id}`}>
              <Link
                to={`/g/${encodeURIComponent(slug)}/commodities/new?draft=${encodeURIComponent(candidate.draftId)}`}
              >
                {t("commodities:intake.candidate.continue")}
              </Link>
            </Button>
          ) : null}
        </div>
      </div>
      <div className="flex flex-wrap items-center gap-3 pl-7">
        <label className="flex items-center gap-2 text-xs text-muted-foreground">
          {t("commodities:fields.count")}
          <Input
            type="number"
            min={1}
            max={INTAKE_MAX_QUANTITY}
            step={1}
            inputMode="numeric"
            className="h-8 w-20"
            value={quantity ?? String(candidate.quantity)}
            disabled={!editable}
            onChange={(e) => setQuantity(e.target.value)}
            onBlur={commitQuantity}
            data-testid={`intake-quantity-${id}`}
          />
        </label>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          className="gap-1"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          data-testid={`intake-fields-toggle-${id}`}
        >
          {open ? (
            <ChevronUp className="size-4" aria-hidden="true" />
          ) : (
            <ChevronDown className="size-4" aria-hidden="true" />
          )}
          {t("commodities:intake.candidate.fields", { count: acceptedCount })}
        </Button>
        {candidate.outcome === "failed" ? (
          <span className="text-xs text-destructive">{draftError(candidate.errorCode)}</span>
        ) : null}
      </div>
      {open ? (
        <FieldList
          candidate={candidate}
          customFields={customFields}
          editable={editable && !review.isPending}
          onChange={(patch) => void save(patch)}
        />
      ) : null}
    </li>
  )
}

function OutcomeBadge({ candidate }: { candidate: IntakeCandidate }) {
  const { t } = useTranslation()
  switch (candidate.outcome) {
    case "created":
      return <Badge variant="secondary">{t("commodities:intake.candidate.created")}</Badge>
    case "skipped":
      return <Badge variant="outline">{t("commodities:intake.candidate.skipped")}</Badge>
    case "failed":
      return <Badge variant="destructive">{t("commodities:intake.candidate.failed")}</Badge>
    default:
      return null
  }
}

interface FieldListProps {
  candidate: IntakeCandidate
  // The custom fields to show, each with a value the scan read.
  customFields: readonly CustomField[]
  editable: boolean
  onChange: (patch: { acceptedFields?: string[]; acceptedCustomFields?: string[] }) => void
}

// FieldList shows what the scan read for one product: the item fields, then
// the group's custom fields. A checked field goes into the draft; an
// unchecked one stays unknown there.
function FieldList({ candidate, customFields, editable, onChange }: FieldListProps) {
  const { t } = useTranslation()
  const label = useIntakeFieldLabel()
  const itemTypes = useItemTypeLookup()
  const fields = INTAKE_REVIEW_FIELDS.filter((f) => candidate.fields[f] !== undefined)
  const accepted = new Set(candidate.acceptedFields)
  const acceptedCustom = new Set(candidate.acceptedCustomFields)

  function toggle(field: IntakeReviewField, on: boolean) {
    const next = new Set(accepted)
    if (on) next.add(field)
    else next.delete(field)
    onChange({ acceptedFields: INTAKE_REVIEW_FIELDS.filter((f) => next.has(f)) })
  }

  function toggleCustom(id: string, on: boolean) {
    const next = new Set(acceptedCustom)
    if (on) next.add(id)
    else next.delete(id)
    onChange({ acceptedCustomFields: customFields.map((f) => f.id).filter((f) => next.has(f)) })
  }

  function display(field: IntakeReviewField): string {
    const value = candidate.fields[field]?.value
    if (value === null || value === undefined || value === "") return "—"
    if (Array.isArray(value)) return value.map(String).join(", ")
    if (field === "type" && typeof value === "string") {
      return itemTypes.get(value)?.label ?? value
    }
    return String(value)
  }

  if (fields.length === 0 && customFields.length === 0) {
    return (
      <p className="pl-7 text-xs text-muted-foreground">
        {t("commodities:intake.candidate.noFields")}
      </p>
    )
  }

  return (
    <ul className="flex flex-col gap-1.5 pl-7" data-testid={`intake-fields-${candidate.id}`}>
      {fields.map((field) => (
        <GuessRow
          key={field}
          id={`intake-field-${candidate.id}-${field}`}
          label={label(field)}
          text={display(field)}
          confidence={candidate.fields[field]?.confidence ?? 0}
          checked={accepted.has(field)}
          editable={editable}
          onToggle={(on) => toggle(field, on)}
        />
      ))}
      {customFields.map((def) => {
        const guess = candidate.customFields[def.id]
        return (
          <GuessRow
            key={def.id}
            id={`intake-custom-field-${candidate.id}-${def.id}`}
            label={def.name}
            text={customFieldGuessText(def, guess?.value, t)}
            confidence={guess?.confidence ?? 0}
            checked={acceptedCustom.has(def.id)}
            editable={editable}
            onToggle={(on) => toggleCustom(def.id, on)}
          />
        )
      })}
    </ul>
  )
}

interface GuessRowProps {
  // The checkbox's id and test id.
  id: string
  label: string
  text: string
  // Between 0 and 1.
  confidence: number
  checked: boolean
  editable: boolean
  onToggle: (on: boolean) => void
}

function GuessRow({ id, label, text, confidence, checked, editable, onToggle }: GuessRowProps) {
  const { t } = useTranslation()
  return (
    <li className="flex min-w-0 items-start gap-2 text-sm">
      <Checkbox
        id={id}
        className="mt-0.5"
        checked={checked}
        disabled={!editable}
        onCheckedChange={(v) => onToggle(v === true)}
        data-testid={id}
      />
      <label htmlFor={id} className="min-w-0 flex-1">
        <span className="text-muted-foreground">{label}: </span>
        <span className="break-words">{text}</span>
      </label>
      <span className="shrink-0 text-xs text-muted-foreground">
        {t("commodities:intake.candidate.confidence", { value: Math.round(confidence * 100) })}
      </span>
    </li>
  )
}
