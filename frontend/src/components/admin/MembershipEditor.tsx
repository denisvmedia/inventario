import { useEffect, useMemo, useState } from "react"
import { Ellipsis, Trash2, Users, UserPlus } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Alert, AlertDescription } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type { AdminGroupMember, GroupRole } from "@/features/admin/api"
import {
  useAddAdminGroupMember,
  useAdminGroupMembers,
  useAdminTenantUsers,
  useRemoveAdminGroupMember,
  useUpdateAdminGroupMemberRole,
} from "@/features/admin/hooks"
import { useConfirm } from "@/hooks/useConfirm"
import { getServerErrorCode } from "@/lib/server-error"

import { ROLE_CONFIG, ROLE_ORDER } from "@/pages/admin/admin-shared"

// Typed 422 codes the membership endpoints return, mapped to their flat
// i18n key suffix under `groupDetail.members.errors.*`. The BE codes are
// dotted (`admin.member.tenant_mismatch`); i18n keys must NOT contain dots
// (the catalog uses `.` as the nesting separator), so this map translates
// each code to a single key segment. Mirrors AdminUserDetailPage's
// BLOCK_ERROR_KEY pattern. An uncoded 422 (membership cap / already a
// member) and any other failure fall through to the `generic` key.
const MEMBER_ERROR_KEY: Record<string, string> = {
  "admin.member.tenant_mismatch": "tenantMismatch",
  "admin.member.invalid_role": "invalidRole",
  // `admin.member.user_required` should be unreachable from the UI (the
  // Add button is gated on a resolved user), but map it so a defensive
  // surface still shows a typed message instead of the generic catch-all.
  "admin.member.user_required": "invalidRole",
  "group.last_owner": "lastOwner",
  "group.last_member": "lastMember",
}

// Resolves a thrown mutation error to a localized banner message: a known
// typed 422 code gets its specific copy; everything else (an uncoded 422
// such as the membership cap, a 404, a 5xx) gets the generic copy.
function memberErrorKey(err: unknown): string {
  const code = getServerErrorCode(err)
  const suffix = code ? MEMBER_ERROR_KEY[code] : undefined
  return `groupDetail.members.errors.${suffix ?? "generic"}`
}

// Debounce delay for the add-member email lookup. Long enough that a
// typist doesn't fire a request per keystroke, short enough to feel live.
const LOOKUP_DEBOUNCE_MS = 300

interface MembershipEditorProps {
  // The group whose roster is being edited.
  groupId: string
  // The group's display name — used in the remove-confirm copy.
  groupName: string
  // The group's owning tenant. The add-member email lookup is scoped to
  // this tenant: searching only within it structurally prevents
  // cross-tenant adds from the UI (the BE still rejects them with
  // `admin.member.tenant_mismatch`, surfaced defensively).
  tenantId: string
  // The owning tenant's human-readable display name — used only in the
  // add-member dialog copy, so the operator reads a tenant name rather
  // than the raw `tenantId` UUID. `tenantId` is still the lookup key.
  tenantName: string
  // When the group is `pending_deletion` the whole page is read-only:
  // Add / Remove / role-change controls are suppressed, mirroring how the
  // page's DangerZone disables itself.
  readOnly?: boolean
}

