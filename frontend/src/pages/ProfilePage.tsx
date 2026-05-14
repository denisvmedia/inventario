import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"
import {
  Building2,
  Calendar,
  Mail,
  Package,
  Pencil,
  ShieldCheck,
  TrendingUp,
  Users,
  Zap,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { ComingSoonBanner } from "@/components/coming-soon"
import { RouteTitle } from "@/components/routing/RouteTitle"
import { useAuth } from "@/features/auth/AuthContext"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useGroups } from "@/features/group/hooks"
import { useDashboardData } from "@/features/dashboard/hooks"
import type { LocationGroup } from "@/features/group/api"
import type { GroupRole } from "@/features/group/api"
import { withGroupQuery } from "@/lib/group-aware-url"
import { formatCurrency, formatDate } from "@/lib/intl"

// Cap the Groups tab grid at MAX_GROUP_TILES; the "+N more" affordance
// matches the design mock's overflow pattern. Picked so the grid stays
// at or below 3 visual rows on a 2-column layout — beyond that we'd be
// better off sending the user to a dedicated "all groups" surface.
const MAX_GROUP_TILES = 6

// Initials for the avatar fallback. "Alex Johnson" → "AJ", "alex" → "A".
// We keep at most two letters and fall back to the email's local part if
// `name` isn't available, then to "?" so the badge never reads as empty.
function initialsFor(name?: string, email?: string): string {
  const source = name?.trim() || (email?.split("@")[0] ?? "")
  if (!source) return "?"
  const parts = source.split(/\s+/).filter(Boolean)
  const letters = (parts[0]?.[0] ?? "") + (parts[parts.length - 1]?.[0] ?? "")
  return (letters || source[0] || "?").toUpperCase().slice(0, 2)
}

