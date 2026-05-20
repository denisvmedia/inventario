import { Building2, Shield, User, Crown, Eye, ArrowLeft } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { cn } from "@/lib/utils"
import {
  tenantById,
  TENANT_STATUS_CONFIG,
  GROUP_STATUS_CONFIG,
  type TenantStatus,
  type GroupStatus,
  type MemberRole,
} from "@/data/mock"

// ─── Tenant chip ──────────────────────────────────────────────
// Compact, non-interactive tenant indicator used inside admin tables.
export function TenantChip({ tenantId, className }: { tenantId: string; className?: string }) {
  const tenant = tenantById(tenantId)
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border border-border bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground select-none",
        className
      )}
    >
      <Building2 className="size-3 shrink-0" />
      <span className="truncate max-w-32">{tenant?.name ?? "Unknown tenant"}</span>
    </span>
  )
}

// ─── Status badges ────────────────────────────────────────────
export function TenantStatusBadge({ status }: { status: TenantStatus }) {
  const cfg = TENANT_STATUS_CONFIG[status]
  return (
    <Badge variant="outline" className={cn("h-5 text-xs border-current/20 font-medium", cfg.color, cfg.bg)}>
      {cfg.label}
    </Badge>
  )
}

export function GroupStatusBadge({ status }: { status: GroupStatus }) {
  const cfg = GROUP_STATUS_CONFIG[status]
  return (
    <Badge variant="outline" className={cn("h-5 text-xs border-current/20 font-medium", cfg.color, cfg.bg)}>
      {cfg.label}
    </Badge>
  )
}

// ─── Active / blocked badge ───────────────────────────────────
export function AccountStateBadge({ active }: { active: boolean }) {
  return (
    <Badge
      variant="outline"
      className={cn(
        "h-5 text-xs border-current/20 font-medium gap-1",
        active ? "text-status-active bg-status-active/10" : "text-status-expired bg-status-expired/10"
      )}
    >
      <span className={cn("size-1.5 rounded-full", active ? "bg-status-active" : "bg-status-expired")} />
      {active ? "Active" : "Blocked"}
    </Badge>
  )
}

// ─── Role badge — mirrors MembersView role styling ────────────
export const ADMIN_ROLE_CONFIG: Record<
  MemberRole,
  { label: string; icon: React.ElementType; badgeClass: string }
> = {
  viewer: { label: "Viewer", icon: Eye, badgeClass: "bg-muted text-muted-foreground border-0" },
  user: { label: "User", icon: User, badgeClass: "bg-chart-3/10 text-chart-3 border-0" },
  admin: { label: "Administrator", icon: Shield, badgeClass: "bg-primary/10 text-primary border-0" },
  owner: { label: "Owner", icon: Crown, badgeClass: "bg-accent text-accent-foreground border-0" },
}

export function RoleBadge({ role }: { role: MemberRole }) {
  const cfg = ADMIN_ROLE_CONFIG[role]
  const Icon = cfg.icon
  return (
    <Badge variant="secondary" className={cn("gap-1 h-5 text-xs", cfg.badgeClass)}>
      <Icon className="size-3" />
      {cfg.label}
    </Badge>
  )
}

// ─── Back button — matches EditProfileView / PlansView onBack ─
export function AdminBackButton({ label, onClick }: { label: string; onClick?: () => void }) {
  return (
    <Button
      variant="ghost"
      size="sm"
      className="gap-1.5 -ml-2 self-start text-muted-foreground hover:text-foreground"
      onClick={onClick}
    >
      <ArrowLeft className="size-4" />
      {label}
    </Button>
  )
}

// ─── Date formatting helpers ──────────────────────────────────
export function fmtDate(iso: string): string {
  return new Date(iso).toLocaleDateString("en-US", { month: "short", day: "numeric", year: "numeric" })
}

export function fmtDateTime(iso: string | null): string {
  if (!iso) return "Never"
  // toLocaleString (not toLocaleDateString) — some runtimes ignore the
  // hour/minute fields on toLocaleDateString, dropping the time portion.
  return new Date(iso).toLocaleString("en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  })
}

export function relativeFromNow(iso: string): string {
  const diffMs = Date.now() - new Date(iso).getTime()
  // Clamp future timestamps (mock session data can land later-today) to
  // "just now" instead of rendering a negative "-Xm ago".
  if (diffMs < 60000) return "just now"
  const mins = Math.round(diffMs / 60000)
  if (mins < 60) return `${mins}m ago`
  const hrs = Math.round(mins / 60)
  if (hrs < 24) return `${hrs}h ago`
  const days = Math.round(hrs / 24)
  return `${days}d ago`
}
