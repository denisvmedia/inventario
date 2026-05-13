import { useEffect, useMemo, useRef, useState, type ElementType } from "react"
import { useTranslation } from "react-i18next"
import {
  Check,
  Clock,
  Copy,
  Crown,
  Eye,
  Mail,
  MoreHorizontal,
  Shield,
  Trash2,
  User as UserIcon,
  UserPlus,
  Users,
} from "lucide-react"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { useAuth } from "@/features/auth/AuthContext"
import { useCurrentGroup } from "@/features/group/GroupContext"
import {
  useChangeMemberRole,
  useCreateInvite,
  useInvites,
  useMembers,
  useRemoveMember,
  useResendInvite,
  useRevokeInvite,
} from "@/features/group/hooks"
import type { GroupRole, MemberRow } from "@/features/group/api"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { formatDate } from "@/lib/intl"
import { parseServerError } from "@/lib/server-error"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { cn } from "@/lib/utils"

// Role configuration mirrors design-mocks/src/views/MembersView.tsx
// (ROLE_CONFIG / ROLE_ORDER). The label / description text comes from
// i18n; the icon + badge color stays here because they're visual
// constants of the role taxonomy, not user-tweakable copy.
const ROLE_ORDER: GroupRole[] = ["viewer", "user", "admin", "owner"]
const ROLE_ICON: Record<GroupRole, ElementType> = {
  viewer: Eye,
  user: UserIcon,
  admin: Shield,
  owner: Crown,
}
// Subtle tinting per role — viewer / user stay neutral / cool; admin
// uses the primary brand color; owner uses the amber accent. Kept as
// Tailwind utility strings so the values flow through the design-token
// system rather than being raw hex codes.
const ROLE_BADGE_CLASS: Record<GroupRole, string> = {
  viewer: "bg-muted text-muted-foreground border-0",
  user: "bg-chart-3/10 text-chart-3 border-0",
  admin: "bg-primary/10 text-primary border-0",
  owner: "bg-accent text-accent-foreground border-0",
}

// Roles offered in the Invite dialog. Owner is intentionally absent —
// owner is a transfer-of-ownership action, not an invite role. The BE
// rejects role=owner on POST /invites with 422; the UI mirrors that.
const INVITE_ROLE_OPTIONS: GroupRole[] = ["viewer", "user", "admin"]

function inviteUrl(token: string | undefined): string {
  if (!token) return ""
  return `${window.location.origin}/invite/${encodeURIComponent(token)}`
}

function initialsFor(name: string | undefined, fallback: string): string {
  const trimmed = (name ?? "").trim()
  if (!trimmed) return fallback.slice(0, 2).toUpperCase()
  const parts = trimmed.split(/\s+/).slice(0, 2)
  return parts.map((p) => p[0]?.toUpperCase() ?? "").join("") || fallback.slice(0, 2).toUpperCase()
}

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
  const myRole = (myMembership?.role ?? null) as GroupRole | null
  const canManageMembers = myRole === "admin" || myRole === "owner"
  const isOwner = myRole === "owner"

  const ownerCount = useMemo(
    () => membersQuery.data?.filter((m) => m.role === "owner").length ?? 0,
    [membersQuery.data]
  )
  const adminOrOwnerCount = useMemo(
    () => membersQuery.data?.filter((m) => m.role === "admin" || m.role === "owner").length ?? 0,
    [membersQuery.data]
  )

  const invitesQuery = useInvites(groupId, { enabled: !!groupId && canManageMembers })

  const [inviteDialogOpen, setInviteDialogOpen] = useState(false)

  return (
    <>
      <RouteTitle title={t("members:title")} />
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-8 p-6" data-testid="members-page">
        <header className="flex items-start justify-between gap-3">
          <div>
            <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">
              {t("members:title")}
            </h1>
            <p className="text-sm text-muted-foreground">
              {t("members:subtitle")}
              {currentGroup?.name ? (
                <>
                  {" "}
                  <span className="font-medium text-foreground">{currentGroup.name}</span>
                </>
              ) : null}
            </p>
          </div>
          <div className="flex items-center gap-2 shrink-0">
            {/* #1660: the Group-settings shortcut button was dropped —
                the same destination is reachable from the sidebar's
                Manage › Group settings row and the GroupSelector's
                shortcut, so duplicating it here added noise without
                reachability. Invite stays as the sole header CTA, in
                parity with design-mocks/src/views/MembersView.tsx. */}
            {canManageMembers ? (
              <Button
                type="button"
                size="sm"
                className="gap-1.5"
                onClick={() => setInviteDialogOpen(true)}
                data-testid="members-invite-cta"
              >
                <UserPlus className="size-3.5" aria-hidden="true" />
                {t("members:invite.cta")}
              </Button>
            ) : null}
          </div>
        </header>

        <StatsRow
          memberCount={membersQuery.data?.length ?? 0}
          adminCount={adminOrOwnerCount}
          // For non-managing viewers the invites endpoint is gated
          // (useInvites is disabled), so we have no count to surface.
          // Show null → "—" instead of "0", which would falsely claim
          // there are zero pending invites when we genuinely don't know.
          pendingCount={canManageMembers ? (invitesQuery.data?.length ?? 0) : null}
        />

        <RoleLegend />

        <MembersList
          members={membersQuery.data ?? []}
          isLoading={membersQuery.isLoading}
          isError={membersQuery.isError}
          canManage={canManageMembers}
          isOwner={isOwner}
          ownerCount={ownerCount}
          currentUserId={user?.id}
          groupId={groupId}
        />

        {canManageMembers ? (
          <>
            <Separator />
            <PendingInvitesSection
              groupId={groupId}
              invites={invitesQuery.data ?? []}
              isLoading={invitesQuery.isLoading}
              isError={invitesQuery.isError}
            />
          </>
        ) : (
          <p className="text-xs text-muted-foreground">{t("members:adminOnlyHelp")}</p>
        )}

        {canManageMembers && currentGroup?.id ? (
          <InviteDialog
            open={inviteDialogOpen}
            onOpenChange={setInviteDialogOpen}
            groupId={currentGroup.id}
            groupName={currentGroup.name ?? ""}
          />
        ) : null}
      </div>
    </>
  )
}

