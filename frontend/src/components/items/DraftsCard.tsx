import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"
import { FilePen, Paperclip, Trash2 } from "lucide-react"

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
import type { Draft } from "@/features/drafts/api"
import { useDiscardDraft, useDrafts } from "@/features/drafts/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useMyGroupRole } from "@/features/group/roles"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatRelative } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"

// How many drafts the card lists before "Show all".
const COLLAPSED_COUNT = 5
// The most the list endpoint returns in one page.
const MAX_PAGE = 100

// DraftsCard lists the group's server drafts (#2812) above the Items list,
// so an item saved unfinished on one device can be found and finished on
// another. It renders nothing while the group has no drafts.
export function DraftsCard() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const toast = useAppToast()
  const confirm = useConfirm()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const role = useMyGroupRole()
  // Members who may edit content; nobody while the role is still loading.
  const canWrite = role === "user" || role === "admin" || role === "owner"
  const [expanded, setExpanded] = useState(false)
  const drafts = useDrafts({ perPage: expanded ? MAX_PAGE : COLLAPSED_COUNT })
  const discard = useDiscardDraft()

  const total = drafts.data?.total ?? 0
  if (!slug || total === 0) return null
  const rows = drafts.data?.drafts ?? []

  function resume(draft: Draft) {
    navigate(`/g/${encodeURIComponent(slug)}/commodities/new?draft=${encodeURIComponent(draft.id)}`)
  }

  async function handleDiscard(draft: Draft) {
    const title = draft.title || t("commodities:drafts.untitled")
    const ok = await confirm({
      title: t("commodities:drafts.discardTitle", { title }),
      description: t("commodities:drafts.discardDescription"),
      confirmLabel: t("commodities:drafts.discard"),
      destructive: true,
    })
    if (!ok) return
    try {
      await discard.mutateAsync(draft.id)
      toast.success(t("commodities:drafts.discarded", { title }))
    } catch (err) {
      toast.error(parseServerError(err, t("commodities:drafts.discardFailed")))
    }
  }

  return (
    <Card className="gap-3 py-4" data-testid="drafts-card">
      <CardHeader className="px-4">
        <CardTitle className="flex items-center gap-2 text-base">
          <FilePen className="size-4 text-muted-foreground" aria-hidden="true" />
          {t("commodities:drafts.heading")}
          <Badge variant="secondary" data-testid="drafts-count">
            {total}
          </Badge>
        </CardTitle>
        <CardDescription>{t("commodities:drafts.description")}</CardDescription>
        {total > COLLAPSED_COUNT ? (
          <CardAction>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => setExpanded((v) => !v)}
              data-testid="drafts-toggle"
            >
              {expanded
                ? t("commodities:drafts.showFewer")
                : t("commodities:drafts.showAll", { count: total })}
            </Button>
          </CardAction>
        ) : null}
      </CardHeader>
      <CardContent className="px-4">
        <ul className="divide-y divide-border" data-testid="drafts-list">
          {rows.map((draft) => {
            const title = draft.title || t("commodities:drafts.untitled")
            const saved = formatRelative(draft.updatedAt)
            return (
              <li
                key={draft.id}
                className="flex min-w-0 items-center gap-3 py-2"
                data-testid={`draft-row-${draft.id}`}
              >
                <div className="min-w-0 flex-1">
                  <p
                    className={
                      draft.title
                        ? "truncate text-sm font-medium"
                        : "truncate text-sm italic text-muted-foreground"
                    }
                  >
                    {title}
                  </p>
                  <p className="flex items-center gap-2 text-xs text-muted-foreground">
                    {saved ? (
                      <span>{t("commodities:drafts.savedAgo", { when: saved })}</span>
                    ) : null}
                    {draft.fileCount > 0 ? (
                      <span className="inline-flex items-center gap-1">
                        <Paperclip className="size-3" aria-hidden="true" />
                        {t("commodities:drafts.fileCount", { count: draft.fileCount })}
                      </span>
                    ) : null}
                  </p>
                </div>
                {canWrite ? (
                  <>
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => resume(draft)}
                      data-testid={`draft-resume-${draft.id}`}
                    >
                      {t("commodities:drafts.resume")}
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      className="size-8 text-muted-foreground hover:text-destructive"
                      onClick={() => void handleDiscard(draft)}
                      disabled={discard.isPending}
                      aria-label={t("commodities:drafts.discardLabel", { title })}
                      data-testid={`draft-discard-${draft.id}`}
                    >
                      <Trash2 className="size-4" aria-hidden="true" />
                    </Button>
                  </>
                ) : null}
              </li>
            )
          })}
        </ul>
      </CardContent>
    </Card>
  )
}
