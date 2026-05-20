import { useState } from "react"
import {
  User,
  Monitor,
  Ban,
  CircleCheck,
  UserCog,
  Layers,
  ChevronRight,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import { Badge } from "@/components/ui/badge"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import { cn } from "@/lib/utils"
import { adminUserById, tenantById, adminGroupById } from "@/data/mock"
import {
  AdminBackButton,
  AccountStateBadge,
  RoleBadge,
  TenantChip,
  fmtDateTime,
  relativeFromNow,
} from "./admin-shared"

interface UserDetailViewProps {
  userId: string
  onBack?: () => void
  onSelectGroup?: (groupId: string) => void
  onImpersonate?: (userId: string) => void
}

export function UserDetailView({
  userId,
  onBack,
  onSelectGroup,
  onImpersonate,
}: UserDetailViewProps) {
  const user = adminUserById(userId)
  const [active, setActive] = useState(user?.isActive ?? true)
  const [confirmBlock, setConfirmBlock] = useState(false)
  // Sessions are local state so blocking can revoke them (mock of the
  // real "sign out of all sessions" behaviour). Not restored on unblock —
  // matches the backend, which does not re-issue tokens.
  const [sessions, setSessions] = useState(user?.sessions ?? [])

  if (!user) {
    return (
      <div className="flex flex-col gap-6 p-6 max-w-3xl mx-auto w-full">
        <AdminBackButton label="Back" onClick={onBack} />
        <div className="flex flex-col items-center justify-center gap-3 py-24">
          <User className="size-8 text-muted-foreground/30" />
          <p className="text-sm text-muted-foreground">User not found.</p>
        </div>
      </div>
    )
  }

  const tenant = tenantById(user.tenantId)

  return (
    <div className="flex flex-col gap-6 p-6 max-w-3xl mx-auto w-full">
      <AdminBackButton label="Back" onClick={onBack} />

      {/* Identity card */}
      <div className="rounded-xl border border-border bg-card p-6">
        <div className="flex items-start gap-4">
          <div className="flex size-14 items-center justify-center rounded-full bg-primary text-primary-foreground text-lg font-semibold shrink-0">
            {user.avatarInitials}
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight">{user.name}</h1>
              <AccountStateBadge active={active} />
            </div>
            <p className="mt-0.5 text-sm text-muted-foreground">{user.email}</p>
            <div className="mt-3 flex flex-wrap items-center gap-2">
              <TenantChip tenantId={user.tenantId} />
              <RoleBadge role={user.role} />
            </div>
          </div>
        </div>

        <Separator className="my-5" />

        <div className="flex flex-wrap items-center gap-2">
          <Button
            size="sm"
            variant="outline"
            className="gap-1.5"
            disabled={!active}
            onClick={() => onImpersonate?.(user.id)}
          >
            <UserCog className="size-3.5" />
            Impersonate
          </Button>
          {active ? (
            <Button
              size="sm"
              variant="outline"
              className="gap-1.5 text-destructive hover:bg-destructive/10 hover:text-destructive"
              onClick={() => setConfirmBlock(true)}
            >
              <Ban className="size-3.5" />
              Block user
            </Button>
          ) : (
            <Button size="sm" variant="outline" className="gap-1.5" onClick={() => setActive(true)}>
              <CircleCheck className="size-3.5" />
              Unblock user
            </Button>
          )}
        </div>
      </div>

      {/* Sessions */}
      <div>
        <div className="mb-3 flex items-center gap-2">
          <Monitor className="size-4 text-muted-foreground" />
          <h2 className="text-base font-semibold">Sessions</h2>
          <Badge variant="secondary" className="h-5 text-xs">
            {sessions.length}
          </Badge>
        </div>
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          {sessions.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-2 py-12">
              <Monitor className="size-7 text-muted-foreground/30" />
              <p className="text-sm text-muted-foreground">No active sessions.</p>
            </div>
          ) : (
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="pl-4">Device</TableHead>
                  <TableHead>IP address</TableHead>
                  <TableHead>Location</TableHead>
                  <TableHead className="pr-4">Last active</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sessions.map((s) => (
                  <TableRow key={s.id} className="hover:bg-transparent">
                    <TableCell className="pl-4 py-3.5">
                      <div className="flex items-center gap-2">
                        <span className="text-sm font-medium">{s.device}</span>
                        {s.current && (
                          <Badge
                            variant="outline"
                            className="h-5 text-xs border-current/20 font-medium text-status-active bg-status-active/10"
                          >
                            Current
                          </Badge>
                        )}
                      </div>
                    </TableCell>
                    <TableCell className="py-3.5 font-mono text-xs text-muted-foreground">{s.ip}</TableCell>
                    <TableCell className="py-3.5 text-sm text-muted-foreground">{s.location}</TableCell>
                    <TableCell className="pr-4 py-3.5 text-sm text-muted-foreground">
                      {relativeFromNow(s.lastActive)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </div>
      </div>

      {/* Group memberships */}
      <div>
        <div className="mb-3 flex items-center gap-2">
          <Layers className="size-4 text-muted-foreground" />
          <h2 className="text-base font-semibold">Group memberships</h2>
          <Badge variant="secondary" className="h-5 text-xs">
            {user.groupMemberships.length}
          </Badge>
        </div>
        <div className="rounded-xl border border-border bg-card divide-y divide-border overflow-hidden">
          {user.groupMemberships.length === 0 && (
            <div className="flex flex-col items-center justify-center gap-2 py-12">
              <Layers className="size-7 text-muted-foreground/30" />
              <p className="text-sm text-muted-foreground">Not a member of any group.</p>
            </div>
          )}
          {user.groupMemberships.map((m) => {
            const group = adminGroupById(m.groupId)
            if (!group) return null
            return (
              <button
                key={m.groupId}
                onClick={() => onSelectGroup?.(m.groupId)}
                className="flex w-full items-center gap-3 px-4 py-3.5 text-left transition-colors hover:bg-muted/50"
              >
                <div className="flex size-8 items-center justify-center rounded-lg bg-muted shrink-0">
                  <Layers className="size-4 text-muted-foreground" />
                </div>
                <div className="flex-1 min-w-0">
                  <p className="text-sm font-medium truncate">{group.name}</p>
                  <p className="text-xs text-muted-foreground">{group.currency}</p>
                </div>
                <RoleBadge role={m.role} />
                <ChevronRight className="size-4 text-muted-foreground shrink-0" />
              </button>
            )
          })}
        </div>
      </div>

      {/* Meta */}
      <p className="text-xs text-muted-foreground">
        Member of <span className="font-medium text-foreground">{tenant?.name}</span> · last login{" "}
        {fmtDateTime(user.lastLogin)}
      </p>

      {/* Block confirmation */}
      <AlertDialog open={confirmBlock} onOpenChange={setConfirmBlock}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Block this user?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-medium text-foreground">{user.name}</span> will be signed out of all
              sessions and cannot sign in until unblocked. This does not delete their data.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className={cn("bg-destructive text-destructive-foreground hover:bg-destructive/90")}
              onClick={() => {
                setActive(false)
                setSessions([])
                setConfirmBlock(false)
              }}
            >
              Block user
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
