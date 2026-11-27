import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { CheckCircle2, Loader2, ScanText, Trash2 } from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import type { IntakeSession } from "@/features/intake/api"
import { useDeleteIntakeSession, useIntakeSessions } from "@/features/intake/hooks"
import { useIntakeSummaryText } from "@/features/intake/summary"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useGroupPermissions } from "@/features/group/roles"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatRelative } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"

// How many batches the card lists before "Show all".
const COLLAPSED_COUNT = 5
// The most the list endpoint returns in one page.
const MAX_PAGE = 100

// IntakeCard lists the group's batches of dropped files (#2810) above the
// Items list: what is still being scanned, what waits for a review, and what
// came of the finished ones. It renders nothing while the group has none.
export function IntakeCard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const toast = useAppToast()
  const confirm = useConfirm()
  const summaryText = useIntakeSummaryText()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const canWrite = useGroupPermissions().canEditContent
  const [expanded, setExpanded] = useState(false)
  const sessions = useIntakeSessions({ perPage: expanded ? MAX_PAGE : COLLAPSED_COUNT })
  const remove = useDeleteIntakeSession()

  const total = sessions.data?.total ?? 0
  if (!slug || total === 0) return null
  const rows = sessions.data?.sessions ?? []

  function open(session: IntakeSession) {
    navigate(`/g/${encodeURIComponent(slug)}/intake/${encodeURIComponent(session.id)}`)
  }

  async function handleRemove(session: IntakeSession) {
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
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:intake.removeFailed")))
    }
  }

  return (
    <Card className="gap-3 py-4" data-testid="intake-card">
      <CardHeader className="px-4">
        <CardTitle className="flex items-center gap-2 text-base">
          <ScanText className="size-4 text-muted-foreground" aria-hidden="true" />
          {t("commodities:intake.heading")}
          <Badge variant="secondary" data-testid="intake-count">
            {total}
          </Badge>
        </CardTitle>
        <CardDescription>{t("commodities:intake.description")}</CardDescription>
        {total > COLLAPSED_COUNT ? (
          <CardAction>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => setExpanded((v) => !v)}
              data-testid="intake-toggle"
            >
              {expanded
                ? t("commodities:intake.showFewer")
                : t("commodities:intake.showAll", { count: total })}
            </Button>
          </CardAction>
        ) : null}
      </CardHeader>
      <CardContent className="px-4">
        <ul className="divide-y divide-border" data-testid="intake-list">
          {rows.map((session) => {
            const added = formatRelative(session.createdAt)
            const summary = summaryText(session.summary)
            return (
              <li
                key={session.id}
                className="flex min-w-0 items-center gap-3 py-2"
                data-testid={`intake-row-${session.id}`}
              >
                <StateIcon state={session.state} />
                <div className="min-w-0 flex-1">
                  <p className="flex flex-wrap items-center gap-2 text-sm font-medium">
                    <span>
                      {t("commodities:intake.fileCount", { count: session.summary.files })}
                    </span>
                    <StateBadge state={session.state} />
                  </p>
                  <p className="truncate text-xs text-muted-foreground">
                    {added ? t("commodities:intake.addedAgo", { when: added }) : null}
                    {added && summary ? " · " : null}
                    <span data-testid={`intake-summary-${session.id}`}>{summary}</span>
                  </p>
                </div>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => open(session)}
                  data-testid={`intake-open-${session.id}`}
                >
                  {session.state === "review" && canWrite
                    ? t("commodities:intake.review")
                    : t("commodities:intake.open")}
                </Button>
                {canWrite && session.state !== "processing" ? (
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="size-8 text-muted-foreground hover:text-destructive"
                    onClick={() => void handleRemove(session)}
                    disabled={remove.isPending}
                    aria-label={t("commodities:intake.removeLabel")}
                    data-testid={`intake-remove-${session.id}`}
                  >
                    <Trash2 className="size-4" aria-hidden="true" />
                  </Button>
                ) : null}
              </li>
            )
          })}
        </ul>
      </CardContent>
    </Card>
  )
}

function StateIcon({ state }: { state: IntakeSession["state"] }) {
  if (state === "processing") {
    return (
      <Loader2 className="size-4 shrink-0 animate-spin text-muted-foreground" aria-hidden="true" />
    )
  }
  if (state === "done") {
    return <CheckCircle2 className="size-4 shrink-0 text-status-active" aria-hidden="true" />
  }
  return <ScanText className="size-4 shrink-0 text-primary" aria-hidden="true" />
}

export function StateBadge({ state }: { state: IntakeSession["state"] }) {
  const { t } = useTranslation()
  switch (state) {
    case "processing":
      return <Badge variant="outline">{t("commodities:intake.state.processing")}</Badge>
    case "review":
      return <Badge variant="default">{t("commodities:intake.state.review")}</Badge>
    default:
      return <Badge variant="secondary">{t("commodities:intake.state.done")}</Badge>
  }
}
