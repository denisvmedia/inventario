import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import { Copy, Plus, Settings, Trash2, UserMinus } from "lucide-react"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { useAuth } from "@/features/auth/AuthContext"
import { useCurrentGroup } from "@/features/group/GroupContext"
import {
  useChangeMemberRole,
  useCreateInvite,
  useInvites,
  useMembers,
  useRemoveMember,
  useRevokeInvite,
} from "@/features/group/hooks"
import type { GroupRole } from "@/features/group/api"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatDate, formatDateTime } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { cn } from "@/lib/utils"

// Builds the invite URL the user copies. The /invite/:token route is
// public and lives at the SPA root, so an absolute URL with the current
// origin is the right share format.
function inviteUrl(token: string | undefined): string {
  if (!token) return ""
  return `${window.location.origin}/invite/${encodeURIComponent(token)}`
}

// Members list + invites for a group. The page lives at /g/:slug/members
// and is reached from the sidebar / group settings. Admin-only actions
// are gated by a client-side role check; the server is authoritative
// (rejects with 403 / 422), so the UI only hides controls a non-admin
// shouldn't see at all.
export function MembersPage() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const { currentGroup } = useCurrentGroup()
  const groupId = currentGroup?.id

  const membersQuery = useMembers(groupId)
  const myMembership = useMemo(
    () => membersQuery.data?.find((m) => m.member_user_id === user?.id),
    [membersQuery.data, user?.id]
  )
  const isAdmin = myMembership?.role === "admin"
  const adminCount = useMemo(
    () => membersQuery.data?.filter((m) => m.role === "admin").length ?? 0,
    [membersQuery.data]
  )

  // Invites only loaded for admins — non-admins get a 403 anyway. Saves
  // a fetch and avoids an error toast in the network panel.
  const invitesQuery = useInvites(groupId, { enabled: !!groupId && isAdmin })

  return (
    <>
      <RouteTitle title={t("members:title")} />
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-8" data-testid="members-page">
        <header className="flex items-start justify-between gap-3">
          <div>
            <h1 className="text-2xl font-semibold tracking-tight">{t("members:title")}</h1>
            <p className="text-sm text-muted-foreground">{t("members:subtitle")}</p>
          </div>
          {isAdmin && currentGroup?.id ? (
            <Button asChild variant="outline" size="sm" className="gap-1.5">
              <Link
                to={`/groups/${encodeURIComponent(currentGroup.id)}/settings`}
                data-testid="members-group-settings-link"
              >
                <Settings className="size-3.5" aria-hidden="true" />
                {t("groups:settings.title")}
              </Link>
            </Button>
          ) : null}
        </header>

        <MembersList
          members={membersQuery.data ?? []}
          isLoading={membersQuery.isLoading}
          isError={membersQuery.isError}
          isAdmin={!!isAdmin}
          adminCount={adminCount}
          currentUserId={user?.id}
          groupId={groupId}
        />

        {isAdmin ? (
          <>
            <Separator />
            <InvitesSection
              groupId={groupId}
              invites={invitesQuery.data ?? []}
              isLoading={invitesQuery.isLoading}
              isError={invitesQuery.isError}
            />
          </>
        ) : (
          <p className="text-xs text-muted-foreground">{t("members:adminOnlyHelp")}</p>
        )}
      </div>
    </>
  )
}

function MembersList({
  members,
  isLoading,
  isError,
  isAdmin,
  adminCount,
  currentUserId,
  groupId,
}: {
  members: Array<{ id?: string; member_user_id?: string; role?: GroupRole; joined_at?: string }>
  isLoading: boolean
  isError: boolean
  isAdmin: boolean
  adminCount: number
  currentUserId: string | undefined
  groupId: string | undefined
}) {
  const { t } = useTranslation()

  if (isLoading) {
    return <div className="text-sm text-muted-foreground">{t("members:title")}…</div>
  }
  if (isError) {
    return (
      <Alert variant="destructive" data-testid="members-load-error">
        <AlertDescription>{t("members:loadError")}</AlertDescription>
      </Alert>
    )
  }
  if (!members.length) {
    return (
      <p className="text-sm text-muted-foreground" data-testid="members-empty">
        {t("members:empty")}
      </p>
    )
  }
  return (
    <div
      className="rounded-xl border border-border divide-y divide-border"
      data-testid="members-list"
    >
      {members.map((m) => (
        <MemberRow
          key={m.id ?? m.member_user_id}
          member={m}
          isAdmin={isAdmin}
          isLastAdmin={adminCount === 1 && m.role === "admin"}
          isMe={!!currentUserId && m.member_user_id === currentUserId}
          groupId={groupId}
        />
      ))}
    </div>
  )
}

