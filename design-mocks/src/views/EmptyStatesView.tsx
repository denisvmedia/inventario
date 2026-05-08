import {
  MapPin,
  Layers,
  LayoutGrid,
  Wrench,
  SearchX,
  Building2,
  Mail,
  Plus,
  ArrowRight,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"

/* ─────────────────────────────────────────────
   Shared helpers
───────────────────────────────────────────── */

function EmptyIllustration({ icon: Icon, className }: { icon: React.ElementType; className?: string }) {
  return (
    <div className={`relative flex items-center justify-center ${className}`}>
      <div className="absolute size-32 rounded-full bg-muted/60" />
      <div className="absolute size-20 rounded-full bg-muted" />
      <Icon className="relative size-10 text-muted-foreground/50" />
    </div>
  )
}

interface EmptyStateProps {
  icon: React.ElementType
  title: string
  description: string
  action?: { label: string; onClick?: () => void }
  secondaryAction?: { label: string; onClick?: () => void }
}

function EmptyState({ icon, title, description, action, secondaryAction }: EmptyStateProps) {
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-6 py-24 px-6 text-center">
      <EmptyIllustration icon={icon} className="size-32" />
      <div className="max-w-sm space-y-2">
        <h2 className="text-lg font-semibold tracking-tight">{title}</h2>
        <p className="text-sm text-muted-foreground leading-relaxed">{description}</p>
      </div>
      {(action || secondaryAction) && (
        <div className="flex items-center gap-2">
          {action && (
            <Button size="sm" onClick={action.onClick}>{action.label}</Button>
          )}
          {secondaryAction && (
            <Button size="sm" variant="outline" onClick={secondaryAction.onClick}>{secondaryAction.label}</Button>
          )}
        </div>
      )}
    </div>
  )
}

/* ─────────────────────────────────────────────
   404 – Not Found
───────────────────────────────────────────── */

export function NotFoundView({ onGoHome }: { onGoHome?: () => void }) {
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-6 py-24 px-6 text-center">
      <div className="relative flex items-center justify-center size-32">
        <div className="absolute size-32 rounded-full bg-muted/60" />
        <div className="absolute size-20 rounded-full bg-muted" />
        <SearchX className="relative size-10 text-muted-foreground/50" />
      </div>
      <div className="max-w-sm space-y-2">
        <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">404</p>
        <h2 className="text-2xl font-bold tracking-tight">Page not found</h2>
        <p className="text-sm text-muted-foreground leading-relaxed">
          The page you're looking for doesn't exist or has been moved.
        </p>
      </div>
      <div className="flex items-center gap-2">
        <Button size="sm" onClick={onGoHome}>Go to Dashboard</Button>
        <Button size="sm" variant="outline">Go back</Button>
      </div>
    </div>
  )
}

/* ─────────────────────────────────────────────
   No Location Group yet (simple empty state)
───────────────────────────────────────────── */

export function NoLocationGroupView({ onCreate }: { onCreate?: () => void }) {
  return (
    <EmptyState
      icon={Layers}
      title="No location group yet"
      description="Location groups help you organise your home into separate units — like a main house, a garage, or a storage unit. Create your first group to get started."
      action={{ label: "Create location group", onClick: onCreate }}
    />
  )
}

/* ─────────────────────────────────────────────
   No Group Onboarding (brand-new user)
───────────────────────────────────────────── */

