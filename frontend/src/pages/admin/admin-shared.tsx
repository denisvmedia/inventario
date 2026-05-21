import { Building2, Crown, Eye, Shield, User } from "lucide-react"
import { useTranslation } from "react-i18next"

import { Badge } from "@/components/ui/badge"
import { cn } from "@/lib/utils"
import type { Schema } from "@/types"

// Shared visual primitives for the /admin/* pages. The design mock keeps
// the equivalent bits in design-mocks/src/views/admin/admin-shared.tsx;
// this file is the frontend port — same badge anatomy (status tokens,
// `border-current/20` outline), with literal labels swapped for `admin`
// namespace i18n keys so the chips translate.

type TenantStatus = Schema<"models.TenantStatus">
type GroupStatus = Schema<"models.LocationGroupStatus">
type GroupRole = Schema<"models.GroupRole">

// Per-status badge tone. Mirrors the design-mock TENANT_STATUS_CONFIG
// palette: status tokens, never raw colors. The BE TenantStatus enum is
// `active | suspended | inactive` — narrower than the mock's
// `active | trial | suspended | archived` (see design-deviations.md).
const TENANT_STATUS_TONE: Record<TenantStatus, string> = {
  active: "text-status-active bg-status-active/10",
  suspended: "text-status-expiring bg-status-expiring/10",
  inactive: "text-status-none bg-status-none/10",
}

// Renders a tenant's lifecycle status as a tone-mapped outline badge. An
// unknown or missing status falls back to the `inactive` tone + an em-dash.
export function TenantStatusBadge({ status }: { status: TenantStatus | undefined }) {
  const { t } = useTranslation("admin")
  const tone = (status && TENANT_STATUS_TONE[status]) || TENANT_STATUS_TONE.inactive
  return (
    <Badge variant="outline" className={cn("h-5 text-xs border-current/20 font-medium", tone)}>
      {status ? t(`tenants.status.${status}`) : "—"}
    </Badge>
  )
}

// Per-status badge tone for location groups. The BE LocationGroupStatus
// enum is `active | pending_deletion`.
const GROUP_STATUS_TONE: Record<GroupStatus, string> = {
  active: "text-status-active bg-status-active/10",
  pending_deletion: "text-status-expired bg-status-expired/10",
}

// Neutral tone for an unknown/missing group status — mirrors the
// `inactive` fallback tone TenantStatusBadge uses. An undefined status
// renders an em-dash label, so the tone must be neutral, not active-green.
const GROUP_STATUS_TONE_NONE = "text-status-none bg-status-none/10"

// Renders a group's lifecycle status as a tone-mapped outline badge. An
// unknown or missing status falls back to the neutral tone + an em-dash.
export function GroupStatusBadge({ status }: { status: GroupStatus | undefined }) {
  const { t } = useTranslation("admin")
  const tone = (status && GROUP_STATUS_TONE[status]) || GROUP_STATUS_TONE_NONE
  return (
    <Badge variant="outline" className={cn("h-5 text-xs border-current/20 font-medium", tone)}>
      {status ? t(`tenantDetail.groups.status.${status}`) : "—"}
    </Badge>
  )
}

// Renders a user's active/blocked state as a dot-prefixed outline badge.
// Mirrors the design-mock AccountStateBadge. `is_active` is optional in
// the API schema, so `undefined` is a genuine runtime possibility — it
// renders as a neutral "unknown" badge rather than silently claiming the
// account is active.
export function AccountStateBadge({ active }: { active: boolean | undefined }) {
  const { t } = useTranslation("admin")
  if (active === undefined) {
    return (
      <Badge
        variant="outline"
        className="h-5 text-xs border-current/20 font-medium gap-1 text-status-none bg-status-none/10"
      >
        <span className="size-1.5 rounded-full bg-status-none" />
        {t("tenantDetail.users.state.unknown")}
      </Badge>
    )
  }
  return (
    <Badge
      variant="outline"
      className={cn(
        "h-5 text-xs border-current/20 font-medium gap-1",
        active
          ? "text-status-active bg-status-active/10"
          : "text-status-expired bg-status-expired/10"
      )}
    >
      <span
        className={cn("size-1.5 rounded-full", active ? "bg-status-active" : "bg-status-expired")}
      />
      {active ? t("tenantDetail.users.state.active") : t("tenantDetail.users.state.blocked")}
    </Badge>
  )
}

// Compact, non-interactive tenant indicator used on admin detail surfaces.
// Mirrors the design-mock TenantChip; the mock resolves a tenant name from
// its id, but the admin user-detail BE only carries `tenant_id`, so this
// renders the id verbatim (see devdocs/frontend/design-deviations.md).
export function TenantChip({ tenantId }: { tenantId: string | undefined }) {
  const { t } = useTranslation("admin")
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-border bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground select-none">
      <Building2 className="size-3 shrink-0" />
      <span className="truncate max-w-40">{tenantId || t("userDetail.unknownTenant")}</span>
    </span>
  )
}

// Per-role badge config. Mirrors the design-mock ADMIN_ROLE_CONFIG (which
// itself mirrors MembersView role styling): role-tinted, borderless
// secondary badges. Literal labels are swapped for `admin` namespace i18n
// keys so the chip translates.
const ROLE_CONFIG: Record<GroupRole, { i18nKey: string; icon: typeof Eye; badgeClass: string }> = {
  viewer: {
    i18nKey: "userDetail.roles.viewer",
    icon: Eye,
    badgeClass: "bg-muted text-muted-foreground border-0",
  },
  user: {
    i18nKey: "userDetail.roles.user",
    icon: User,
    badgeClass: "bg-chart-3/10 text-chart-3 border-0",
  },
  admin: {
    i18nKey: "userDetail.roles.admin",
    icon: Shield,
    badgeClass: "bg-primary/10 text-primary border-0",
  },
  owner: {
    i18nKey: "userDetail.roles.owner",
    icon: Crown,
    badgeClass: "bg-accent text-accent-foreground border-0",
  },
}

// Renders a group-membership role as a role-tinted secondary badge. An
// unknown or missing role renders a neutral em-dash badge.
export function RoleBadge({ role }: { role: GroupRole | undefined }) {
  const { t } = useTranslation("admin")
  if (!role || !ROLE_CONFIG[role]) {
    return (
      <Badge
        variant="secondary"
        className="h-5 text-xs gap-1 bg-muted text-muted-foreground border-0"
      >
        —
      </Badge>
    )
  }
  const cfg = ROLE_CONFIG[role]
  const Icon = cfg.icon
  return (
    <Badge variant="secondary" className={cn("h-5 text-xs gap-1", cfg.badgeClass)}>
      <Icon className="size-3" />
      {t(cfg.i18nKey)}
    </Badge>
  )
}