export function ProfilePage() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const { data: groups } = useGroups()
  const { currentGroup } = useCurrentGroup()
  const dashboard = useDashboardData()

  const derivedName = user?.name?.trim() || (user?.email?.split("@")[0] ?? "")
  const name = derivedName || "—"
  const email = user?.email ?? t("settings:profile.noEmail")
  const memberSince = user?.created_at && formatDate(user.created_at, { style: "long" })
  const defaultGroup =
    user?.default_group_id && groups
      ? (groups.find((g) => g.id === user.default_group_id) ?? null)
      : null

  // Stat snapshot pulls from the active group's dashboard data (#1653
  // acceptance criteria). The dash placeholder covers three branches: no
  // active group, the dashboard query still settling, OR a fetch error —
  // we'd rather render a stable "—" than a misleading "0 items / $0" when
  // the API is actually unreachable.
  const hasGroupContext = !!currentGroup
  const groupCurrency = currentGroup?.group_currency ?? "USD"
  const statsReady = hasGroupContext && !dashboard.isLoading && !dashboard.isError
  const dash = "—"
  const statValues = {
    items: statsReady ? String(dashboard.totalItems) : dash,
    activeWarranties: statsReady ? String(dashboard.warrantyStatusCounts.active) : dash,
    expiringSoon: statsReady ? String(dashboard.warrantyStatusCounts.expiring) : dash,
    estValue: statsReady ? formatCurrency(dashboard.totalValue, groupCurrency) : dash,
  }

  // Distinguish "still loading" from "loaded and empty" so the Groups tab
  // doesn't flash the empty state during the /groups round-trip — and so
  // the e2e harness doesn't catch the flicker mid-fetch.
  const groupsLoading = groups === undefined
  const visibleGroups = (groups ?? []).slice(0, MAX_GROUP_TILES)
  const overflowGroups = Math.max(0, (groups?.length ?? 0) - MAX_GROUP_TILES)

  return (
    <>
      <RouteTitle title={t("settings:profile.title")} />
      <div className="mx-auto flex w-full max-w-3xl flex-col gap-8 p-6" data-testid="profile-page">
        {/* Identity card. Banner + avatar + name + plan badge. The cover
            stripe pattern is taken from the design mock; the avatar shows
            initials because uploads are tracked under #1382. */}
        <section className="overflow-hidden rounded-2xl border border-border">
          <div className="relative h-28 overflow-hidden bg-primary">
            <div
              aria-hidden="true"
              className="absolute inset-0 opacity-[0.07]"
              style={{
                backgroundImage:
                  "repeating-linear-gradient(-45deg, currentColor 0, currentColor 1px, transparent 1px, transparent 10px)",
              }}
            />
            <div
              aria-hidden="true"
              className="absolute -bottom-6 -left-6 size-36 rounded-full bg-primary-foreground/10 blur-2xl"
            />
            <div
              aria-hidden="true"
              className="absolute -top-4 right-12 size-24 rounded-full bg-primary-foreground/5 blur-xl"
            />
            <Button
              asChild
              variant="secondary"
              size="sm"
              className="absolute top-3 right-3 gap-1.5 bg-background/80 shadow-sm backdrop-blur-sm hover:bg-background/95"
            >
              <Link
                to={withGroupQuery("/profile/edit", currentGroup?.slug)}
                data-testid="profile-edit-link"
              >
                <Pencil className="size-3.5" />
                {t("settings:profile.editProfile")}
              </Link>
            </Button>
          </div>

          <div className="px-5 pb-5">
            <div className="-mt-9 mb-3 flex items-end justify-between">
              {/* The mock wraps the avatar in a `relative group` so the
                  camera-upload overlay can sit inside on hover (#1382).
                  We keep the wrapper even without the overlay so the
                  alignment math (avatar bottom vs. plan-cluster bottom)
                  matches the mock exactly. */}
              <div className="relative group">
                <div
                  className="flex size-[72px] items-center justify-center rounded-2xl border-4 border-background bg-card text-xl font-bold text-primary shadow-md"
                  aria-hidden="true"
                >
                  {initialsFor(user?.name, user?.email)}
                </div>
              </div>
              <div className="mb-1 flex items-center gap-2">
                <Badge variant="outline" className="text-xs font-medium">
                  {t("settings:profile.planFree")}
                </Badge>
                <Button asChild variant="outline" size="sm" className="h-7 gap-1.5 px-2.5 text-xs">
                  <Link
                    to={withGroupQuery("/plans", currentGroup?.slug)}
                    data-testid="profile-upgrade-link"
                  >
                    <Zap className="size-3" />
                    {t("settings:profile.upgrade")}
                  </Link>
                </Button>
              </div>
            </div>

            <div className="mb-4 space-y-0.5">
              {/* H1 is a stable page title ("My Profile") so the route's
                  accessible name doesn't shift with whatever name the user
                  has set — the live identity (name + email) sits below as
                  the actual headline content. */}
              <h1 className="sr-only">{t("settings:profile.heading")}</h1>
              <p className="text-xl font-bold tracking-tight" data-testid="profile-name">
                {name}
              </p>
              <p className="text-sm text-muted-foreground" data-testid="profile-email">
                {email}
              </p>
            </div>

            <div className="flex flex-wrap gap-x-4 gap-y-1.5">
              {memberSince ? (
                <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <Calendar className="size-3.5" aria-hidden="true" />
                  <span>{t("settings:profile.memberSince", { date: memberSince })}</span>
                </div>
              ) : null}
              <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                <Building2 className="size-3.5" aria-hidden="true" />
                <span data-testid="profile-default-group">
                  {defaultGroup?.name
                    ? `${t("settings:profile.defaultGroup")}: ${defaultGroup.name}`
                    : t("settings:profile.noGroupSet")}
                </span>
              </div>
              {email ? (
                <div className="flex items-center gap-1.5 text-xs text-muted-foreground">
                  <Mail className="size-3.5" aria-hidden="true" />
                  <span>{email}</span>
                </div>
              ) : null}
            </div>
          </div>
        </section>

        {/* Stat snapshot — 4-up grid, mock parity (UserProfileView.tsx:127). */}
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4" data-testid="profile-stats">
          <StatTile
            icon={Package}
            label={t("settings:profile.stats.items")}
            value={statValues.items}
            testId="profile-stat-items"
          />
          <StatTile
            icon={ShieldCheck}
            label={t("settings:profile.stats.activeWarranties")}
            value={statValues.activeWarranties}
            iconClassName="text-status-active"
            valueClassName="text-status-active"
            testId="profile-stat-active-warranties"
          />
          <StatTile
            icon={ShieldCheck}
            label={t("settings:profile.stats.expiringSoon")}
            value={statValues.expiringSoon}
            iconClassName="text-status-expiring"
            valueClassName="text-status-expiring"
            testId="profile-stat-expiring-warranties"
          />
          <StatTile
            icon={TrendingUp}
            label={t("settings:profile.stats.estValue")}
            value={statValues.estValue}
            testId="profile-stat-est-value"
          />
        </div>

        {/* Tabs: Groups (mock's "Inventory" repurposed per issue) + Activity. */}
        <Tabs defaultValue="groups" data-testid="profile-tabs">
          <TabsList variant="line" className="justify-start">
            <TabsTrigger value="groups" data-testid="profile-tab-groups">
              {t("settings:profile.tabs.groups")}
            </TabsTrigger>
            <TabsTrigger value="activity" data-testid="profile-tab-activity">
              {t("settings:profile.tabs.activity")}
            </TabsTrigger>
          </TabsList>

          <TabsContent value="groups" className="mt-4" data-testid="profile-tab-groups-content">
            <GroupsTabBody
              groups={visibleGroups}
              overflow={overflowGroups}
              currentSlug={currentGroup?.slug}
              isLoading={groupsLoading}
            />
          </TabsContent>

          <TabsContent value="activity" className="mt-4" data-testid="profile-tab-activity-content">
            <ActivityTabBody />
          </TabsContent>
        </Tabs>

        {/* Avatar upload tracker — keep the per-section stub linked to
            #1382. Demoted below the snapshot so it doesn't compete with
            the new identity → stats → tabs primary scan. */}
        <ComingSoonBanner surface="profilePhoto" />

        <Separator />

        <div className="text-sm text-muted-foreground">
          <p>
            {t("settings:profile.subtitle")}{" "}
            <Link
              to={withGroupQuery("/settings", currentGroup?.slug)}
              className="font-medium text-foreground underline-offset-4 hover:underline"
            >
              {t("settings:title")}
            </Link>
            .
          </p>
        </div>
      </div>
    </>
  )
}