// ─── Stats tiles ─────────────────────────────────────────────────────────────

function StatsRow({
  memberCount,
  adminCount,
  pendingCount,
}: {
  memberCount: number
  adminCount: number
  // `null` means "the caller doesn't have access to the invites list"
  // (non-managing viewer). The tile renders an em-dash placeholder
  // rather than "0" so we don't falsely claim there are zero pending
  // invites when the data simply isn't fetched.
  pendingCount: number | null
}) {
  const { t } = useTranslation()
  const tiles: Array<{
    label: string
    value: number | string
    icon: ElementType
    testId: string
  }> = [
    { label: t("members:stats.members"), value: memberCount, icon: Users, testId: "stat-members" },
    { label: t("members:stats.admins"), value: adminCount, icon: Crown, testId: "stat-admins" },
    {
      label: t("members:stats.pending"),
      value: pendingCount === null ? "—" : pendingCount,
      icon: Clock,
      testId: "stat-pending",
    },
  ]
  return (
    <div className="grid grid-cols-3 gap-3" data-testid="members-stats">
      {tiles.map((s) => (
        <div
          key={s.label}
          className="flex items-center gap-3 rounded-xl border border-border bg-card px-4 py-3"
          data-testid={s.testId}
        >
          <div className="flex size-8 items-center justify-center rounded-lg bg-muted shrink-0">
            <s.icon className="size-4 text-muted-foreground" aria-hidden="true" />
          </div>
          <div>
            <p className="text-sm font-semibold">{s.value}</p>
            <p className="text-xs text-muted-foreground">{s.label}</p>
          </div>
        </div>
      ))}
    </div>
  )
}

// ─── Role legend card ────────────────────────────────────────────────────────

function RoleLegend() {
  const { t } = useTranslation()
  return (
    <div
      className="rounded-xl border border-border bg-card divide-y divide-border"
      data-testid="role-legend"
    >
      <div className="px-4 py-3">
        <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">
          {t("members:legend.title")}
        </p>
      </div>
      {ROLE_ORDER.map((role) => {
        const Icon = ROLE_ICON[role]
        return (
          <div key={role} className="flex items-center gap-3 px-4 py-3">
            <div className="flex size-7 items-center justify-center rounded-md bg-muted shrink-0">
              <Icon className="size-3.5 text-muted-foreground" aria-hidden="true" />
            </div>
            <div className="flex-1 min-w-0">
              <p className="text-sm font-medium">{t(`members:roles.${role}`)}</p>
              <p className="text-xs text-muted-foreground">
                {t(`members:roleDescriptions.${role}`)}
              </p>
            </div>
            <RoleBadge role={role} />
          </div>
        )
      })}
    </div>
  )
}

