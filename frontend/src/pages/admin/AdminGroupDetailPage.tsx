import { ArrowLeft, Building2, Hash, Layers, Trash2, TriangleAlert, Users } from "lucide-react"
import { Link, useParams } from "react-router-dom"
import { useTranslation } from "react-i18next"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAdminGroup, useDeleteAdminGroup } from "@/features/admin/hooks"
import type { AdminGroupDetail } from "@/features/admin/api"
import { useConfirm } from "@/hooks/useConfirm"
import { HttpError } from "@/lib/http"
import { formatDate } from "@/lib/intl"

import { GroupStatusBadge } from "./admin-shared"

// AdminGroupDetailPage is /admin/groups/:groupId — a read-only group
// header (name, slug, owning-tenant chip, status, currency, member count,
// created_by) plus a Members panel and a Danger zone with a soft-delete
// action.
//
// The Members panel is a deliberate PLACEHOLDER: the real add/remove/role
// editor ships in a later sub-issue. The Danger zone soft-deletes the
// group via DELETE /admin/groups/{id}; on success the group flips to
// `pending_deletion` and the whole page becomes read-only.
//
// Naming follows the #1752 `Admin*Page` convention (see AdminTenantsPage /
// AdminTenantDetailPage), not the issue text's `GroupDetailPage`.
export function AdminGroupDetailPage() {
  const { t } = useTranslation("admin")
  const { groupId = "" } = useParams()

  const group = useAdminGroup(groupId)

  // GET /admin/groups/{id} returns HTTP 404 for a missing group
  // (apiserver maps registry.ErrNotFound → NewNotFoundError), so a
  // genuine not-found surfaces as a query error. Treat that 404 as "not
  // found" (its own friendly empty state) and keep the generic load-error
  // card for every other failure — including a malformed 200-with-empty-
  // body, which `getAdminGroup` rejects as a thrown error.
  const isNotFound = group.isError && group.error instanceof HttpError && group.error.status === 404

  const groupName = group.data?.name ?? t("groupDetail.fallbackName")

  return (
    <>
      <RouteTitle title={groupName} />
      <div className="flex flex-col gap-6" data-testid="admin-group-detail-page">
        <Button
          variant="ghost"
          size="sm"
          asChild
          className="gap-1.5 -ml-2 self-start text-muted-foreground hover:text-foreground"
        >
          <Link to="/admin/groups">
            <ArrowLeft className="size-4" />
            {t("groupDetail.back")}
          </Link>
        </Button>

        {isNotFound ? (
          <div
            className="flex flex-col items-center justify-center gap-3 py-24"
            data-testid="admin-group-not-found"
          >
            <Layers className="size-8 text-muted-foreground/30" />
            <p className="text-sm text-muted-foreground">{t("groupDetail.notFound")}</p>
          </div>
        ) : group.isError ? (
          <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-6 text-sm text-destructive">
            {t("groupDetail.loadError")}
          </div>
        ) : group.isLoading || !group.data ? (
          <div className="rounded-xl border border-border bg-card p-6 text-sm text-muted-foreground">
            {t("groupDetail.loading")}
          </div>
        ) : (
          <GroupDetailBody group={group.data} />
        )}
      </div>
    </>
  )
}

// The loaded-state body: pending-deletion banner, header card, Members
// placeholder, and Danger zone. Split out so the loading / error / 404
// branches above stay flat.
function GroupDetailBody({ group }: { group: AdminGroupDetail }) {
  const { t } = useTranslation("admin")
  const isPendingDeletion = group.status === "pending_deletion"

  return (
    <>
      {isPendingDeletion ? (
        <Alert variant="destructive" data-testid="admin-group-pending-banner">
          <TriangleAlert />
          <AlertTitle>{t("groupDetail.pendingBanner.title")}</AlertTitle>
          <AlertDescription>{t("groupDetail.pendingBanner.body")}</AlertDescription>
        </Alert>
      ) : null}

      <GroupHeaderCard group={group} />
      <MembersPlaceholder memberCount={group.member_count ?? 0} />
      <DangerZone group={group} />
    </>
  )
}

