import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { ArrowLeft, KeyRound, Plus } from "lucide-react"

import { CreateAPITokenDialog } from "@/components/settings/CreateAPITokenDialog"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Page, PageHeader } from "@/components/ui/page"
import { useOptionalImpersonation } from "@/features/admin/impersonation/ImpersonationContext"
import type { APITokenView } from "@/features/api-tokens/api"
import { useAPITokensList, useRevokeAPIToken } from "@/features/api-tokens/hooks"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useGroups } from "@/features/group/hooks"
import { useAppToast } from "@/hooks/useAppToast"
import { withGroupQuery } from "@/lib/group-aware-url"
import { formatDate, formatDateTime, formatRelative } from "@/lib/intl"

// ApiTokensPage renders /profile/api-tokens (#2808): the user's personal API
// tokens with their scope, group, expiry and last use, a create flow that
// shows each new token once, and revoke with confirmation. Same page shape
// as SessionsPage.
export function ApiTokensPage() {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const toast = useAppToast()
  const tokensQuery = useAPITokensList()
  const revokeMutation = useRevokeAPIToken()
  const { data: groups } = useGroups()
  // The server refuses to create or revoke tokens during impersonation; the
  // page says so up front instead of letting the request fail.
  const impersonating = useOptionalImpersonation()?.active ?? false

  const [createOpen, setCreateOpen] = useState(false)
  const [pendingRevoke, setPendingRevoke] = useState<APITokenView | null>(null)

  const tokens = useMemo(() => tokensQuery.data?.tokens ?? [], [tokensQuery.data?.tokens])
  const groupNames = useMemo(() => {
    const names = new Map<string, string>()
    for (const g of groups ?? []) {
      if (g.id) names.set(g.id, g.name ?? g.id)
    }
    return names
  }, [groups])

  const doRevoke = () => {
    if (!pendingRevoke?.id) return
    revokeMutation.mutate(pendingRevoke.id, {
      onSuccess: () => {
        toast.success(t("settings:apiTokens.toasts.revoked"))
        setPendingRevoke(null)
      },
      onError: () => {
        toast.error(t("settings:apiTokens.toasts.revokeFailed"))
      },
    })
  }

  return (
    <>
      <RouteTitle title={t("settings:apiTokens.title")} />
      <Page width="narrow" data-testid="api-tokens-page">
        <PageHeader
          size="detail"
          title={t("settings:apiTokens.title")}
          subtitle={t("settings:apiTokens.subtitle")}
          backLink={
            <Link
              to={withGroupQuery("/settings", currentGroup?.slug)}
              className="inline-flex items-center gap-1.5 text-muted-foreground hover:text-foreground transition-colors"
            >
              <ArrowLeft className="size-4" aria-hidden="true" />
              {t("settings:privacy.title")}
            </Link>
          }
        />

        <div className="flex items-center justify-between gap-3 rounded-xl border border-border bg-card p-4">
          <div className="flex items-center gap-3">
            <div className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-primary/10">
              <KeyRound className="size-4 text-primary" aria-hidden="true" />
            </div>
            <div>
              <p className="text-sm font-medium">{t("settings:apiTokens.createCard.label")}</p>
              <p className="text-xs text-muted-foreground">
                {impersonating
                  ? t("settings:apiTokens.createCard.impersonating")
                  : t("settings:apiTokens.createCard.description")}
              </p>
            </div>
          </div>
          <Button
            size="sm"
            onClick={() => setCreateOpen(true)}
            disabled={impersonating}
            data-testid="api-tokens-create-btn"
            className="gap-1.5 shrink-0"
          >
            <Plus className="size-3.5" aria-hidden="true" />
            {t("settings:apiTokens.createCard.cta")}
          </Button>
        </div>

        {tokensQuery.isLoading ? (
          <div className="rounded-xl border border-border p-6 text-sm text-muted-foreground">
            {t("settings:apiTokens.loading")}
          </div>
        ) : tokensQuery.isError ? (
          <div className="rounded-xl border border-border p-6 text-sm text-destructive">
            {t("settings:apiTokens.loadFailed")}
          </div>
        ) : tokens.length === 0 ? (
          <div
            className="rounded-xl border border-border p-6 text-sm text-muted-foreground"
            data-testid="api-tokens-empty"
          >
            {t("settings:apiTokens.empty")}
          </div>
        ) : (
          <div className="space-y-3" data-testid="api-tokens-list">
            {tokens.map((token) => (
              <APITokenCard
                key={token.id}
                token={token}
                groupName={token.group_id ? (groupNames.get(token.group_id) ?? null) : null}
                onRevoke={setPendingRevoke}
                revokeDisabled={revokeMutation.isPending || impersonating}
              />
            ))}
          </div>
        )}
      </Page>

      <CreateAPITokenDialog open={createOpen} onOpenChange={setCreateOpen} />

      <Dialog open={!!pendingRevoke} onOpenChange={(o) => !o && setPendingRevoke(null)}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>{t("settings:apiTokens.confirmRevoke.title")}</DialogTitle>
            <DialogDescription>
              {t("settings:apiTokens.confirmRevoke.description", {
                name: pendingRevoke?.name ?? "",
              })}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter className="gap-2">
            <Button
              variant="outline"
              onClick={() => setPendingRevoke(null)}
              disabled={revokeMutation.isPending}
            >
              {t("common:actions.cancel")}
            </Button>
            <Button
              onClick={doRevoke}
              disabled={revokeMutation.isPending}
              variant="destructive"
              data-testid="api-tokens-confirm-revoke-btn"
            >
              {t("settings:apiTokens.confirmRevoke.cta")}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}

interface APITokenCardProps {
  token: APITokenView
  // null with a group_id set means the group is no longer visible to the
  // user (they left it); the token cannot reach it either.
  groupName: string | null
  onRevoke: (token: APITokenView) => void
  revokeDisabled: boolean
}

function APITokenCard({ token, groupName, onRevoke, revokeDisabled }: APITokenCardProps) {
  const { t } = useTranslation()
  const scope = token.scope ?? "read_only"
  const groupLabel = token.group_id
    ? (groupName ?? t("settings:apiTokens.card.unknownGroup"))
    : t("settings:apiTokens.card.allGroups")
  const lastUsed = token.last_used_at
    ? t("settings:apiTokens.card.lastUsed", { value: formatRelative(token.last_used_at) })
    : t("settings:apiTokens.card.neverUsed")
  const expiry = token.expires_at
    ? t(token.expired ? "settings:apiTokens.card.expiredOn" : "settings:apiTokens.card.expiresOn", {
        value: formatDate(token.expires_at),
      })
    : t("settings:apiTokens.card.noExpiry")

  return (
    <div
      className="rounded-xl border border-border bg-card p-4"
      data-testid="api-token-card"
      data-token-id={token.id}
    >
      <div className="flex items-start gap-3">
        <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-muted">
          <KeyRound className="size-5 text-muted-foreground" aria-hidden="true" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <p className="text-sm font-medium break-all">{token.name}</p>
            <Badge variant="secondary" className="text-xs" data-testid="api-token-scope-badge">
              {t(`settings:apiTokens.scopes.${scope}`)}
            </Badge>
            {token.expired ? (
              <Badge
                variant="destructive"
                className="text-xs"
                data-testid="api-token-expired-badge"
              >
                {t("settings:apiTokens.card.expired")}
              </Badge>
            ) : null}
          </div>
          <p className="mt-0.5 text-xs text-muted-foreground">
            <span className="font-mono">{token.token_prefix}…</span> · {groupLabel}
          </p>
          <p className="text-xs text-muted-foreground" data-testid="api-token-usage">
            {lastUsed} · {expiry}
          </p>
          <p className="text-xs text-muted-foreground" title={token.created_at ?? ""}>
            {t("settings:apiTokens.card.createdAt", {
              value: formatDateTime(token.created_at ?? ""),
            })}
          </p>
        </div>
        <Button
          variant="outline"
          size="sm"
          disabled={revokeDisabled || !token.id}
          onClick={() => onRevoke(token)}
          data-testid="api-token-revoke-btn"
        >
          {t("settings:apiTokens.revoke")}
        </Button>
      </div>
    </div>
  )
}