export function NoGroupOnboardingView({
  onCreateGroup,
}: {
  onCreateGroup?: () => void
}) {
  const PENDING_INVITES = [
    { id: "inv1", groupName: "Johnson Family Home", invitedBy: "Morgan Johnson", role: "user" },
  ]

  return (
    <div className="flex flex-1 flex-col items-center justify-center py-16 px-6">
      <div className="w-full max-w-md space-y-8">
        {/* Hero */}
        <div className="text-center space-y-3">
          <div className="flex justify-center">
            <div className="relative flex items-center justify-center size-20">
              <div className="absolute size-20 rounded-full bg-muted/60" />
              <div className="absolute size-14 rounded-full bg-muted" />
              <Building2 className="relative size-8 text-muted-foreground/60" />
            </div>
          </div>
          <h1 className="text-2xl font-semibold tracking-tight">Welcome to Inventario</h1>
          <p className="text-sm text-muted-foreground leading-relaxed">
            You don't belong to any inventory group yet. Create one to start tracking your belongings, or accept a pending invite below.
          </p>
        </div>

        {/* Create group CTA */}
        <div className="rounded-xl border border-border bg-card p-5 space-y-3">
          <div className="flex items-center gap-3">
            <div className="flex size-10 items-center justify-center rounded-lg bg-primary/10 shrink-0">
              <Plus className="size-5 text-primary" />
            </div>
            <div>
              <p className="font-semibold text-sm">Create a new group</p>
              <p className="text-xs text-muted-foreground">Set up your own inventory and invite others</p>
            </div>
          </div>
          <Button className="w-full gap-2" onClick={onCreateGroup}>
            Create group
            <ArrowRight className="size-4" />
          </Button>
        </div>

        {/* Pending invites */}
        {PENDING_INVITES.length > 0 && (
          <div className="space-y-3">
            <div className="relative">
              <Separator />
              <span className="absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 bg-background px-2 text-xs text-muted-foreground">
                or accept an invite
              </span>
            </div>

            <div className="space-y-2">
              {PENDING_INVITES.map((invite) => (
                <div key={invite.id} className="rounded-xl border border-border bg-card p-4 space-y-3">
                  <div className="flex items-center gap-3">
                    <div className="flex size-9 items-center justify-center rounded-lg bg-muted shrink-0">
                      <Mail className="size-4 text-muted-foreground" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="font-medium text-sm truncate">{invite.groupName}</p>
                      <p className="text-xs text-muted-foreground">
                        Invited by {invite.invitedBy} · as <span className="capitalize">{invite.role}</span>
                      </p>
                    </div>
                  </div>
                  <div className="grid grid-cols-2 gap-2">
                    <Button variant="outline" size="sm">Decline</Button>
                    <Button size="sm">Accept</Button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

/* ─────────────────────────────────────────────
   No Location yet
───────────────────────────────────────────── */

export function NoLocationView({ onCreate }: { onCreate?: () => void }) {
  return (
    <EmptyState
      icon={MapPin}
      title="No locations yet"
      description="Locations are rooms or zones within a location group — like Kitchen, Living Room, or Attic. Add your first location to start organising your items."
      action={{ label: "Add location", onClick: onCreate }}
      secondaryAction={{ label: "Learn more" }}
    />
  )
}

/* ─────────────────────────────────────────────
   No Area yet
───────────────────────────────────────────── */

export function NoAreaView({ onCreate }: { onCreate?: () => void }) {
  return (
    <EmptyState
      icon={LayoutGrid}
      title="No areas in this location"
      description="Areas are specific spots within a location — a shelf, a drawer, a cabinet. Break down your location into areas for precise item tracking."
      action={{ label: "Add area", onClick: onCreate }}
      secondaryAction={{ label: "Skip for now" }}
    />
  )
}

/* ─────────────────────────────────────────────
   Maintenance
───────────────────────────────────────────── */

export function MaintenanceView() {
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-6 py-24 px-6 text-center">
      <div className="relative flex items-center justify-center size-32">
        <div className="absolute size-32 rounded-full bg-muted/60" />
        <div className="absolute size-20 rounded-full bg-muted" />
        <Wrench className="relative size-10 text-muted-foreground/50" />
      </div>
      <div className="max-w-sm space-y-2">
        <div className="inline-flex items-center gap-1.5 rounded-full border border-border bg-muted px-3 py-1 text-xs font-medium text-muted-foreground">
          <span className="size-1.5 rounded-full bg-status-expiring animate-pulse" />
          Scheduled maintenance
        </div>
        <h2 className="text-2xl font-bold tracking-tight">We'll be right back</h2>
        <p className="text-sm text-muted-foreground leading-relaxed">
          The system is temporarily unavailable while we perform scheduled maintenance.
          This usually takes less than 30 minutes.
        </p>
      </div>
      <div className="rounded-xl border border-border bg-card px-5 py-4 text-left max-w-xs w-full space-y-3">
        <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">Status</p>
        {[
          { label: "API", status: "Degraded" },
          { label: "Database", status: "Maintenance" },
          { label: "File storage", status: "Operational" },
        ].map(({ label, status }) => (
          <div key={label} className="flex items-center justify-between">
            <span className="text-sm">{label}</span>
            <span className={`text-xs font-medium ${
              status === "Operational" ? "text-status-active" :
              status === "Degraded" ? "text-status-expiring" :
              "text-muted-foreground"
            }`}>{status}</span>
          </div>
        ))}
      </div>
      <p className="text-xs text-muted-foreground">
        Expected to resume at <strong>14:30 UTC</strong>. Check our status page for live updates.
      </p>
    </div>
  )
}
