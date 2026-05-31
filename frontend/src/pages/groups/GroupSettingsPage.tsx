import { useEffect, useMemo, useState, type ReactNode } from "react"
import { useForm, Controller } from "react-hook-form"
import { zodResolver } from "@hookform/resolvers/zod"
import { useTranslation } from "react-i18next"
import { Link, Navigate, useNavigate, useParams } from "react-router-dom"
import {
  ArrowRight,
  ArrowRightLeft,
  Bell,
  ChevronRight,
  Database,
  Download,
  History,
  Info,
  LogOut,
  ShieldAlert,
  Trash2,
  Users,
} from "lucide-react"

import { CurrencyMigrationsList } from "@/components/groups/CurrencyMigrationsList"
import { IconPicker } from "@/components/groups/IconPicker"
import { MigrateCurrencyDialog } from "@/components/groups/MigrateCurrencyDialog"
import { NotificationsCard } from "@/components/groups/NotificationsCard"
import { PlanCard } from "@/components/groups/PlanCard"
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
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Page, PageHeader } from "@/components/ui/page"
import { StorageCard } from "@/features/storage/StorageCard"
import { useAuth } from "@/features/auth/AuthContext"
import { useCurrencyMigrations } from "@/features/currency-migration/hooks"
import { useFeatureFlag } from "@/features/feature-flags/hooks"
import {
  useDeleteGroup,
  useGroup,
  useLeaveGroup,
  useMembers,
  useUpdateGroup,
} from "@/features/group/hooks"
import {
  deleteGroupSchema,
  updateGroupSchema,
  type DeleteGroupInput,
  type UpdateGroupInput,
} from "@/features/group/schemas"
import { useAppToast } from "@/hooks/useAppToast"
import { HttpError } from "@/lib/http"
import { applyServerFieldErrors, shouldShowGenericError } from "@/lib/form-errors"
import { getServerErrorCode, parseServerError } from "@/lib/server-error"
import { cn } from "@/lib/utils"
import { RouteTitle } from "@/components/routing/RouteTitle"

type SectionId = "info" | "members" | "notifications" | "data" | "management"

interface SectionMeta {
  id: SectionId
  icon: typeof Info
}

const SECTIONS: SectionMeta[] = [
  { id: "info", icon: Info },
  { id: "members", icon: Users },
  { id: "notifications", icon: Bell },
  { id: "data", icon: Database },
  { id: "management", icon: ShieldAlert },
]

// /groups/:groupId/settings — group admin panel split across five
// sub-sections behind a left rail (mirrors the user Preferences pattern
// at /settings):
//   - Info           identity (name, icon, slug, currency + migrate)
//                    + Plan & quota card (#1887: moved out of the
//                    page header so it no longer renders everywhere)
//   - Members        members link + leave-group panel
//   - Notifications  per-group notification preference toggles
//                    (#1887: pulled out of the page header into its
//                    own sub-section)
//   - Data           per-group storage usage + exports shortcut
//   - Management     destructive actions (delete group)
//
// Group `id` (UUID) is the path key here, not the slug — slugs are
// random and not in URLs that admin tools reach for. Sub-pages that
// need the slug (members, exports) compose it from the loaded group.
export function GroupSettingsPage() {
  const { groupId } = useParams<{ groupId: string }>()
  if (!groupId) return <Navigate to="/no-group" replace />
  return <GroupSettingsBody groupId={groupId} />
}