function MemberRow({
  member,
  isAdmin,
  isLastAdmin,
  isMe,
  groupId,
}: {
  member: { id?: string; member_user_id?: string; role?: GroupRole; joined_at?: string }
  isAdmin: boolean
  isLastAdmin: boolean
  isMe: boolean
  groupId: string | undefined
}) {
  const { t } = useTranslation()
  const confirm = useConfirm()
  const toast = useAppToast()
  const changeRoleMutation = useChangeMemberRole()
  const removeMutation = useRemoveMember()

  const memberUserId = member.member_user_id ?? ""
  // The BE returns memberships only — no user name/email yet (see #1413
  // notes; a `users:included` join is a backend follow-up). We surface
  // a short hash of the user id so admins can distinguish rows; it's
  // ugly but accurate. The "(you)" pill makes self-row obvious.
  const shortId = memberUserId ? memberUserId.slice(0, 8) : "?"
  const role = (member.role ?? "user") as GroupRole

  async function handleRoleChange(next: string) {
    if (!groupId || !memberUserId) return
    if (next === role) return
    try {
      await changeRoleMutation.mutateAsync({
        groupId,
        memberUserId,
        role: next as GroupRole,
      })
    } catch (err) {
      toast.error(parseServerError(err, t("members:loadError")))
    }
  }

  async function handleRemove() {
    if (!groupId || !memberUserId) return
    const ok = await confirm({
      title: t("members:removeConfirm.title", { name: t("members:memberLabel", { id: shortId }) }),
      description: t("members:removeConfirm.description"),
      confirmLabel: t("members:removeConfirm.confirm"),
      cancelLabel: t("members:removeConfirm.cancel"),
      destructive: true,
    })
    if (!ok) return
    try {
      await removeMutation.mutateAsync({ groupId, memberUserId })
    } catch (err) {
      toast.error(parseServerError(err, t("members:loadError")))
    }
  }

  return (
    <div
      className="flex items-center justify-between gap-3 px-4 py-3"
      data-testid={`member-row-${shortId}`}
    >
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <p className="text-sm font-medium truncate">
            {t("members:memberLabel", { id: shortId })}
          </p>
          {isMe ? (
            <Badge variant="secondary" className="text-[10px] uppercase">
              {t("members:you")}
            </Badge>
          ) : null}
          <Badge
            variant={role === "admin" ? "default" : "outline"}
            className="text-[10px] uppercase"
            data-testid={`member-role-${shortId}`}
          >
            {t(`members:roles.${role}`)}
          </Badge>
        </div>
        {member.joined_at ? (
          <p className="text-[11px] text-muted-foreground mt-0.5">
            {t("members:joinedOn", { date: formatDate(member.joined_at, { style: "medium" }) })}
          </p>
        ) : null}
      </div>
      {isAdmin ? (
        <div className="flex items-center gap-2 shrink-0">
          <select
            value={role}
            onChange={(e) => handleRoleChange(e.target.value)}
            disabled={changeRoleMutation.isPending || isLastAdmin}
            aria-label={t("members:actions.changeRole")}
            data-testid={`member-role-select-${shortId}`}
            className={cn(
              "h-8 rounded-md border border-input bg-background px-2.5 text-sm shadow-xs",
              "focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50",
              "disabled:cursor-not-allowed disabled:opacity-50"
            )}
          >
            <option value="admin">{t("members:roles.admin")}</option>
            <option value="user">{t("members:roles.user")}</option>
          </select>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className={cn(
              "gap-1.5 text-destructive border-destructive/40 hover:bg-destructive/10",
              "disabled:cursor-not-allowed disabled:opacity-50"
            )}
            disabled={isLastAdmin || removeMutation.isPending}
            title={isLastAdmin ? t("members:actions.removeLastAdmin") : undefined}
            onClick={handleRemove}
            data-testid={`member-remove-${shortId}`}
          >
            <UserMinus className="size-3.5" aria-hidden="true" />
            {removeMutation.isPending ? t("members:actions.removing") : t("members:actions.remove")}
          </Button>
        </div>
      ) : null}
    </div>
  )
}

