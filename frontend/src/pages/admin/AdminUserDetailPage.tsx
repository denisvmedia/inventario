import { useState } from "react"
import { ArrowLeft, Ban, CircleCheck, Layers, Monitor, User, UserCog } from "lucide-react"
import { Link, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { Textarea } from "@/components/ui/textarea"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAdminUser, useBlockAdminUser, useUnblockAdminUser } from "@/features/admin/hooks"
import type { AdminUserDetail } from "@/features/admin/api"
import { HttpError } from "@/lib/http"
import { formatDateTime, formatRelative } from "@/lib/intl"
import { getServerErrorCode } from "@/lib/server-error"
import { cn } from "@/lib/utils"

import { AccountStateBadge, RoleBadge, TenantChip } from "./admin-shared"

// The BE caps the block / unblock `reason` at 500 Unicode code points
// (`utf8.RuneCountInString(reason) > 500` → 422 `admin.block.reason_too_long`).
// The dialog mirrors that cap client-side by counting code points — see
// `reasonLength` below — and gates the confirm button on it. If a value
// somehow slips past the gate the BE still rejects it and the typed
// `reasonTooLong` banner surfaces, so the cap is enforced defence-in-depth.
const REASON_MAX = 500

// Typed 422 error codes the block / unblock endpoints return, mapped to
// their i18n key suffix. The page surfaces each as a specific localized
// inline banner message instead of a generic toast (issue #1754 acceptance
// criteria). The BE codes are dotted (`admin.block.self_blocked`); i18n
// keys must NOT contain dots (the catalog uses `.` as the nesting
// separator), so this map translates each code to a flat key segment.
const BLOCK_ERROR_KEY: Record<string, string> = {
  "admin.block.self_blocked": "selfBlocked",
  "admin.block.admin_requires_force": "adminRequiresForce",
  "admin.block.reason_required": "reasonRequired",
  "admin.block.reason_too_long": "reasonTooLong",
}

// Builds an avatar initials string from a display name — first letters of
// the first two words, uppercased. Falls back to "?" when the name is empty.
function initialsOf(name: string | undefined): string {
  const parts = (name ?? "").trim().split(/\s+/).filter(Boolean)
  if (parts.length === 0) return "?"
  return parts
    .slice(0, 2)
    .map((p) => p[0]!.toUpperCase())
    .join("")
}

// AdminUserDetailPage is /admin/users/:userId — a per-user admin surface
// with an identity card, an active-session count summary, a group-
// membership table, and block / unblock + (placeholder) impersonate
// controls.
//
// Naming: the issue text proposed `UserDetailPage.tsx`, but the #1752
// foundation established the `Admin*Page` convention for this surface
// (AdminTenantsPage / AdminTenantDetailPage / AdminUsersPage); this page
// follows the codebase convention. See devdocs/frontend/design-deviations.md.
//
// Sessions: the design mock renders a per-session table (device / IP /
// location). The BE only returns `active_session_count` — an integer — so
// this page renders a count summary instead. Logged as a design deviation.
export function AdminUserDetailPage() {
  const { t } = useTranslation("admin")
  const { userId = "" } = useParams()

  const user = useAdminUser(userId)

  // GET /admin/users/{id} returns HTTP 404 for a missing user — treat that
  // 404 as a friendly not-found empty state, and keep the generic
  // load-error card for every other failure (mirrors AdminTenantDetailPage).
  const isNotFound = user.isError && user.error instanceof HttpError && user.error.status === 404

  const userName = user.data?.name ?? t("userDetail.fallbackName")

  return (
    <>
      <RouteTitle title={userName} />
      <div className="flex flex-col gap-6" data-testid="admin-user-detail-page">
        <Button
          variant="ghost"
          size="sm"
          asChild
          className="gap-1.5 -ml-2 self-start text-muted-foreground hover:text-foreground"
        >
          <Link to="/admin/users">
            <ArrowLeft className="size-4" />
            {t("userDetail.back")}
          </Link>
        </Button>

        {isNotFound ? (
          <div
            className="flex flex-col items-center justify-center gap-3 py-24"
            data-testid="admin-user-not-found"
          >
            <User className="size-8 text-muted-foreground/30" />
            <p className="text-sm text-muted-foreground">{t("userDetail.notFound")}</p>
          </div>
        ) : user.isError ? (
          <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-6 text-sm text-destructive">
            {t("userDetail.loadError")}
          </div>
        ) : user.isLoading || !user.data ? (
          <div className="rounded-xl border border-border bg-card p-6 text-sm text-muted-foreground">
            {t("userDetail.loading")}
          </div>
        ) : (
          <UserDetailContent user={user.data} />
        )}
      </div>
    </>
  )
}