function GroupSettingsBody({ groupId }: { groupId: string }) {
  const { t } = useTranslation()
  const { user } = useAuth()
  const groupQuery = useGroup(groupId)
  const membersQuery = useMembers(groupId)
  const [active, setActive] = useState<SectionId>("info")

  const myMembership = useMemo(
    () => membersQuery.data?.find((m) => m.member_user_id === user?.id),
    [membersQuery.data, user?.id]
  )
  // Owner identity for the Plan card subtitle. There can be more than
  // one owner on a group (post-#1533 promotion flow); we surface the
  // first owner row's display name as a pragmatic v1 — the mock shows
  // a single owner line and the card doesn't have room for a list.
  // Falls back to null so PlanCard renders "Owner: —" while members
  // are loading or in the rare case the group has no owner yet (purge
  // window).
  const ownerName = useMemo(() => {
    const owner = membersQuery.data?.find((m) => m.role === "owner")
    return owner?.user?.name ?? null
  }, [membersQuery.data])
  // Post-#1533 role taxonomy: admin / owner share admin-tier
  // capabilities; the ≥1 invariant moves from "≥1 admin" to "≥1 owner"
  // because only owners can delete the group. The leave-group flow
  // therefore guards on owners (the role you'd strand the group of by
  // leaving), not on the broader admin tier.
  const isAdmin = myMembership?.role === "admin" || myMembership?.role === "owner"
  const isOwner = myMembership?.role === "owner"
  const ownerCount = useMemo(
    () => membersQuery.data?.filter((m) => m.role === "owner").length ?? 0,
    [membersQuery.data]
  )
  // Post-#1533 this guard is "user is the sole owner" — leaving in
  // that state would strand the group without anyone able to delete
  // it. Name follows the new semantics; the test-id ("last-admin-notice")
  // is kept for compatibility with the existing e2e/test selectors.
  const isLastOwner = isOwner && ownerCount === 1

  if (groupQuery.isLoading) {
    return (
      <Page width="wide">
        <p className="text-sm text-muted-foreground">{t("groups:settings.title")}…</p>
      </Page>
    )
  }
  if (groupQuery.isError || !groupQuery.data) {
    return (
      <Page width="wide">
        <Alert variant="destructive">
          <AlertDescription>{t("groups:settings.errorGeneric")}</AlertDescription>
        </Alert>
      </Page>
    )
  }

  const group = groupQuery.data

  return (
    <>
      <RouteTitle title={t("groups:settings.title")} />
      <Page width="wide" className="gap-8" data-testid="group-settings-page">
        <PageHeader
          title={
            <>
              {group.icon ? <span aria-hidden="true">{group.icon} </span> : null}
              {group.name}
            </>
          }
          subtitle={t("groups:settings.subtitle")}
        />

        <div className="flex flex-col gap-6 md:flex-row">
          <GroupSettingsNav active={active} onSelect={setActive} />
          <div className="min-w-0 flex-1">
            {active === "info" ? (
              <InfoSection
                groupId={groupId}
                groupSlug={group.slug ?? null}
                isAdmin={isAdmin}
                ownerName={ownerName}
              />
            ) : null}
            {active === "members" ? (
              <MembersSection
                groupId={groupId}
                groupSlug={group.slug ?? null}
                isLastOwner={isLastOwner}
                membersLoading={membersQuery.isLoading}
              />
            ) : null}
            {active === "notifications" ? (
              <NotificationsSection groupSlug={group.slug ?? null} />
            ) : null}
            {active === "data" ? <DataSection groupSlug={group.slug ?? null} /> : null}
            {active === "management" ? (
              <ManagementSection groupId={groupId} groupName={group.name ?? ""} isAdmin={isAdmin} />
            ) : null}
          </div>
        </div>
      </Page>
    </>
  )
}

function GroupSettingsNav({
  active,
  onSelect,
}: {
  active: SectionId
  onSelect: (id: SectionId) => void
}) {
  const { t } = useTranslation()

  return (
    <aside className="md:w-48 md:shrink-0">
      <nav className="space-y-0.5" aria-label={t("groups:settings.title")}>
        {SECTIONS.map(({ id, icon: Icon }) => (
          <button
            key={id}
            type="button"
            onClick={() => onSelect(id)}
            data-testid={`group-settings-nav-${id}`}
            data-active={active === id ? "true" : undefined}
            className={cn(
              "flex w-full items-center gap-2.5 rounded-md px-3 py-2 text-sm transition-colors",
              active === id
                ? "bg-accent text-accent-foreground font-medium"
                : "text-muted-foreground hover:bg-muted hover:text-foreground"
            )}
            aria-current={active === id ? "page" : undefined}
          >
            <Icon className="size-4 shrink-0" aria-hidden="true" />
            {t(`groups:settings.sections.${id}`)}
            {active === id ? (
              <ChevronRight className="ml-auto size-3.5" aria-hidden="true" />
            ) : null}
          </button>
        ))}
      </nav>
    </aside>
  )
}