function RoleBadge({ role }: { role: GroupRole }) {
  const { t } = useTranslation()
  const Icon = ROLE_ICON[role]
  return (
    <Badge
      variant="secondary"
      className={cn("gap-1 h-5 text-[10px]", ROLE_BADGE_CLASS[role])}
      data-testid={`role-badge-${role}`}
    >
      <Icon className="size-3" aria-hidden="true" />
      {t(`members:roles.${role}`)}
    </Badge>
  )
}

// ─── Members list ────────────────────────────────────────────────────────────

function MembersList({
  members,
  isLoading,
  isError,
  canManage,
  isOwner,
  ownerCount,
  currentUserId,
  groupId,
}: {
  members: MemberRow[]
  isLoading: boolean
  isError: boolean
  canManage: boolean
  isOwner: boolean
  ownerCount: number
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
      className="rounded-xl border border-border overflow-hidden bg-card"
      data-testid="members-list"
    >
      {members.map((m, i) => (
        <div key={m.id ?? m.member_user_id}>
          {i > 0 ? <Separator /> : null}
          <MemberRowView
            member={m}
            canManage={canManage}
            isOwner={isOwner}
            isLastOwner={ownerCount === 1 && m.role === "owner"}
            isMe={!!currentUserId && m.member_user_id === currentUserId}
            groupId={groupId}
          />
        </div>
      ))}
    </div>
  )
}

function MemberRowView({
  member,
  canManage,
  isOwner,
  isLastOwner,
  isMe,
  groupId,
}: {
  member: MemberRow
  canManage: boolean
  isOwner: boolean
  isLastOwner: boolean
  isMe: boolean
  groupId: string | undefined
}) {
  const { t } = useTranslation()
  const confirm = useConfirm()
  const toast = useAppToast()
  const changeRoleMutation = useChangeMemberRole()
  const removeMutation = useRemoveMember()

  const role = (member.role ?? "user") as GroupRole
  const memberUserId = member.member_user_id ?? ""
  const displayName = member.user?.name?.trim() || (memberUserId ? memberUserId.slice(0, 8) : "?")
  const email = member.user?.email ?? ""
  // Demoting an owner requires being an owner yourself — admins can
  // manage everyone except owner rows. Self-row hides destructive
  // actions to avoid the foot-gun of removing oneself; "Leave group"
  // belongs on the group-level surface.
  const rowEditable = canManage && !isMe && (role !== "owner" || isOwner)
  // Last-owner guard: disable the role select / remove button so the
  // user doesn't trip the server-side ErrLastOwner.
  const disableMutation = isLastOwner

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
      title: t("members:removeConfirm.title", { name: displayName }),
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

  const shortId = memberUserId ? memberUserId.slice(0, 8) : "x"

  return (
    <div className="flex items-center gap-3 px-4 py-3" data-testid={`member-row-${shortId}`}>
      <div
        className={cn(
          "flex size-8 items-center justify-center rounded-full text-xs font-semibold shrink-0",
          isMe ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground"
        )}
        aria-hidden="true"
      >
        {initialsFor(displayName, memberUserId)}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1.5">
          <p className="text-sm font-medium truncate" data-testid={`member-name-${shortId}`}>
            {displayName}
          </p>
          {isMe ? <span className="text-xs text-muted-foreground">{t("members:you")}</span> : null}
        </div>
        {email ? (
          <p
            className="text-xs text-muted-foreground truncate"
            data-testid={`member-email-${shortId}`}
          >
            {email}
          </p>
        ) : null}
      </div>
      <RoleBadge role={role} />
      {member.joined_at ? (
        <p className="text-xs text-muted-foreground hidden sm:block">
          {t("members:joinedSince", {
            date: formatDate(member.joined_at, { style: "medium" }),
          })}
        </p>
      ) : null}
      {rowEditable ? (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button
              variant="ghost"
              size="icon"
              className="size-7"
              aria-label={t("members:actions.changeRole")}
              data-testid={`member-actions-${shortId}`}
            >
              <MoreHorizontal className="size-4" aria-hidden="true" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end" className="w-56">
            <div className="px-2 py-1.5 text-xs font-semibold uppercase tracking-widest text-muted-foreground">
              {t("members:actions.changeRole")}
            </div>
            {ROLE_ORDER.filter((r) => isOwner || r !== "owner").map((r) => {
              const Icon = ROLE_ICON[r]
              return (
                <DropdownMenuItem
                  key={r}
                  onSelect={(e) => {
                    e.preventDefault()
                    void handleRoleChange(r)
                  }}
                  disabled={disableMutation || changeRoleMutation.isPending || r === role}
                  data-testid={`member-role-${shortId}-${r}`}
                >
                  <Icon className="size-4 mr-2" aria-hidden="true" />
                  {t(`members:roles.${r}`)}
                  {r === role ? <Check className="size-3.5 ml-auto" aria-hidden="true" /> : null}
                </DropdownMenuItem>
              )
            })}
            <DropdownMenuSeparator />
            <DropdownMenuItem
              className="text-destructive focus:text-destructive"
              disabled={disableMutation || removeMutation.isPending}
              onSelect={(e) => {
                e.preventDefault()
                void handleRemove()
              }}
              data-testid={`remove-member-btn-${memberUserId}`}
            >
              <Trash2 className="size-4 mr-2" aria-hidden="true" />
              {removeMutation.isPending
                ? t("members:actions.removing")
                : t("members:actions.remove")}
            </DropdownMenuItem>
            {disableMutation ? (
              <p className="px-2 py-1.5 text-[11px] text-muted-foreground">
                {t("members:actions.removeLastOwner")}
              </p>
            ) : null}
          </DropdownMenuContent>
        </DropdownMenu>
      ) : null}
    </div>
  )
}