// The loaded body — split out so the data-gating above stays a flat ladder
// and the optimistic block/unblock state can be hooked unconditionally.
function UserDetailContent({ user }: { user: AdminUserDetail }) {
  const { t } = useTranslation("admin")
  const userId = user.id ?? ""

  // Optimistic active state: the badge flips immediately on confirm and
  // rolls back to the server's authoritative value if the mutation fails.
  // Seeded from the query and only ever moved by an optimistic write or a
  // rollback — the post-success invalidation re-fetches the real value.
  const [optimisticActive, setOptimisticActive] = useState<boolean | undefined>(undefined)
  const active = optimisticActive ?? user.is_active

  const [dialog, setDialog] = useState<"block" | "unblock" | null>(null)
  const [reason, setReason] = useState("")
  // Inline error banner state for a failed block / unblock. A single
  // discriminated value: `{ kind: "typed", key }` carries a known 422
  // `admin.block.*` code mapped to its i18n key suffix (see
  // BLOCK_ERROR_KEY); `{ kind: "generic" }` is the catch-all; `null` is
  // "no error". This replaces the earlier `errorKey` + `genericError`
  // pair so the two can never disagree.
  const [actionError, setActionError] = useState<
    { kind: "typed"; key: string } | { kind: "generic" } | null
  >(null)

  const blockMutation = useBlockAdminUser(userId)
  const unblockMutation = useUnblockAdminUser(userId)
  const pending = blockMutation.isPending || unblockMutation.isPending

  const memberships = user.group_memberships ?? []
  const sessionCount = user.active_session_count ?? 0

  // Code-point length of the reason — `String.prototype.length` counts
  // UTF-16 code units, which over-counts non-BMP input (emoji, rare CJK)
  // versus the BE's `utf8.RuneCountInString`. Spreading into an array
  // iterates by code point, matching the server-side rule exactly.
  const reasonLength = [...reason].length
  const reasonInvalid = reasonLength === 0 || reasonLength > REASON_MAX

  function openDialog(kind: "block" | "unblock") {
    setReason("")
    setActionError(null)
    setDialog(kind)
  }

  function closeDialog() {
    setDialog(null)
  }

  // Maps a thrown mutation error to the inline banner state: a known
  // 422 `admin.block.*` code surfaces as a specific localized message;
  // anything else surfaces as the generic banner.
  function applyError(err: unknown) {
    const code = getServerErrorCode(err)
    const key = code ? BLOCK_ERROR_KEY[code] : undefined
    setActionError(key ? { kind: "typed", key } : { kind: "generic" })
  }

  function handleBlock() {
    const trimmed = reason.trim()
    setActionError(null)
    // Optimistic flip — the badge shows "Blocked" before the round-trip.
    setOptimisticActive(false)
    blockMutation.mutate(
      { reason: trimmed, force: false },
      {
        onSuccess: () => {
          // The mutation's onSuccess already wrote the authoritative
          // is_active into the cache, so dropping the optimistic value
          // here reads the fresh cached value, not a stale one.
          setOptimisticActive(undefined)
          closeDialog()
          toast.success(t("userDetail.block.success", { name: user.name ?? "" }))
        },
        onError: (err) => {
          // Roll the optimistic flip back to the authoritative value.
          setOptimisticActive(undefined)
          applyError(err)
        },
      }
    )
  }

  function handleUnblock() {
    const trimmed = reason.trim()
    setActionError(null)
    setOptimisticActive(true)
    unblockMutation.mutate(
      { reason: trimmed },
      {
        onSuccess: () => {
          setOptimisticActive(undefined)
          closeDialog()
          toast.success(t("userDetail.unblock.success", { name: user.name ?? "" }))
        },
        onError: (err) => {
          setOptimisticActive(undefined)
          applyError(err)
        },
      }
    )
  }

  return (
    <>
      {/* Identity card */}
      <div
        className="rounded-xl border border-border bg-card p-6"
        data-testid="admin-user-identity"
      >
        <div className="flex items-start gap-4">
          <div className="flex size-14 items-center justify-center rounded-full bg-primary text-primary-foreground text-lg font-semibold shrink-0">
            {initialsOf(user.name)}
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight">
                {user.name || t("userDetail.fallbackName")}
              </h1>
              <AccountStateBadge active={active} />
            </div>
            <p className="mt-0.5 text-sm text-muted-foreground">{user.email || "—"}</p>
            <div className="mt-3 flex flex-wrap items-center gap-2">
              <TenantChip tenantId={user.tenant_id} />
              {user.is_system_admin ? (
                <Badge
                  variant="outline"
                  className="h-5 text-xs border-current/20 font-medium gap-1 text-primary bg-primary/10"
                >
                  {t("userDetail.systemAdminBadge")}
                </Badge>
              ) : null}
            </div>
          </div>
        </div>

        <Separator className="my-5" />

        <div className="flex flex-wrap items-center gap-2">
          {/* Impersonate is a placeholder in #1754 — full wiring is #1757. */}
          <Button
            size="sm"
            variant="outline"
            className="gap-1.5"
            disabled
            title={t("userDetail.impersonateSoon")}
            data-testid="admin-user-impersonate"
          >
            <UserCog className="size-3.5" />
            {t("userDetail.impersonate")}
          </Button>
          {active ? (
            <Button
              size="sm"
              variant="outline"
              className="gap-1.5 text-destructive hover:bg-destructive/10 hover:text-destructive"
              onClick={() => openDialog("block")}
              data-testid="admin-user-block"
            >
              <Ban className="size-3.5" />
              {t("userDetail.block.action")}
            </Button>
          ) : (
            <Button
              size="sm"
              variant="outline"
              className="gap-1.5"
              onClick={() => openDialog("unblock")}
              data-testid="admin-user-unblock"
            >
              <CircleCheck className="size-3.5" />
              {t("userDetail.unblock.action")}
            </Button>
          )}
        </div>
      </div>

      {/* Sessions — the BE returns only a count, so this is a summary, not
          a table (see devdocs/frontend/design-deviations.md). */}
      <div>
        <div className="mb-3 flex items-center gap-2">
          <Monitor className="size-4 text-muted-foreground" />
          <h2 className="text-base font-semibold">{t("userDetail.sessions.title")}</h2>
          <Badge variant="secondary" className="h-5 text-xs">
            {sessionCount}
          </Badge>
        </div>
        <div className="rounded-xl border border-border bg-card" data-testid="admin-user-sessions">
          {sessionCount === 0 ? (
            <div className="flex flex-col items-center justify-center gap-2 py-12">
              <Monitor className="size-7 text-muted-foreground/30" />
              <p className="text-sm text-muted-foreground">{t("userDetail.sessions.empty")}</p>
            </div>
          ) : (
            <div className="flex items-center gap-3 p-6">
              <div className="flex size-10 items-center justify-center rounded-lg bg-muted shrink-0">
                <Monitor className="size-5 text-muted-foreground" />
              </div>
              <div>
                <p className="text-sm font-medium">
                  {t("userDetail.sessions.count", { count: sessionCount })}
                </p>
                <p className="text-xs text-muted-foreground">
                  {t("userDetail.sessions.countHint")}
                </p>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Group memberships */}
      <div>
        <div className="mb-3 flex items-center gap-2">
          <Layers className="size-4 text-muted-foreground" />
          <h2 className="text-base font-semibold">{t("userDetail.groups.title")}</h2>
          <Badge variant="secondary" className="h-5 text-xs">
            {memberships.length}
          </Badge>
        </div>
        <div
          className="rounded-xl border border-border bg-card divide-y divide-border overflow-hidden"
          data-testid="admin-user-groups"
        >
          {memberships.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-2 py-12">
              <Layers className="size-7 text-muted-foreground/30" />
              <p className="text-sm text-muted-foreground">{t("userDetail.groups.empty")}</p>
            </div>
          ) : (
            memberships.map((m, index) => {
              // Inner content is shared by the linked and non-linked row
              // variants. `joined_at` renders as relative time for parity
              // with the design mock (`relativeFromNow`); see admin-shared
              // `formatRelative` re-export.
              const rowBody = (
                <>
                  <div className="flex size-8 items-center justify-center rounded-lg bg-muted shrink-0">
                    <Layers className="size-4 text-muted-foreground" />
                  </div>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm font-medium truncate">{m.group_name || "—"}</p>
                    <p className="text-xs text-muted-foreground">
                      {m.joined_at
                        ? t("userDetail.groups.joined", { date: formatRelative(m.joined_at) })
                        : "—"}
                    </p>
                  </div>
                  <RoleBadge role={m.role} />
                </>
              )
              const rowClass =
                "flex w-full items-center gap-3 px-4 py-3.5 text-left transition-colors"
              // When `group_id` is absent the admin group-detail route
              // cannot be built, so render a non-interactive row instead
              // of a Link that would point at a broken `/admin/groups/`.
              return m.group_id ? (
                <Link
                  key={m.group_id}
                  to={`/admin/groups/${encodeURIComponent(m.group_id)}`}
                  className={`${rowClass} hover:bg-muted/50`}
                  data-testid="admin-user-group-row"
                >
                  {rowBody}
                </Link>
              ) : (
                <div key={`idx-${index}`} className={rowClass} data-testid="admin-user-group-row">
                  {rowBody}
                </div>
              )
            })
          )}
        </div>
      </div>

      {/* Meta */}
      <p className="text-xs text-muted-foreground">
        {t("userDetail.meta.lastLogin")}{" "}
        <span className="font-medium text-foreground">
          {user.last_login_at ? formatDateTime(user.last_login_at) : t("userDetail.neverLoggedIn")}
        </span>
        {user.created_at ? (
          <>
            {" · "}
            {t("userDetail.meta.created")}{" "}
            <span className="font-medium text-foreground">{formatDateTime(user.created_at)}</span>
          </>
        ) : null}
      </p>

      {/* Block / unblock confirmation with required reason textarea. The
          shared ConfirmProvider only supports title + description text, so
          a dedicated Dialog hosts the textarea (see the page header comment
          + the final report). */}
      <Dialog
        open={dialog !== null}
        onOpenChange={(open) => {
          if (!open && !pending) closeDialog()
        }}
      >
        <DialogContent data-testid="admin-user-action-dialog">
          <DialogHeader>
            <DialogTitle>
              {dialog === "unblock"
                ? t("userDetail.unblock.confirmTitle")
                : t("userDetail.block.confirmTitle")}
            </DialogTitle>
            <DialogDescription>
              {dialog === "unblock"
                ? t("userDetail.unblock.confirmBody", { name: user.name ?? "" })
                : t("userDetail.block.confirmBody", { name: user.name ?? "" })}
            </DialogDescription>
          </DialogHeader>

          <div className="space-y-1.5">
            <Label htmlFor="admin-user-action-reason">{t("userDetail.reason.label")}</Label>
            <Textarea
              id="admin-user-action-reason"
              value={reason}
              onChange={(event) => setReason(event.target.value)}
              rows={3}
              placeholder={t("userDetail.reason.placeholder")}
              data-testid="admin-user-action-reason"
            />
            <p
              className={cn(
                "text-xs text-muted-foreground",
                reasonLength > REASON_MAX && "text-destructive"
              )}
            >
              {t("userDetail.reason.counter", { current: reasonLength, max: REASON_MAX })}
            </p>
          </div>

          {actionError ? (
            <div
              role="alert"
              className="rounded-lg border border-destructive/30 bg-destructive/5 px-3 py-2 text-sm text-destructive"
              data-testid="admin-user-action-error"
            >
              {actionError.kind === "typed"
                ? t(`userDetail.errors.${actionError.key}`)
                : t("userDetail.errors.generic")}
            </div>
          ) : null}

          <DialogFooter className="gap-2">
            <Button
              variant="outline"
              onClick={closeDialog}
              disabled={pending}
              data-testid="admin-user-action-cancel"
            >
              {t("userDetail.dialog.cancel")}
            </Button>
            {dialog === "unblock" ? (
              <Button
                onClick={handleUnblock}
                disabled={pending || reasonInvalid}
                data-testid="admin-user-action-confirm"
              >
                {t("userDetail.unblock.action")}
              </Button>
            ) : (
              <Button
                variant="destructive"
                onClick={handleBlock}
                disabled={pending || reasonInvalid}
                data-testid="admin-user-action-confirm"
              >
                {t("userDetail.block.action")}
              </Button>
            )}
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