interface StatTileProps {
  icon: React.ElementType
  label: string
  value: string
  iconClassName?: string
  valueClassName?: string
  testId: string
}

function StatTile({
  icon: Icon,
  label,
  value,
  iconClassName,
  valueClassName,
  testId,
}: StatTileProps) {
  return (
    <Card className="gap-2 py-4" data-testid={testId}>
      <CardHeader className="px-4 pb-0">
        <Icon className={`size-4 ${iconClassName ?? "text-foreground"}`} aria-hidden="true" />
      </CardHeader>
      <CardContent className="px-4">
        <p
          className={`text-xl font-bold tracking-tight ${valueClassName ?? "text-foreground"}`}
          data-testid={`${testId}-value`}
        >
          {value}
        </p>
        <p className="mt-0.5 text-xs text-muted-foreground">{label}</p>
      </CardContent>
    </Card>
  )
}

interface GroupsTabBodyProps {
  groups: LocationGroup[]
  overflow: number
  currentSlug?: string
  isLoading: boolean
}

function GroupsTabBody({ groups, overflow, currentSlug, isLoading }: GroupsTabBodyProps) {
  const { t } = useTranslation()

  // While the /groups round-trip is in flight, render skeleton tiles
  // (not the empty-state) so the layout doesn't flash "you're not in any
  // groups yet" before the first list resolves. The e2e harness also
  // relies on this: `toHaveCount(0)` on `profile-groups-empty` after a
  // bare `goto` would race the fetch otherwise.
  if (isLoading) {
    return (
      <div
        className="grid gap-3 sm:grid-cols-2"
        data-testid="profile-groups-loading"
        aria-busy="true"
      >
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-[62px] rounded-xl" />
        ))}
      </div>
    )
  }

  if (groups.length === 0) {
    return (
      <div
        className="flex flex-col items-center justify-center gap-3 rounded-xl border border-border bg-card px-6 py-12 text-center"
        data-testid="profile-groups-empty"
      >
        <Users className="size-8 text-muted-foreground/40" aria-hidden="true" />
        <div>
          <p className="text-sm font-semibold">{t("settings:profile.groupsTab.emptyTitle")}</p>
          <p className="mt-1 text-xs text-muted-foreground">
            {t("settings:profile.groupsTab.emptyDescription")}
          </p>
        </div>
      </div>
    )
  }

  return (
    <div data-testid="profile-groups-list">
      <div className="grid gap-3 sm:grid-cols-2">
        {groups.map((group) => (
          <GroupTile key={group.id ?? group.slug} group={group} currentSlug={currentSlug} />
        ))}
      </div>
      {overflow > 0 ? (
        <p
          className="mt-3 text-center text-xs text-muted-foreground"
          data-testid="profile-groups-overflow"
        >
          {t("settings:profile.groupsTab.moreGroups", { count: overflow })}
        </p>
      ) : null}
    </div>
  )
}