// ─── Pending invites section ────────────────────────────────────────────────

function PendingInvitesSection({
  groupId,
  invites,
  isLoading,
  isError,
}: {
  groupId: string | undefined
  invites: Array<{
    id?: string
    token?: string
    created_at?: string
    expires_at?: string
    invitee_email?: string | null
    role?: GroupRole
  }>
  isLoading: boolean
  isError: boolean
}) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const confirm = useConfirm()
  const resendMutation = useResendInvite()
  const revokeMutation = useRevokeInvite()

  async function handleResend(inviteId: string | undefined) {
    if (!groupId || !inviteId) return
    try {
      await resendMutation.mutateAsync({ groupId, inviteId })
      toast.success(t("members:invite.resendSuccess"))
    } catch (err) {
      toast.error(parseServerError(err, t("members:invite.resendError")))
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

  async function handleRevoke(invite: { id?: string; invitee_email?: string | null }) {
    if (!groupId || !invite.id) return
    const ok = await confirm({
      title: t("members:invite.revokeConfirm.title"),
      description: t("members:invite.revokeConfirm.description", {
        email: invite.invitee_email ?? "",
      }),
      confirmLabel: t("members:invite.revokeConfirm.confirm"),
      cancelLabel: t("members:invite.revokeConfirm.cancel"),
      destructive: true,
    })
    if (!ok) return
    try {
      await revokeMutation.mutateAsync({ groupId, inviteId: invite.id })
    } catch (err) {
      toast.error(parseServerError(err, t("members:invites.revokeError")))
    }
  }

  return (
    <section className="space-y-3" data-testid="invites-section">
      <h2 className="text-base font-semibold">{t("members:invites.title")}</h2>
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
        <div className="rounded-xl border border-border overflow-hidden bg-card">
          {invites.map((inv, i) => {
            const tokenShort = inv.token ? inv.token.slice(0, 12) : "?"
            const role = (inv.role ?? "user") as GroupRole
            const canEmailResend = !!inv.invitee_email
            // "Invited {{date}}" copy should reflect when the invite
            // was minted, not when it expires. expires_at would put
            // the future cancellation date in the past-tense label.
            const dateLabel = inv.created_at
              ? t("members:invite.sentAt", {
                  date: formatDate(inv.created_at, { style: "medium" }),
                })
              : ""
            return (
              <div key={inv.id ?? inv.token}>
                {i > 0 ? <Separator /> : null}
                <div
                  className="flex items-center gap-3 px-4 py-3"
                  data-testid={`invite-row-${tokenShort}`}
                >
                  <div className="flex size-8 items-center justify-center rounded-full bg-muted text-muted-foreground shrink-0">
                    <Mail className="size-4" aria-hidden="true" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p
                      className="text-sm font-medium truncate"
                      data-testid={`invite-email-${tokenShort}`}
                    >
                      {inv.invitee_email ?? t("members:invites.tokenLabel")}
                    </p>
                    {dateLabel ? (
                      <p className="text-xs text-muted-foreground">{dateLabel}</p>
                    ) : null}
                  </div>
                  <RoleBadge role={role} />
                  <Badge variant="secondary" className="text-[10px] h-5">
                    {t("members:invite.pendingBadge")}
                  </Badge>
                  <DropdownMenu>
                    <DropdownMenuTrigger asChild>
                      <Button
                        variant="ghost"
                        size="icon"
                        className="size-7"
                        aria-label={t("members:invites.title")}
                        data-testid={`invite-actions-${tokenShort}`}
                      >
                        <MoreHorizontal className="size-4" aria-hidden="true" />
                      </Button>
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align="end">
                      {canEmailResend ? (
                        <DropdownMenuItem
                          onSelect={(e) => {
                            e.preventDefault()
                            void handleResend(inv.id)
                          }}
                          disabled={resendMutation.isPending}
                          data-testid={`invite-resend-${tokenShort}`}
                        >
                          <Mail className="size-4 mr-2" aria-hidden="true" />
                          {t("members:invite.actions.resend")}
                        </DropdownMenuItem>
                      ) : null}
                      <DropdownMenuItem
                        onSelect={(e) => {
                          e.preventDefault()
                          void handleCopy(inv.token)
                        }}
                        data-testid={`invite-copy-${tokenShort}`}
                      >
                        <Copy className="size-4 mr-2" aria-hidden="true" />
                        {t("members:invites.copy")}
                      </DropdownMenuItem>
                      <DropdownMenuSeparator />
                      <DropdownMenuItem
                        className="text-destructive focus:text-destructive"
                        onSelect={(e) => {
                          e.preventDefault()
                          void handleRevoke(inv)
                        }}
                        disabled={revokeMutation.isPending}
                        data-testid={`invite-revoke-${tokenShort}`}
                      >
                        <Trash2 className="size-4 mr-2" aria-hidden="true" />
                        {revokeMutation.isPending
                          ? t("members:invites.revoking")
                          : t("members:invite.actions.revoke")}
                      </DropdownMenuItem>
                    </DropdownMenuContent>
                  </DropdownMenu>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </section>
  )
}

// ─── Invite dialog ──────────────────────────────────────────────────────────

function InviteDialog({
  open,
  onOpenChange,
  groupId,
  groupName,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  groupId: string
  groupName: string
}) {
  const { t } = useTranslation()
  const toast = useAppToast()
  const createMutation = useCreateInvite()
  const [email, setEmail] = useState("")
  const [role, setRole] = useState<GroupRole>("user")
  const [sent, setSent] = useState(false)
  // tokenFallback carries the freshly-minted invite token when the
  // admin chose the legacy copy-paste flow inside this dialog. Display
  // the URL in-place so they can copy it without leaving the dialog.
  const [tokenFallback, setTokenFallback] = useState<string | null>(null)
  // Track the auto-close timer so a manual cancel, an Escape press,
  // or an unmount clears it before it can call setState on a torn-down
  // component (React's "setState on unmounted component" warning).
  const autoCloseTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  function clearAutoCloseTimer() {
    if (autoCloseTimer.current !== null) {
      clearTimeout(autoCloseTimer.current)
      autoCloseTimer.current = null
    }
  }

  useEffect(() => {
    return () => clearAutoCloseTimer()
  }, [])

  function reset() {
    clearAutoCloseTimer()
    setEmail("")
    setRole("user")
    setSent(false)
    setTokenFallback(null)
  }

  function handleOpenChange(next: boolean) {
    if (!next) reset()
    onOpenChange(next)
  }

  async function sendEmailInvite() {
    if (!email.trim()) return
    try {
      await createMutation.mutateAsync({ groupId, email: email.trim(), role })
      setSent(true)
      // Close the dialog after a short feedback delay so the user sees
      // the "Sent!" confirmation; mirrors the mock's transient state.
      // The handle is captured in a ref so reset() / unmount clears it.
      clearAutoCloseTimer()
      autoCloseTimer.current = setTimeout(() => {
        autoCloseTimer.current = null
        handleOpenChange(false)
      }, 1200)
    } catch (err) {
      toast.error(parseServerError(err, t("members:invites.createError")))
    }
  }

  async function createTokenOnlyInvite() {
    try {
      const created = await createMutation.mutateAsync({ groupId, role })
      setTokenFallback(created.token ?? null)
    } catch (err) {
      toast.error(parseServerError(err, t("members:invites.createError")))
    }
  }

  async function copyToken() {
    if (!tokenFallback) return
    try {
      await navigator.clipboard.writeText(inviteUrl(tokenFallback))
      toast.success(t("members:invites.copied"))
    } catch {
      toast.error(t("members:invites.copyFailed"))
    }
  }

  return (
    <Dialog open={open} onOpenChange={handleOpenChange}>
      <DialogContent className="sm:max-w-md" data-testid="invite-dialog">
        <DialogHeader>
          <DialogTitle>{t("members:invite.dialog.title", { group: groupName })}</DialogTitle>
          <DialogDescription>{t("members:invite.dialog.description")}</DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4 py-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="invite-email">{t("members:invite.dialog.emailLabel")}</Label>
            <Input
              id="invite-email"
              type="email"
              // Radix Dialog auto-focuses the first tabbable child on
              // open, which lands on this email input. Skipping the
              // explicit autoFocus prop keeps jsx-a11y/no-autofocus
              // happy without changing the actual UX.
              placeholder={t("members:invite.dialog.emailPlaceholder")}
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && email.trim() && !sent) {
                  e.preventDefault()
                  void sendEmailInvite()
                }
              }}
              data-testid="invite-email-input"
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="invite-role">{t("members:invite.dialog.roleLabel")}</Label>
            <Select value={role} onValueChange={(v) => setRole(v as GroupRole)}>
              <SelectTrigger id="invite-role" data-testid="invite-role-select">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {INVITE_ROLE_OPTIONS.map((r) => {
                  const Icon = ROLE_ICON[r]
                  return (
                    <SelectItem key={r} value={r} data-testid={`invite-role-option-${r}`}>
                      <div className="flex items-center gap-2">
                        <Icon
                          className="size-3.5 text-muted-foreground shrink-0"
                          aria-hidden="true"
                        />
                        <div>
                          <p className="font-medium">{t(`members:roles.${r}`)}</p>
                          <p className="text-xs text-muted-foreground">
                            {t(`members:roleDescriptions.${r}`)}
                          </p>
                        </div>
                      </div>
                    </SelectItem>
                  )
                })}
              </SelectContent>
            </Select>
          </div>
          {tokenFallback ? (
            <div
              className="rounded-lg border border-primary/40 bg-primary/5 p-3 space-y-2"
              data-testid="invite-token-fallback"
            >
              <p className="text-xs font-medium">{t("members:invites.tokenLabel")}</p>
              <div className="flex items-center gap-2">
                <code
                  className="flex-1 truncate rounded-md bg-background px-2 py-1.5 text-xs font-mono"
                  data-testid="invite-token-url"
                >
                  {inviteUrl(tokenFallback)}
                </code>
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  className="gap-1.5"
                  onClick={copyToken}
                  data-testid="invite-token-copy"
                >
                  <Copy className="size-3.5" aria-hidden="true" />
                  {t("members:invites.copy")}
                </Button>
              </div>
            </div>
          ) : null}
          <div className="flex gap-2 pt-2">
            <Button
              type="button"
              variant="outline"
              className="flex-1"
              onClick={() => handleOpenChange(false)}
              data-testid="invite-cancel"
            >
              {t("members:invite.dialog.cancel")}
            </Button>
            <Button
              type="button"
              className="flex-1 gap-2"
              onClick={sendEmailInvite}
              disabled={!email.trim() || sent || createMutation.isPending}
              data-testid="invite-send"
            >
              {sent ? (
                <>
                  <Check className="size-4" aria-hidden="true" />
                  {t("members:invite.dialog.sent")}
                </>
              ) : createMutation.isPending ? (
                <>
                  <UserPlus className="size-4" aria-hidden="true" />
                  {t("members:invite.dialog.sending")}
                </>
              ) : (
                <>
                  <UserPlus className="size-4" aria-hidden="true" />
                  {t("members:invite.dialog.send")}
                </>
              )}
            </Button>
          </div>
          {!tokenFallback ? (
            <button
              type="button"
              onClick={createTokenOnlyInvite}
              disabled={createMutation.isPending}
              className="text-xs text-muted-foreground hover:text-foreground underline-offset-4 hover:underline self-start"
              data-testid="invite-token-fallback-cta"
            >
              {t("members:invite.dialog.tokenFallback")}
            </button>
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  )
}