function SectionTitle({ children }: { children: ReactNode }) {
  return <h2 className="mb-4 text-base font-semibold">{children}</h2>
}

// InfoSection — identity (name, icon, slug, currency) + currency
// migration controls. Admin-only edit form; non-admins see a read-only
// notice. The Plan & quota card (#1389) lives here too — moved out of the
// per-page header (#1887) so it no longer renders on every settings
// sub-page. PlanCard is read-only identity, so it renders for non-admins
// as well.
// TODO(#1647): description field (Textarea) once BE lands `description` on
// LocationGroup. Belongs below the icon picker (mock parity with Location +
// Area description fields).
function InfoSection({
  groupId,
  groupSlug,
  isAdmin,
  ownerName,
}: {
  groupId: string
  groupSlug: string | null
  isAdmin: boolean
  ownerName: string | null
}) {
  const { t } = useTranslation()
  const groupQuery = useGroup(groupId)
  const updateMutation = useUpdateGroup()
  const toast = useAppToast()
  const [serverError, setServerError] = useState<string | null>(null)
  const [migrateOpen, setMigrateOpen] = useState(false)
  const [historyOpen, setHistoryOpen] = useState(false)
  // Deployment kill-switch — when the backend has the currency-migration
  // feature gated off (#1616) we don't render the CTA, the history sheet,
  // or the background list query. The flag is fail-closed: while the
  // /feature-flags request is in flight we keep the UI hidden so a slow
  // boot can't flash a button that 404s on click.
  const currencyMigrationEnabled = useFeatureFlag("currency_migration")
  // Migrations list: only fetch when a group is loaded AND the feature
  // is on. The group's `slug` is required because /groups/:groupId/
  // settings has no :groupSlug URL param, so the http rewrite slot is
  // empty here — the API takes the slug explicitly and builds
  // /g/${slug}/currency-migrations itself.
  const migrationsQuery = useCurrencyMigrations(groupSlug ?? "", {
    enabled: !!groupQuery.data && currencyMigrationEnabled && !!groupSlug,
  })
  const migrations = migrationsQuery.data?.migrations ?? []
  const migrationInFlightId = groupQuery.data?.currency_migration_id

  // If the feature flag flips off between renders (operator pushed a
  // re-deploy while the user had the wizard or history sheet open), the
  // gated JSX below stops mounting — but the `open` state we kept here
  // would still read as true. Reset both so a subsequent flag-on flip
  // doesn't re-open a sheet the user already closed-by-proxy.
  useEffect(() => {
    if (!currencyMigrationEnabled) {
      setMigrateOpen(false)
      setHistoryOpen(false)
    }
  }, [currencyMigrationEnabled])

  const form = useForm<UpdateGroupInput>({
    resolver: zodResolver(updateGroupSchema),
    defaultValues: { name: "", icon: "" },
  })

  // Reset the form once the group lands. useForm reads defaults at
  // mount; a hard refresh races the GET /groups/:id round-trip.
  useEffect(() => {
    if (!groupQuery.data) return
    form.reset({
      name: groupQuery.data.name ?? "",
      icon: groupQuery.data.icon ?? "",
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [groupQuery.data?.id, groupQuery.data?.updated_at])

  useEffect(() => {
    const sub = form.watch(() => {
      if (serverError) setServerError(null)
    })
    return () => sub.unsubscribe()
  }, [form, serverError])

  if (!groupQuery.data) return null
  const group = groupQuery.data

  async function onSave(values: UpdateGroupInput) {
    setServerError(null)
    try {
      await updateMutation.mutateAsync({
        groupId,
        patch: { name: values.name.trim(), icon: values.icon },
      })
      toast.success(t("groups:settings.saved"))
    } catch (err) {
      const fieldResult = applyServerFieldErrors(err, form.setError, {
        fields: Object.keys(updateGroupSchema.shape),
      })
      setServerError(
        shouldShowGenericError(fieldResult)
          ? parseServerError(err, t("groups:settings.errorGeneric"))
          : null
      )
    }
  }

  if (!isAdmin) {
    return (
      <div className="space-y-6" data-testid="group-section-info">
        <SectionTitle>{t("groups:settings.sections.info")}</SectionTitle>
        {/* Plan & quota lives in Info (#1887). It's read-only identity,
            not an admin-gated action, so non-admins see it too. */}
        <PlanCard groupSlug={groupSlug} ownerName={ownerName} />
        <div className="rounded-xl border border-border bg-muted/30 p-5 text-sm text-muted-foreground">
          {t("members:adminOnlyHelp")}
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6" data-testid="group-section-info">
      <SectionTitle>{t("groups:settings.sections.info")}</SectionTitle>

      {/* Plan & quota (#1389). Lives at the top of Info (#1887) so it
          no longer renders on every settings sub-page. */}
      <PlanCard groupSlug={groupSlug} ownerName={ownerName} />

      <form
        className="space-y-4 rounded-xl border border-border bg-card p-5"
        onSubmit={form.handleSubmit(onSave)}
        noValidate
      >
        <div className="space-y-1.5">
          <Label htmlFor="settings-group-name">{t("groups:settings.nameLabel")}</Label>
          <Input
            id="settings-group-name"
            maxLength={100}
            disabled={updateMutation.isPending}
            aria-invalid={!!form.formState.errors.name}
            data-testid="settings-name-input"
            {...form.register("name")}
          />
          {form.formState.errors.name ? (
            <p className="text-xs text-destructive" data-testid="settings-name-error">
              {t(form.formState.errors.name.message ?? "")}
            </p>
          ) : null}
        </div>

        <div className="space-y-1.5">
          <Label>{t("groups:settings.iconLabel")}</Label>
          <Controller
            control={form.control}
            name="icon"
            render={({ field }) => (
              <IconPicker
                value={field.value}
                onChange={field.onChange}
                disabled={updateMutation.isPending}
                testId="group-settings-icon-picker"
              />
            )}
          />
          {form.formState.errors.icon ? (
            <p className="text-xs text-destructive" data-testid="settings-icon-error">
              {t(form.formState.errors.icon.message ?? "")}
            </p>
          ) : null}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="settings-group-slug">{t("groups:settings.slugLabel")}</Label>
          <Input
            id="settings-group-slug"
            value={group.slug ?? ""}
            readOnly
            disabled
            className="font-mono text-xs"
          />
          <p className="text-[11px] text-muted-foreground">{t("groups:settings.slugHelp")}</p>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="settings-group-currency">{t("groups:settings.currencyLabel")}</Label>
          <div className="flex gap-2">
            <Input
              id="settings-group-currency"
              value={group.group_currency ?? "—"}
              readOnly
              disabled
              className="font-mono uppercase"
            />
            {/* Migrate-currency CTA: visible only when the deployment
                has the currency-migration feature enabled (#1616). When
                off, the BE 404s the wizard endpoints — showing the CTA
                anyway is the silent-failure UX bug we're fixing. */}
            {currencyMigrationEnabled ? (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="shrink-0 gap-1.5"
                // The BE blocks a second migration on the same group at the
                // start handler with 409 migration_in_progress. We mirror
                // that as a disabled CTA driven by the group's own
                // currency_migration_id (read-only on JSON:API; the
                // migration registry sets it). The 409 is the safety net
                // for the race between this read and the click.
                disabled={!!migrationInFlightId}
                title={migrationInFlightId ? t("errors:lockedDuringMigration") : undefined}
                aria-disabled={!!migrationInFlightId || undefined}
                onClick={() => setMigrateOpen(true)}
                data-testid="migrate-currency-open"
              >
                <ArrowRightLeft className="size-3.5" aria-hidden="true" />
                {t("groups:settings.migrateCurrency")}
              </Button>
            ) : null}
          </div>
          <div className="flex flex-wrap items-center justify-between gap-2 pt-0.5">
            <p className="text-[11px] text-muted-foreground">
              {currencyMigrationEnabled
                ? t("groups:settings.migrateCurrencyHelp")
                : t("groups:settings.migrateCurrencyDisabledHelp")}
            </p>
            {/* History link mounts only after at least one migration
                has been started — there's nothing useful behind it
                on a fresh group. Opens a right-side Sheet instead
                of inlining the list, since this is reference data
                a user opens occasionally, not the primary content
                of the page. Hidden along with the CTA when the
                feature is gated off in this deployment. */}
            {currencyMigrationEnabled && migrations.length > 0 ? (
              <button
                type="button"
                onClick={() => setHistoryOpen(true)}
                className="inline-flex items-center gap-1 text-[11px] text-muted-foreground underline-offset-2 hover:text-foreground hover:underline transition-colors"
                data-testid="migrations-history-open"
              >
                <History className="size-3" aria-hidden="true" />
                {t("groups:settings.migrationsHistoryCta", {
                  count: migrations.length,
                })}
              </button>
            ) : null}
          </div>
        </div>

        {serverError ? (
          <Alert variant="destructive" data-testid="settings-server-error">
            <AlertDescription>{serverError}</AlertDescription>
          </Alert>
        ) : null}

        <div className="flex justify-end pt-2">
          <Button
            type="submit"
            className="gap-2"
            disabled={updateMutation.isPending}
            data-testid="settings-save"
          >
            {updateMutation.isPending ? t("groups:settings.saving") : t("groups:settings.save")}
            {!updateMutation.isPending ? <ArrowRight className="size-4" /> : null}
          </Button>
        </div>
      </form>

      {currencyMigrationEnabled && group.group_currency && groupSlug ? (
        <MigrateCurrencyDialog
          open={migrateOpen}
          onOpenChange={setMigrateOpen}
          groupName={group.name ?? ""}
          fromCurrency={group.group_currency}
          groupSlug={groupSlug}
        />
      ) : null}
      <Sheet open={historyOpen} onOpenChange={setHistoryOpen}>
        <SheetContent
          side="right"
          className="w-full sm:max-w-md flex flex-col gap-4 overflow-y-auto p-6"
          data-testid="migrations-history-sheet"
        >
          <SheetHeader className="p-0">
            <SheetTitle>{t("groups:settings.migrationsTitle")}</SheetTitle>
            <SheetDescription>{t("groups:settings.migrationsHelp")}</SheetDescription>
          </SheetHeader>
          <CurrencyMigrationsList loading={migrationsQuery.isLoading} migrations={migrations} />
        </SheetContent>
      </Sheet>
    </div>
  )
}

// MembersSection — short list link out to /g/<slug>/members plus the
// leave-group control. Both render for non-admins too: viewing the
// member list is unrestricted, and a non-admin can always leave —
// only the sole owner is blocked (and only on the BE).
function MembersSection({
  groupId,
  groupSlug,
  isLastOwner,
  membersLoading,
}: {
  groupId: string
  groupSlug: string | null
  isLastOwner: boolean
  membersLoading: boolean
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const leaveMutation = useLeaveGroup()
  const toast = useAppToast()

  async function onLeave() {
    try {
      await leaveMutation.mutateAsync({ groupId })
      toast.success(t("groups:settings.leaveSuccess"))
      navigate("/no-group")
    } catch (err) {
      // #1652: distinct copy for the ≥1-member invariant. Without
      // the code-match the toast collapses to the generic
      // "Couldn't leave" path, which hides the actionable hint
      // ("delete the group instead"). leaveLastAdmin already covers
      // the sole-owner case via the disabled button + inline notice
      // above, but we still surface a specific toast for the rare
      // race where the gate is bypassed (membership query loading,
      // role-data drift, etc.).
      if (getServerErrorCode(err) === "group.last_member") {
        toast.error(t("groups:settings.leaveLastMember"))
        return
      }
      toast.error(parseServerError(err, t("groups:settings.leaveError")))
    }
  }

  return (
    <div className="space-y-6" data-testid="group-section-members">
      <SectionTitle>{t("groups:settings.sections.members")}</SectionTitle>

      {/* Members shortcut — chevron-right divide-y row (mock parity with
          design-mocks/.../GroupSettingsView.tsx Data card). Works for non-
          admins too (they see the list; actions are gated inside MembersPage).
          Horizontal padding sits on the <Link> (not the wrapper) so the full
          card width is a hit target; overflow-hidden clips the hover bg to
          the wrapper's rounded corners. */}
      {groupSlug ? (
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          <div className="divide-y divide-border">
            <Link
              to={`/g/${encodeURIComponent(groupSlug)}/members`}
              data-testid="settings-members-link"
              className="flex w-full items-center gap-3 px-4 py-3.5 text-left text-sm font-medium hover:bg-muted/50 transition-colors"
            >
              <Users className="size-4 text-muted-foreground shrink-0" aria-hidden="true" />
              <span className="flex-1">{t("groups:settings.membersLink")}</span>
              <ChevronRight className="size-4 text-muted-foreground" aria-hidden="true" />
            </Link>
          </div>
        </div>
      ) : null}

      {/* Leave-group panel. The BE rejects "leave as last owner" with
          a 422; we mirror that as a disabled button + explanation so
          the user doesn't waste a round-trip. The `last-admin-notice`
          test-id is kept for e2e selector compatibility — renaming it
          would churn an unrelated test surface for no behavior gain. */}
      <div className="rounded-xl border border-border bg-card p-5 space-y-3">
        <div>
          <p className="text-sm font-semibold">{t("groups:settings.leaveTitle")}</p>
          <p
            className="text-xs text-muted-foreground mt-0.5"
            data-testid={isLastOwner ? "last-admin-notice" : undefined}
          >
            {isLastOwner
              ? t("groups:settings.leaveLastAdmin")
              : t("groups:settings.leaveDescription")}
          </p>
        </div>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="gap-1.5 text-amber-600 border-amber-500/40 hover:bg-amber-500/10"
          // Also gate on the membership query: while it's loading,
          // ownerCount defaults to 0 and isLastOwner is false, so the
          // guard would briefly let the click through. The BE rejects
          // with 422 anyway, but the UX is cleaner if the button stays
          // unclickable until we know the answer.
          disabled={membersLoading || isLastOwner || leaveMutation.isPending}
          aria-disabled={isLastOwner || undefined}
          title={isLastOwner ? t("groups:settings.leaveLastAdminTitle") : undefined}
          onClick={onLeave}
          data-testid="leave-group-btn"
        >
          <LogOut className="size-3.5" aria-hidden="true" />
          {t("groups:settings.leaveCta")}
        </Button>
      </div>
    </div>
  )
}

// NotificationsSection — per-group notification preferences (#1648),
// pulled out of the per-page header (#1887) so users only see the
// toggles when they navigate here. The card already carries its own
// title + subtitle, so we drop the SectionTitle to avoid double
// headings.
function NotificationsSection({ groupSlug }: { groupSlug: string | null }) {
  return (
    <div className="space-y-6" data-testid="group-section-notifications">
      <NotificationsCard groupSlug={groupSlug} />
    </div>
  )
}

// DataSection — per-group storage usage panel + exports shortcut.
// Moved here from /settings (user Preferences), where these were
// rendered against an implicit "active group" fallback. Both are
// strictly group-scoped, so they belong on the group page.
function DataSection({ groupSlug }: { groupSlug: string | null }) {
  const { t } = useTranslation()

  return (
    <div className="space-y-6" data-testid="group-section-data">
      <SectionTitle>{t("groups:settings.data.title")}</SectionTitle>

      {/* Export shortcut — chevron-right divide-y row (mock parity with
          design-mocks/.../GroupSettingsView.tsx Data card). The destination
          page carries the long-form description, so we drop it here in
          favour of a clean nav row. Padding/hit-target/hover handled the
          same way as the Members shortcut above. */}
      {groupSlug ? (
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          <div className="divide-y divide-border">
            <Link
              to={`/g/${encodeURIComponent(groupSlug)}/exports`}
              data-testid="settings-open-exports"
              className="flex w-full items-center gap-3 px-4 py-3.5 text-left text-sm font-medium hover:bg-muted/50 transition-colors"
            >
              <Download className="size-4 text-muted-foreground shrink-0" aria-hidden="true" />
              <span className="flex-1">{t("groups:settings.data.exportTitle")}</span>
              <ChevronRight className="size-4 text-muted-foreground" aria-hidden="true" />
            </Link>
          </div>
        </div>
      ) : (
        <div className="rounded-xl border border-border bg-card p-4">
          <p className="text-xs text-muted-foreground">{t("groups:settings.data.noGroupSlug")}</p>
        </div>
      )}

      <StorageCard />
    </div>
  )
}

// ManagementSection — destructive group lifecycle actions. Admin-only;
// non-admins see an empty-state explainer instead.
function ManagementSection({
  groupId,
  groupName,
  isAdmin,
}: {
  groupId: string
  groupName: string
  isAdmin: boolean
}) {
  const { t } = useTranslation()
  const [deleteOpen, setDeleteOpen] = useState(false)

  if (!isAdmin) {
    return (
      <div className="space-y-6" data-testid="group-section-management">
        <SectionTitle>{t("groups:settings.sections.management")}</SectionTitle>
        <div className="rounded-xl border border-border bg-muted/30 p-5 text-sm text-muted-foreground">
          {t("members:adminOnlyHelp")}
        </div>
      </div>
    )
  }

  return (
    <div className="space-y-6" data-testid="group-section-management">
      <SectionTitle>{t("groups:settings.sections.management")}</SectionTitle>

      <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-5 space-y-3">
        <p className="text-sm font-semibold text-destructive">{t("groups:settings.dangerTitle")}</p>
        <p className="text-xs text-muted-foreground">{t("groups:settings.dangerDescription")}</p>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="gap-1.5 text-destructive border-destructive/40 hover:bg-destructive/10"
          onClick={() => setDeleteOpen(true)}
          data-testid="delete-group-open"
        >
          <Trash2 className="size-3.5" aria-hidden="true" />
          {t("groups:settings.deleteCta")}
        </Button>
      </div>

      <DeleteGroupDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        group={{ id: groupId, name: groupName }}
      />
    </div>
  )
}

function DeleteGroupDialog({
  open,
  onOpenChange,
  group,
}: {
  open: boolean
  onOpenChange: (next: boolean) => void
  group: { id: string; name: string }
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const deleteMutation = useDeleteGroup()
  const [serverError, setServerError] = useState<string | null>(null)

  const form = useForm<DeleteGroupInput>({
    resolver: zodResolver(deleteGroupSchema),
    defaultValues: { confirmWord: "", password: "" },
  })

  // Reset the form whenever the dialog opens — we don't want a stale
  // password lingering across re-opens.
  useEffect(() => {
    // Sync external `open` prop → form state + serverError.
    if (open) {
      form.reset({ confirmWord: "", password: "" })
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setServerError(null)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps -- intentionally only re-run on open changes; form is a stable RHF handle
  }, [open])

  async function onSubmit(values: DeleteGroupInput) {
    setServerError(null)
    // Client-side guard: confirm_word must match the group's current
    // name. The server checks this too, but catching it here saves a
    // round-trip on the very common typo case.
    if (values.confirmWord.trim() !== group.name) {
      form.setError("confirmWord", { message: "groups:validation.confirmWordMismatch" })
      return
    }
    try {
      await deleteMutation.mutateAsync({
        groupId: group.id,
        confirm_word: values.confirmWord.trim(),
        password: values.password,
      })
      onOpenChange(false)
      navigate("/no-group")
    } catch (err) {
      // The client-side guard above already rejected mismatched confirm-words
      // before the request was sent, so a 422 from the BE here can only mean
      // the password was wrong. Surface that on the password field directly
      // (#1289 Gap A: wrong password must be distinguishable from wrong
      // confirm-word in the UX, not just at the handler).
      if (err instanceof HttpError && err.status === 422) {
        // Pre-translate so the i18next-cli extractor sees the key statically
        // (the form renderer feeds errors.password.message back through t(),
        // but t() of an already-translated string is a no-op lookup that
        // returns the same string — which is what we want here).
        form.setError("password", {
          message: t("groups:settings.deleteDialog.wrongPassword"),
        })
        return
      }
      setServerError(parseServerError(err, t("groups:settings.deleteDialog.errorGeneric")))
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent data-testid="delete-group-dialog">
        <DialogHeader>
          <DialogTitle>{t("groups:settings.deleteDialog.title", { name: group.name })}</DialogTitle>
          <DialogDescription>{t("groups:settings.deleteDialog.body")}</DialogDescription>
        </DialogHeader>
        <form className="space-y-4" onSubmit={form.handleSubmit(onSubmit)} noValidate>
          <div className="space-y-1.5">
            <Label htmlFor="delete-group-name">
              {t("groups:settings.deleteDialog.confirmWordLabel")}
            </Label>
            <Input
              id="delete-group-name"
              autoComplete="off"
              placeholder={group.name}
              disabled={deleteMutation.isPending}
              aria-invalid={!!form.formState.errors.confirmWord}
              data-testid="delete-confirm-word"
              {...form.register("confirmWord")}
            />
            {form.formState.errors.confirmWord ? (
              <p className="text-xs text-destructive" data-testid="delete-confirm-word-error">
                {t(form.formState.errors.confirmWord.message ?? "")}
              </p>
            ) : null}
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="delete-group-password">
              {t("groups:settings.deleteDialog.passwordLabel")}
            </Label>
            <Input
              id="delete-group-password"
              type="password"
              autoComplete="current-password"
              disabled={deleteMutation.isPending}
              aria-invalid={!!form.formState.errors.password}
              data-testid="delete-password"
              {...form.register("password")}
            />
            {form.formState.errors.password ? (
              <p className="text-xs text-destructive" data-testid="delete-password-error">
                {t(form.formState.errors.password.message ?? "")}
              </p>
            ) : null}
          </div>
          {serverError ? (
            <Alert variant="destructive" data-testid="delete-server-error">
              <AlertDescription>{serverError}</AlertDescription>
            </Alert>
          ) : null}
          <DialogFooter>
            <Button
              type="button"
              variant="ghost"
              onClick={() => onOpenChange(false)}
              disabled={deleteMutation.isPending}
            >
              {t("groups:settings.deleteDialog.cancel")}
            </Button>
            <Button
              type="submit"
              variant="destructive"
              disabled={deleteMutation.isPending}
              data-testid="delete-group-submit"
            >
              {deleteMutation.isPending
                ? t("groups:settings.deleteDialog.deleting")
                : t("groups:settings.deleteDialog.confirm")}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