function InvitesSection({
  groupId,
  invites,
  isLoading,
  isError,
}: {
  groupId: string | undefined
  invites: Array<{ id?: string; token?: string; expires_at?: string }>
  isLoading: boolean
  isError: boolean
}) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const createMutation = useCreateInvite()
  const revokeMutation = useRevokeInvite()
  const [latestInvite, setLatestInvite] = useState<{ token?: string } | null>(null)

  async function handleCreate() {
    if (!groupId) return
    try {
      const created = await createMutation.mutateAsync({ groupId })
      setLatestInvite(created)
    } catch (err) {
      toast.error(parseServerError(err, t("members:invites.createError")))
    }
  }

  async function handleCopy(token: string | undefined) {
    if (!token) return
    const url = inviteUrl(token)
    try {
      await navigator.clipboard.writeText(url)
      toast.success(t("members:invites.copied"))
    } catch {
      toast.error(t("members:invites.copyFailed"))
    }
  }

  async function handleRevoke(inviteId: string | undefined) {
    if (!groupId || !inviteId) return
    try {
      await revokeMutation.mutateAsync({ groupId, inviteId })
    } catch (err) {
      toast.error(parseServerError(err, t("members:invites.revokeError")))
    }
  }

  return (
    <section className="space-y-4" data-testid="invites-section">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-base font-semibold">{t("members:invites.title")}</h2>
          <p className="text-xs text-muted-foreground mt-0.5">{t("members:invites.subtitle")}</p>
        </div>
        <Button
          type="button"
          size="sm"
          className="gap-1.5"
          disabled={createMutation.isPending}
          onClick={handleCreate}
          data-testid="invite-create"
        >
          <Plus className="size-3.5" aria-hidden="true" />
          {createMutation.isPending
            ? t("members:invites.generating")
            : t("members:invites.generate")}
        </Button>
      </div>

      {latestInvite?.token ? (
        <div
          className="rounded-lg border border-primary/40 bg-primary/5 p-3 space-y-2"
          data-testid="invite-latest"
        >
          <p className="text-xs font-medium">{t("members:invites.tokenLabel")}</p>
          <div className="flex items-center gap-2">
            <code
              className="flex-1 truncate rounded-md bg-background px-2 py-1.5 text-xs font-mono"
              data-testid="invite-latest-url"
            >
              {inviteUrl(latestInvite.token)}
            </code>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="gap-1.5"
              onClick={() => handleCopy(latestInvite.token)}
              data-testid="invite-latest-copy"
            >
              <Copy className="size-3.5" aria-hidden="true" />
              {t("members:invites.copy")}
            </Button>
          </div>
        </div>
      ) : null}

      {isLoading ? (
        <p className="text-sm text-muted-foreground">{t("members:invites.title")}…</p>
      ) : isError ? (
        <Alert variant="destructive" data-testid="invites-load-error">
          <AlertDescription>{t("members:loadError")}</AlertDescription>
        </Alert>
      ) : invites.length === 0 ? (
        <p className="text-xs text-muted-foreground" data-testid="invites-empty">
          {t("members:invites.empty")}
        </p>
      ) : (
        <div className="rounded-xl border border-border divide-y divide-border">
          {invites.map((inv) => {
            const tokenShort = inv.token ? inv.token.slice(0, 12) : "?"
            return (
              <div
                key={inv.id ?? inv.token}
                className="flex items-center justify-between gap-3 px-4 py-3"
                data-testid={`invite-row-${tokenShort}`}
              >
                <div className="min-w-0">
                  <code className="text-xs font-mono text-muted-foreground">{tokenShort}…</code>
                  {inv.expires_at ? (
                    <p className="text-[11px] text-muted-foreground mt-0.5">
                      {t("members:invites.expiresAt", {
                        date: formatDateTime(inv.expires_at),
                      })}
                    </p>
                  ) : null}
                </div>
                <div className="flex items-center gap-2 shrink-0">
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="gap-1.5"
                    onClick={() => handleCopy(inv.token)}
                    data-testid={`invite-copy-${tokenShort}`}
                  >
                    <Copy className="size-3.5" aria-hidden="true" />
                    {t("members:invites.copy")}
                  </Button>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    className="gap-1.5 text-destructive border-destructive/40 hover:bg-destructive/10"
                    disabled={revokeMutation.isPending}
                    onClick={() => handleRevoke(inv.id)}
                    data-testid={`invite-revoke-${tokenShort}`}
                  >
                    <Trash2 className="size-3.5" aria-hidden="true" />
                    {revokeMutation.isPending
                      ? t("members:invites.revoking")
                      : t("members:invites.revoke")}
                  </Button>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </section>
  )
}