// MembershipEditor is the embedded add/remove/role editor on the admin
// Group detail page (#1756). It replaces the earlier #1755 placeholder.
//
// Layout authority is the admin design mock
// (design-mocks/src/views/admin/GroupDetailView.tsx): the roster is a
// <Table> (Member / Role / actions) with an inline role <Select> per row
// and a per-row dropdown carrying "Remove from group". The role taxonomy
// (viewer/user/admin/owner ordering, icons, tints) is reused from the
// admin ROLE_CONFIG / ROLE_ORDER — the same taxonomy the per-group
// MembersPage uses. The mock's remove-confirmation AlertDialog is replaced
// by the codebase's `useConfirm()` primitive (no AlertDialog primitive
// exists); see devdocs/frontend/design-deviations.md.
export function MembershipEditor({
  groupId,
  groupName,
  tenantId,
  tenantName,
  readOnly = false,
}: MembershipEditorProps) {
  const { t } = useTranslation("admin")
  const confirm = useConfirm()

  const membersQuery = useAdminGroupMembers(groupId)
  const addMember = useAddAdminGroupMember(groupId)
  const removeMember = useRemoveAdminGroupMember(groupId)
  const updateRole = useUpdateAdminGroupMemberRole(groupId)

  const [addOpen, setAddOpen] = useState(false)
  // Bumped each time the add dialog opens; used as the dialog's React
  // `key` so it remounts with fresh internal state every time — a clean
  // reset without a setState-in-effect (which the lint rules forbid).
  const [addOpenCount, setAddOpenCount] = useState(0)
  // Inline banner for a failed remove / role-change — placed above the
  // table. A successful add closes the dialog, so add errors live in the
  // dialog (`addError`) rather than here.
  const [tableError, setTableError] = useState<string | null>(null)

  function openAddDialog() {
    setAddOpenCount((n) => n + 1)
    setAddOpen(true)
  }

  const members = membersQuery.data ?? []

  function handleChangeRole(member: AdminGroupMember, role: GroupRole) {
    const userId = member.member_user_id
    if (!userId || role === member.role) return
    setTableError(null)
    updateRole.mutate({ userId, role }, { onError: (err) => setTableError(memberErrorKey(err)) })
  }

  async function handleRemove(member: AdminGroupMember) {
    const userId = member.member_user_id
    if (!userId) return
    const name = member.user?.name || member.user?.email || userId
    const ok = await confirm({
      title: t("groupDetail.members.remove.title"),
      description: t("groupDetail.members.remove.body", { name, group: groupName }),
      confirmLabel: t("groupDetail.members.remove.confirm"),
      cancelLabel: t("groupDetail.members.remove.cancel"),
      destructive: true,
    })
    if (!ok) return
    setTableError(null)
    removeMember.mutate(userId, {
      onError: (err) => setTableError(memberErrorKey(err)),
    })
  }

  return (
    <div data-testid="admin-group-members">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h2 className="text-base font-semibold">{t("groupDetail.members.title")}</h2>
        {readOnly ? null : (
          <Button
            size="sm"
            className="gap-1.5"
            onClick={openAddDialog}
            data-testid="admin-group-members-add"
          >
            <UserPlus className="size-3.5" />
            {t("groupDetail.members.add.button")}
          </Button>
        )}
      </div>

      {tableError ? (
        <Alert variant="destructive" className="mb-3" data-testid="admin-group-members-error">
          <AlertDescription>{t(tableError)}</AlertDescription>
        </Alert>
      ) : null}

      <div className="rounded-xl border border-border bg-card overflow-hidden">
        {membersQuery.isError ? (
          <div
            className="p-6 text-sm text-destructive"
            data-testid="admin-group-members-load-error"
          >
            {t("groupDetail.members.loadError")}
          </div>
        ) : membersQuery.isLoading ? (
          <div className="p-6 text-sm text-muted-foreground">
            {t("groupDetail.members.loading")}
          </div>
        ) : (
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="pl-4">{t("groupDetail.members.colMember")}</TableHead>
                <TableHead>{t("groupDetail.members.colRole")}</TableHead>
                <TableHead className="w-10 pr-4" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {members.length === 0 ? (
                <TableRow className="hover:bg-transparent">
                  <TableCell colSpan={3} className="h-24 text-center text-sm text-muted-foreground">
                    <div className="flex flex-col items-center justify-center gap-2">
                      <Users className="size-8 text-muted-foreground/30" />
                      <span>{t("groupDetail.members.empty")}</span>
                    </div>
                  </TableCell>
                </TableRow>
              ) : (
                members.map((member) => (
                  <MemberRow
                    key={member.id ?? member.member_user_id}
                    member={member}
                    readOnly={readOnly}
                    anyMutationPending={updateRole.isPending || removeMember.isPending}
                    onChangeRole={handleChangeRole}
                    onRemove={handleRemove}
                  />
                ))
              )}
            </TableBody>
          </Table>
        )}
      </div>

      {readOnly ? null : (
        <AddMemberDialog
          key={addOpenCount}
          open={addOpen}
          onOpenChange={setAddOpen}
          tenantId={tenantId}
          tenantName={tenantName}
          isAdding={addMember.isPending}
          onAdd={(userID, role, onError) =>
            addMember.mutate({ userID, role }, { onSuccess: () => setAddOpen(false), onError })
          }
        />
      )}
    </div>
  )
}