// Read-only identity + metrics card for the group. Mirrors the design
// mock's header card (icon + name + status badge, tenant chip + currency
// metadata line) plus the issue's required metric grid.
function GroupHeaderCard({ group }: { group: AdminGroupDetail }) {
  const { t } = useTranslation("admin")
  const stats = [
    { label: t("groupDetail.header.currency"), value: group.currency || "—" },
    { label: t("groupDetail.header.members"), value: String(group.member_count ?? 0) },
    { label: t("groupDetail.header.createdBy"), value: group.created_by || "—" },
    {
      label: t("groupDetail.header.created"),
      value: group.created_at ? formatDate(group.created_at) : "—",
    },
  ]
  return (
    <div className="rounded-xl border border-border bg-card p-6" data-testid="admin-group-header">
      <div className="flex items-start gap-4">
        <div className="flex size-12 items-center justify-center rounded-xl bg-primary/10 shrink-0">
          <Layers className="size-6 text-primary" />
        </div>
        <div className="flex-1 min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <h1 className="text-2xl font-semibold tracking-tight">{group.name ?? "—"}</h1>
            <GroupStatusBadge status={group.status} />
          </div>
          <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-sm text-muted-foreground">
            <span className="inline-flex items-center gap-1.5">
              <Hash className="size-3.5" />
              <span className="font-mono text-xs">{group.slug ?? "—"}</span>
            </span>
            <span className="inline-flex max-w-48 items-center gap-1.5 rounded-full border border-border bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
              <Building2 className="size-3 shrink-0" />
              <span className="truncate">
                {group.tenant?.name || t("groupDetail.header.unknownTenant")}
              </span>
            </span>
          </div>
        </div>
      </div>

      <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
        {stats.map((s) => (
          <div key={s.label} className="rounded-lg border border-border bg-muted/40 px-3 py-2.5">
            <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
              {s.label}
            </p>
            <p className="mt-0.5 truncate text-sm font-semibold">{s.value}</p>
          </div>
        ))}
      </div>
    </div>
  )
}

// PLACEHOLDER Members panel. The real add/remove/role editor — which the
// design mock shows in full — ships in a later sub-issue (#1755 is
// list + detail + soft-delete only). This card makes the gap explicit so
// nobody mistakes it for an unfinished feature.
function MembersPlaceholder({ memberCount }: { memberCount: number }) {
  const { t } = useTranslation("admin")
  return (
    <div data-testid="admin-group-members-placeholder">
      <h2 className="mb-3 text-base font-semibold">{t("groupDetail.members.title")}</h2>
      <div className="rounded-xl border border-dashed border-border bg-card p-6">
        <div className="flex flex-col items-center justify-center gap-2 py-6 text-center">
          <Users className="size-8 text-muted-foreground/30" />
          <p className="text-sm font-medium">
            {t("groupDetail.members.count", { count: memberCount })}
          </p>
          <p className="text-sm text-muted-foreground">{t("groupDetail.members.placeholder")}</p>
        </div>
      </div>
    </div>
  )
}

// Danger zone — destructive-tinted card with the soft-delete action.
// Once the group is `pending_deletion` the button is disabled and the
// whole surface is read-only (the issue's idempotent re-delete is simply
// unreachable through the UI). The confirm dialog explains the two-phase
// nature: the group enters `pending_deletion` and the purge worker
// removes the data asynchronously.
//
// Confirmation uses the codebase's `useConfirm()` primitive (a root-mounted
// Dialog), not a per-call shadcn AlertDialog — the frontend has no
// AlertDialog primitive and `useConfirm` is the established destructive-
// confirm pattern. See devdocs/frontend/design-deviations.md.
function DangerZone({ group }: { group: AdminGroupDetail }) {
  const { t } = useTranslation("admin")
  const confirm = useConfirm()
  const deleteGroup = useDeleteAdminGroup()
  const isPendingDeletion = group.status === "pending_deletion"

  async function handleDelete() {
    if (!group.id) return
    const ok = await confirm({
      title: t("groupDetail.danger.dialog.title"),
      description: t("groupDetail.danger.dialog.body", { name: group.name ?? "" }),
      confirmLabel: t("groupDetail.danger.dialog.confirm"),
      cancelLabel: t("groupDetail.danger.dialog.cancel"),
      destructive: true,
    })
    if (!ok) return
    // The cache update in the hook flips the detail query to the returned
    // pending_deletion row, so the page re-renders read-only on its own.
    deleteGroup.mutate(group.id)
  }

  return (
    <div
      className="rounded-xl border border-destructive/30 bg-destructive/5 p-5"
      data-testid="admin-group-danger-zone"
    >
      <div className="flex items-start gap-3">
        <div className="flex size-9 items-center justify-center rounded-lg bg-destructive/10 shrink-0">
          <TriangleAlert className="size-4 text-destructive" />
        </div>
        <div className="flex-1 min-w-0">
          <h3 className="text-sm font-semibold">{t("groupDetail.danger.title")}</h3>
          <p className="mt-0.5 text-sm text-muted-foreground">
            {isPendingDeletion
              ? t("groupDetail.danger.descriptionPending")
              : t("groupDetail.danger.description")}
          </p>
        </div>
      </div>

      {deleteGroup.isError ? (
        <p className="mt-3 text-sm text-destructive" data-testid="admin-group-delete-error">
          {t("groupDetail.danger.deleteError")}
        </p>
      ) : null}

      <div className="mt-4">
        <Button
          size="sm"
          variant="destructive"
          className="gap-1.5"
          disabled={isPendingDeletion || deleteGroup.isPending}
          onClick={handleDelete}
          data-testid="admin-group-delete-button"
        >
          <Trash2 className="size-3.5" />
          {isPendingDeletion
            ? t("groupDetail.danger.deletionPending")
            : t("groupDetail.danger.deleteButton")}
        </Button>
      </div>
    </div>
  )
}
