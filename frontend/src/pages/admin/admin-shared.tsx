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

// Renders a group's lifecycle status as a tone-mapped outline badge.
export function GroupStatusBadge({ status }: { status: GroupStatus | undefined }) {
  const { t } = useTranslation("admin")
  const tone = (status && GROUP_STATUS_TONE[status]) || GROUP_STATUS_TONE.active
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