// One roster row: a Member identity cell, an inline role <Select>, and a
// per-row dropdown with "Remove from group". In read-only mode the role
// renders as static text and the actions dropdown is dropped.
function MemberRow({
  member,
  readOnly,
  anyMutationPending,
  onChangeRole,
  onRemove,
}: {
  member: AdminGroupMember
  readOnly: boolean
  // True while a role-change OR remove mutation is in flight for ANY row —
  // editor-global, not row-local. It disables every row's role <Select> so
  // concurrent mutations can't race.
  anyMutationPending: boolean
  onChangeRole: (member: AdminGroupMember, role: GroupRole) => void
  onRemove: (member: AdminGroupMember) => void
}) {
  const { t } = useTranslation("admin")
  const name = member.user?.name || member.user?.email || "—"
  const email = member.user?.email ?? ""
  const role = member.role
  // `role` is free-form TEXT on the BE and the enum can evolve, so a value
  // outside viewer|user|admin|owner leaves `ROLE_CONFIG[role]` undefined.
  // Guard both — mirrors RoleBadge in admin-shared.tsx — so an unknown role
  // renders the em-dash fallback instead of crashing on `.i18nKey`.
  const roleConfig = role ? ROLE_CONFIG[role] : undefined

  return (
    <TableRow className="hover:bg-transparent" data-testid="admin-group-member-row">
      <TableCell className="pl-4 py-3.5">
        <div className="flex items-center gap-2.5">
          <div className="flex size-8 items-center justify-center rounded-full bg-muted text-xs font-semibold text-muted-foreground shrink-0">
            {initialsOf(member.user?.name, member.user?.email)}
          </div>
          <div className="min-w-0">
            <p className="text-sm font-medium truncate">{name}</p>
            {email ? <p className="text-xs text-muted-foreground truncate">{email}</p> : null}
          </div>
        </div>
      </TableCell>
      <TableCell className="py-3.5">
        {readOnly || !roleConfig ? (
          <span className="text-sm text-muted-foreground">
            {roleConfig ? t(roleConfig.i18nKey) : "—"}
          </span>
        ) : (
          <Select
            value={role}
            onValueChange={(v) => onChangeRole(member, v as GroupRole)}
            disabled={anyMutationPending}
          >
            <SelectTrigger size="sm" className="w-40" data-testid="admin-group-member-role">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ROLE_ORDER.map((option) => {
                const cfg = ROLE_CONFIG[option]
                const Icon = cfg.icon
                return (
                  <SelectItem key={option} value={option}>
                    <Icon className="size-3.5 text-muted-foreground shrink-0" />
                    {t(cfg.i18nKey)}
                  </SelectItem>
                )
              })}
            </SelectContent>
          </Select>
        )}
      </TableCell>
      <TableCell className="pr-4 py-3.5 text-right">
        {readOnly ? null : (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="ghost"
                size="icon"
                className="size-7"
                aria-label={t("groupDetail.members.rowActions")}
                data-testid="admin-group-member-actions"
              >
                <Ellipsis className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem
                className="text-destructive focus:text-destructive"
                onClick={() => onRemove(member)}
                data-testid="admin-group-member-remove"
              >
                <Trash2 className="size-4 mr-2" />
                {t("groupDetail.members.remove.action")}
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </TableCell>
    </TableRow>
  )
}

// The add-member dialog: an email field + a role picker. The BE add
// endpoint needs a resolved `userID`, not an email — so as the operator
// types, the dialog debounces a `?q=<email>` search scoped to the group's
// tenant (useAdminTenantUsers) and looks for the single exact
// case-insensitive email match. The Add button enables only once a match
// is resolved; a no-match shows an inline "no user with that email"
// notice. This structurally prevents cross-tenant adds; a
// `tenant_mismatch` 422 is still mapped defensively by the parent.
function AddMemberDialog({
  open,
  onOpenChange,
  tenantId,
  tenantName,
  isAdding,
  onAdd,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  // The tenant the email lookup is scoped to (the raw id).
  tenantId: string
  // The tenant's human-readable name, shown in the dialog description copy.
  tenantName: string
  isAdding: boolean
  onAdd: (userID: string, role: GroupRole, onError: (err: unknown) => void) => void
}) {
  const { t } = useTranslation("admin")

  // This component is remounted (via a React `key` in the parent) every
  // time the dialog opens, so a fresh useState seed is the reset — no
  // setState-in-effect needed to clear a previous failed attempt.
  const [email, setEmail] = useState("")
  const [role, setRole] = useState<GroupRole>("user")
  // Debounced copy of `email` — the lookup query keys off this so it
  // doesn't refetch on every keystroke.
  const [debouncedEmail, setDebouncedEmail] = useState("")
  // Inline banner inside the dialog for a failed add (typed 422 etc.).
  const [addError, setAddError] = useState<string | null>(null)

  // Debounce the email → lookup. The query trims + lower-cases for the
  // exact match below; here we only gate on a non-empty trimmed value.
  useEffect(() => {
    const trimmed = email.trim()
    const handle = setTimeout(() => setDebouncedEmail(trimmed), LOOKUP_DEBOUNCE_MS)
    return () => clearTimeout(handle)
  }, [email])

  // Scoped tenant-user search. Enabled only with a debounced query and an
  // open dialog — a closed dialog never holds an active lookup.
  const lookup = useAdminTenantUsers(
    tenantId,
    { q: debouncedEmail, perPage: 10 },
    { enabled: open && debouncedEmail.length > 0 }
  )

  // The single exact case-insensitive email match within the tenant, if
  // any. `?q=` is a fuzzy match server-side, so we still pin it to an
  // exact-equality check before treating it as resolved.
  const resolvedUser = useMemo(() => {
    const target = debouncedEmail.toLowerCase()
    if (!target) return undefined
    return (lookup.data?.users ?? []).find((u) => (u.email ?? "").toLowerCase() === target)
  }, [lookup.data, debouncedEmail])

  // The lookup is "settled with a verdict" only once the debounced query
  // matches the current input and the request isn't in flight. Until then
  // the not-found notice is suppressed (it would otherwise flash while the
  // operator is mid-type).
  const lookupSettled =
    debouncedEmail.length > 0 && debouncedEmail === email.trim() && !lookup.isFetching
  const showNotFound = lookupSettled && !lookup.isError && !resolvedUser

  function handleAdd() {
    if (!resolvedUser?.id) return
    setAddError(null)
    onAdd(resolvedUser.id, role, (err) => setAddError(memberErrorKey(err)))
  }

  return (
    <Dialog open={open} onOpenChange={(next) => !isAdding && onOpenChange(next)}>
      <DialogContent className="sm:max-w-md" data-testid="admin-group-add-dialog">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <div className="flex size-7 items-center justify-center rounded-lg bg-primary/10">
              <UserPlus className="size-4 text-primary" />
            </div>
            {t("groupDetail.members.add.title")}
          </DialogTitle>
          <DialogDescription>
            {t("groupDetail.members.add.description", { tenant: tenantName })}
          </DialogDescription>
        </DialogHeader>

        <div className="flex flex-col gap-4 py-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="admin-group-add-email">{t("groupDetail.members.add.emailLabel")}</Label>
            <Input
              id="admin-group-add-email"
              type="email"
              placeholder={t("groupDetail.members.add.emailPlaceholder")}
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              data-testid="admin-group-add-email"
            />
            {/* Lookup status line: searching → found → not-found. */}
            {lookup.isFetching && debouncedEmail.length > 0 ? (
              <p className="text-xs text-muted-foreground">
                {t("groupDetail.members.add.searching")}
              </p>
            ) : resolvedUser ? (
              <p className="text-xs text-status-active" data-testid="admin-group-add-resolved">
                {t("groupDetail.members.add.found", {
                  name: resolvedUser.name || resolvedUser.email,
                  email: resolvedUser.email,
                })}
              </p>
            ) : lookupSettled && lookup.isError ? (
              <p className="text-xs text-destructive">{t("groupDetail.members.add.lookupError")}</p>
            ) : showNotFound ? (
              <p className="text-xs text-destructive" data-testid="admin-group-add-not-found">
                {t("groupDetail.members.add.notFound")}
              </p>
            ) : null}
          </div>

          <div className="flex flex-col gap-1.5">
            <Label htmlFor="admin-group-add-role">{t("groupDetail.members.add.roleLabel")}</Label>
            <Select value={role} onValueChange={(v) => setRole(v as GroupRole)}>
              <SelectTrigger id="admin-group-add-role" data-testid="admin-group-add-role">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ROLE_ORDER.map((option) => {
                  const cfg = ROLE_CONFIG[option]
                  const Icon = cfg.icon
                  return (
                    <SelectItem key={option} value={option}>
                      <Icon className="size-3.5 text-muted-foreground shrink-0" />
                      {t(cfg.i18nKey)}
                    </SelectItem>
                  )
                })}
              </SelectContent>
            </Select>
          </div>
        </div>

        {addError ? (
          <Alert variant="destructive" data-testid="admin-group-add-error">
            <AlertDescription>{t(addError)}</AlertDescription>
          </Alert>
        ) : null}

        <DialogFooter className="gap-2">
          <Button
            variant="outline"
            onClick={() => onOpenChange(false)}
            disabled={isAdding}
            data-testid="admin-group-add-cancel"
          >
            {t("groupDetail.members.remove.cancel")}
          </Button>
          <Button
            className="gap-2"
            onClick={handleAdd}
            disabled={isAdding || !resolvedUser}
            data-testid="admin-group-add-confirm"
          >
            <UserPlus className="size-4" />
            {t("groupDetail.members.add.confirm")}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

// Builds avatar initials from a member's display name (first letters of
// the first two words, uppercased), falling back to the first letter of
// the email and finally "?". Mirrors AdminUserDetailPage's `initialsOf`.
function initialsOf(name: string | undefined, email: string | undefined): string {
  const parts = (name ?? "").trim().split(/\s+/).filter(Boolean)
  if (parts.length > 0) {
    return parts
      .slice(0, 2)
      .map((p) => p[0]!.toUpperCase())
      .join("")
  }
  const e = (email ?? "").trim()
  return e ? e[0]!.toUpperCase() : "?"
}