interface GroupTileProps {
  group: LocationGroup
  currentSlug?: string
}

function GroupTile({ group, currentSlug }: GroupTileProps) {
  const { t } = useTranslation()
  const slug = group.slug ?? ""
  const href = slug ? `/g/${slug}` : "/groups"
  const memberCount = group.members_count ?? 0
  const role = group.current_user_role
  const roleLabel = roleLabelFor(t, role)
  const isActive = !!currentSlug && currentSlug === slug
  const icon = group.icon?.trim() ?? ""

  return (
    <Link
      to={href}
      data-testid="profile-group-tile"
      data-group-slug={slug}
      aria-label={t("settings:profile.groupsTab.tileAria", { name: group.name ?? "" })}
      className={`flex items-center gap-3 rounded-xl border border-border bg-card p-3 text-left transition-all hover:-translate-y-0.5 hover:shadow-sm focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-none ${
        isActive ? "border-primary/40" : ""
      }`}
    >
      <div
        className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-muted text-base"
        aria-hidden="true"
      >
        {icon || <Building2 className="size-4 text-muted-foreground" />}
      </div>
      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium" data-testid="profile-group-tile-name">
          {group.name ?? slug}
        </p>
        <p className="text-xs text-muted-foreground">
          {t("settings:profile.groupsTab.membersCount", { count: memberCount })}
        </p>
      </div>
      {role ? (
        <Badge
          variant="outline"
          className="shrink-0 text-xs font-medium"
          data-testid="profile-group-tile-role"
          data-role={role}
        >
          {roleLabel}
        </Badge>
      ) : null}
    </Link>
  )
}

// roleLabelFor maps a GroupRole to its translated display string. Inlined
// switch-with-literal-keys (rather than `t(\`...${role}\`)`) so the i18n
// extractor can see each key statically — dynamic keys silently disappear
// from the extracted catalogue and ship as the raw key in production.
function roleLabelFor(t: (key: string) => string, role: GroupRole | undefined): string {
  switch (role) {
    case "owner":
      return t("settings:profile.groupsTab.roleOwner")
    case "admin":
      return t("settings:profile.groupsTab.roleAdmin")
    case "user":
      return t("settings:profile.groupsTab.roleUser")
    case "viewer":
      return t("settings:profile.groupsTab.roleViewer")
    default:
      return t("settings:profile.groupsTab.roleUser")
  }
}

function ActivityTabBody() {
  const { t } = useTranslation()
  // The dedicated user-scoped activity feed isn't wired yet — the
  // commodity-events registry only exposes a per-commodity reader today
  // (#1450), and the audit_logs reader is admin-only. Render an
  // empty-state shell matching the mock's activity-list card frame so
  // the tab body never measures empty; the wired feed lands separately.
  return (
    <div
      className="flex flex-col items-center justify-center gap-3 rounded-xl border border-border bg-card px-6 py-12 text-center"
      data-testid="profile-activity-empty"
    >
      <Calendar className="size-8 text-muted-foreground/40" aria-hidden="true" />
      <div>
        <p className="text-sm font-semibold">{t("settings:profile.activityTab.emptyTitle")}</p>
        <p className="mt-1 text-xs text-muted-foreground">
          {t("settings:profile.activityTab.emptyDescription")}
        </p>
      </div>
    </div>
  )
}
