import { useState } from "react"
import { UserPlus, MoveHorizontal as MoreHorizontal, Shield, User, Mail, Crown, Trash2, Check, X, Users, Clock, Plus, Building2, Eye } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Badge } from "@/components/ui/badge"
import { Separator } from "@/components/ui/separator"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog"
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
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Label } from "@/components/ui/label"

import { MOCK_GROUPS, type Member, type MemberRole } from "@/data/mock"
import { cn } from "@/lib/utils"
import { CurrencyCombobox } from "@/components/CurrencyCombobox"

const ROLE_CONFIG: Record<MemberRole, {
  label: string
  description: string
  icon: React.ElementType
  badgeClass: string
}> = {
  viewer: {
    label: "Viewer",
    description: "Can view all items, but cannot make any changes",
    icon: Eye,
    badgeClass: "bg-muted text-muted-foreground border-0",
  },
  user: {
    label: "User",
    description: "Can add and remove items (not locations or areas)",
    icon: User,
    badgeClass: "bg-chart-3/10 text-chart-3 border-0",
  },
  admin: {
    label: "Administrator",
    description: "Can manage members, group info, locations and areas",
    icon: Shield,
    badgeClass: "bg-primary/10 text-primary border-0",
  },
  owner: {
    label: "Owner",
    description: "Full access including deleting the group and all data",
    icon: Crown,
    badgeClass: "bg-accent text-accent-foreground border-0",
  },
}

const ROLE_ORDER: MemberRole[] = ["viewer", "user", "admin", "owner"]

const PENDING_INVITES = [
  { id: "inv1", email: "jordan@example.com", role: "user" as MemberRole, sentAt: "2026-04-20" },
  { id: "inv2", email: "riley@example.com", role: "viewer" as MemberRole, sentAt: "2026-04-24" },
]

interface MembersViewProps {
  activeGroupId: string
}

export function MembersView({ activeGroupId }: MembersViewProps) {
  const group = MOCK_GROUPS.find((g) => g.id === activeGroupId) ?? MOCK_GROUPS[0]
  const [inviteOpen, setInviteOpen] = useState(false)
  const [inviteEmail, setInviteEmail] = useState("")
  const [inviteRole, setInviteRole] = useState<MemberRole>("user")
  const [inviteSent, setInviteSent] = useState(false)

  const [removeMemberTarget, setRemoveMemberTarget] = useState<Member | null>(null)
  const [revokeInviteTarget, setRevokeInviteTarget] = useState<{ id: string; email: string } | null>(null)
  const [removedMemberIds, setRemovedMemberIds] = useState<Set<string>>(new Set())
  const [revokedInviteIds, setRevokedInviteIds] = useState<Set<string>>(new Set())

  const activeMembers = group.members.filter((m) => !removedMemberIds.has(m.id))
  const activeInvites = PENDING_INVITES.filter((i) => !revokedInviteIds.has(i.id))

  const currentUser = group.members.find((m) => m.id === "u1")
  const currentUserRoleIdx = currentUser ? ROLE_ORDER.indexOf(currentUser.role) : -1
  const canManageMembers = currentUserRoleIdx >= ROLE_ORDER.indexOf("admin")

  function sendInvite() {
    if (!inviteEmail.trim()) return
    setInviteSent(true)
    setTimeout(() => {
      setInviteOpen(false)
      setInviteSent(false)
      setInviteEmail("")
      setInviteRole("user")
    }, 1200)
  }

  return (
    <div className="flex flex-col gap-8 p-6 max-w-3xl mx-auto w-full">
      {/* Header */}
      <div className="flex items-start justify-between gap-4">
        <div>
          <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">Members</h1>
          <p className="mt-1 text-muted-foreground">
            People with access to <span className="font-medium text-foreground">{group.name}</span>.
          </p>
        </div>
        {canManageMembers && (
          <Button size="sm" className="gap-1.5 shrink-0" onClick={() => setInviteOpen(true)}>
            <UserPlus className="size-4" />
            Invite
          </Button>
        )}
      </div>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-3">
        {[
          { label: "Members", value: activeMembers.length, icon: Users },
          { label: "Admins & Owners", value: activeMembers.filter((m) => m.role === "admin" || m.role === "owner").length, icon: Crown },
          { label: "Pending invites", value: activeInvites.length, icon: Clock },
        ].map((s) => (
          <div key={s.label} className="flex items-center gap-3 rounded-xl border border-border bg-card px-4 py-3">
            <div className="flex size-8 items-center justify-center rounded-lg bg-muted">
              <s.icon className="size-4 text-muted-foreground" />
            </div>
            <div>
              <p className="text-sm font-semibold">{s.value}</p>
              <p className="text-xs text-muted-foreground">{s.label}</p>
            </div>
          </div>
        ))}
      </div>

      {/* Role legend */}
      <div className="rounded-xl border border-border bg-card divide-y divide-border">
        <div className="px-4 py-3">
          <p className="text-xs font-semibold uppercase tracking-widest text-muted-foreground">Role permissions</p>
        </div>
        {ROLE_ORDER.map((role) => {
          const cfg = ROLE_CONFIG[role]
          const Icon = cfg.icon
          return (
            <div key={role} className="flex items-center gap-3 px-4 py-3">
              <div className="flex size-7 items-center justify-center rounded-md bg-muted shrink-0">
                <Icon className="size-3.5 text-muted-foreground" />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-medium">{cfg.label}</p>
                <p className="text-xs text-muted-foreground">{cfg.description}</p>
              </div>
              <RoleBadge role={role} />
            </div>
          )
        })}
      </div>

      <div className="space-y-6">
        {/* Active members */}
        <div>
          <h2 className="text-sm font-semibold mb-3">Active members</h2>
          <div className="rounded-xl border border-border overflow-hidden bg-card">
            {activeMembers.map((member, i) => (
              <div key={member.id}>
                {i > 0 && <Separator />}
                <MemberRow
                  member={member}
                  isCurrentUser={member.id === "u1"}
                  canManage={canManageMembers && member.id !== "u1" && member.role !== "owner"}
                  onRemove={() => setRemoveMemberTarget(member)}
                />
              </div>
            ))}
          </div>
        </div>

        {/* Pending invites */}
        {activeInvites.length > 0 && (
          <div>
            <h2 className="text-sm font-semibold mb-3">Pending invites</h2>
            <div className="rounded-xl border border-border overflow-hidden bg-card">
              {activeInvites.map((invite, i) => (
                <div key={invite.id}>
                  {i > 0 && <Separator />}
                  <div className="flex items-center gap-3 px-4 py-3">
                    <div className="flex size-8 items-center justify-center rounded-full bg-muted text-xs font-semibold text-muted-foreground shrink-0">
                      <Mail className="size-4" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-sm font-medium truncate">{invite.email}</p>
                      <p className="text-xs text-muted-foreground">
                        Invited {new Date(invite.sentAt).toLocaleDateString("en-US", { month: "short", day: "numeric" })}
                      </p>
                    </div>
                    <RoleBadge role={invite.role} />
                    <Badge variant="secondary" className="text-[10px] h-5">Pending</Badge>
                    {canManageMembers && (
                      <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                          <Button variant="ghost" size="icon" className="size-7">
                            <MoreHorizontal className="size-4" />
                          </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end">
                          <DropdownMenuItem>Resend invite</DropdownMenuItem>
                          <DropdownMenuSeparator />
                          <DropdownMenuItem
                            className="text-destructive focus:text-destructive"
                            onClick={() => setRevokeInviteTarget({ id: invite.id, email: invite.email })}
                          >
                            <X className="size-4 mr-2" />Revoke
                          </DropdownMenuItem>
                        </DropdownMenuContent>
                      </DropdownMenu>
                    )}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Invite dialog */}
      <Dialog open={inviteOpen} onOpenChange={setInviteOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>Invite to {group.name}</DialogTitle>
            <DialogDescription>
              Send an invite link to add someone to this location group.
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-2">
            <div className="flex flex-col gap-1.5">
              <label className="text-sm font-medium">Email address</label>
              <Input
                type="email"
                placeholder="colleague@example.com"
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && sendInvite()}
                autoFocus
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <label className="text-sm font-medium">Role</label>
              <Select value={inviteRole} onValueChange={(v) => setInviteRole(v as MemberRole)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent>
                  {(["viewer", "user", "admin"] as MemberRole[]).map((role) => {
                    const cfg = ROLE_CONFIG[role]
                    const Icon = cfg.icon
                    return (
                      <SelectItem key={role} value={role}>
                        <div className="flex items-center gap-2">
                          <Icon className="size-3.5 text-muted-foreground shrink-0" />
                          <div>
                            <p className="font-medium">{cfg.label}</p>
                            <p className="text-xs text-muted-foreground">{cfg.description}</p>
                          </div>
                        </div>
                      </SelectItem>
                    )
                  })}
                </SelectContent>
              </Select>
            </div>
            <div className="flex gap-2 pt-2">
              <Button variant="outline" className="flex-1" onClick={() => setInviteOpen(false)}>Cancel</Button>
              <Button className="flex-1 gap-2" onClick={sendInvite} disabled={!inviteEmail.trim() || inviteSent}>
                {inviteSent ? <><Check className="size-4" />Sent!</> : <><UserPlus className="size-4" />Send invite</>}
              </Button>
            </div>
          </div>
        </DialogContent>
      </Dialog>

      {/* Remove member confirmation */}
      <AlertDialog open={!!removeMemberTarget} onOpenChange={(open) => !open && setRemoveMemberTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove member?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-medium text-foreground">{removeMemberTarget?.name}</span> will lose access to{" "}
              <span className="font-medium text-foreground">{group.name}</span> and all its inventory. They can be re-invited later.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                if (removeMemberTarget) {
                  setRemovedMemberIds((prev) => new Set([...prev, removeMemberTarget.id]))
                  setRemoveMemberTarget(null)
                }
              }}
            >
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Revoke invite confirmation */}
      <AlertDialog open={!!revokeInviteTarget} onOpenChange={(open) => !open && setRevokeInviteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Revoke invite?</AlertDialogTitle>
            <AlertDialogDescription>
              The invite sent to{" "}
              <span className="font-medium text-foreground">{revokeInviteTarget?.email}</span> will be cancelled.
              They won't be able to use the invite link anymore.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              onClick={() => {
                if (revokeInviteTarget) {
                  setRevokedInviteIds((prev) => new Set([...prev, revokeInviteTarget.id]))
                  setRevokeInviteTarget(null)
                }
              }}
            >
              Revoke
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}

// ─── Member row ───────────────────────────────────────────────────────────────

function MemberRow({
  member,
  isCurrentUser,
  canManage,
  onRemove,
}: {
  member: Member
  isCurrentUser: boolean
  canManage: boolean
  onRemove: () => void
}) {
  return (
    <div className="flex items-center gap-3 px-4 py-3">
      <div className={cn(
        "flex size-8 items-center justify-center rounded-full text-xs font-semibold shrink-0",
        isCurrentUser ? "bg-primary text-primary-foreground" : "bg-muted text-muted-foreground"
      )}>
        {member.avatarInitials}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-center gap-1.5">
          <p className="text-sm font-medium truncate">{member.name}</p>
          {isCurrentUser && <span className="text-xs text-muted-foreground">(you)</span>}
        </div>
        <p className="text-xs text-muted-foreground truncate">{member.email}</p>
      </div>
      <RoleBadge role={member.role} />
      <p className="text-xs text-muted-foreground hidden sm:block">
        since {new Date(member.joinedAt).toLocaleDateString("en-US", { month: "short", year: "numeric" })}
      </p>
      {canManage && (
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <Button variant="ghost" size="icon" className="size-7">
              <MoreHorizontal className="size-4" />
            </Button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="end">
            <DropdownMenuItem>
              <Shield className="size-4 mr-2" />Change role
            </DropdownMenuItem>
            <DropdownMenuSeparator />
            <DropdownMenuItem className="text-destructive focus:text-destructive" onClick={onRemove}>
              <Trash2 className="size-4 mr-2" />Remove from group
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      )}
    </div>
  )
}

function RoleBadge({ role }: { role: MemberRole }) {
  const cfg = ROLE_CONFIG[role]
  const Icon = cfg.icon
  return (
    <Badge variant="secondary" className={cn("gap-1 h-5 text-[10px]", cfg.badgeClass)}>
      <Icon className="size-3" />
      {cfg.label}
    </Badge>
  )
}

// ─── Create group dialog ──────────────────────────────────────────────────────

interface CreateGroupDialogProps {
  open: boolean
  onClose: () => void
  onCreated: (name: string) => void
}

export function CreateGroupDialog({ open, onClose, onCreated }: CreateGroupDialogProps) {
  const [name, setName] = useState("")
  const [description, setDescription] = useState("")
  const [currency, setCurrency] = useState("USD")

  function handleCreate() {
    if (!name.trim()) return
    onCreated(name.trim())
    setName("")
    setDescription("")
    setCurrency("USD")
    onClose()
  }

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle className="flex items-center gap-2">
            <div className="flex size-7 items-center justify-center rounded-lg bg-primary/10">
              <Building2 className="size-4 text-primary" />
            </div>
            Create group
          </DialogTitle>
          <DialogDescription>
            A group is a shared inventory space — for your home, office, or storage unit.
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-4 py-2">
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="cg-name">Group name <span className="text-destructive">*</span></Label>
            <Input
              id="cg-name"
              placeholder="e.g. Main Residence"
              value={name}
              onChange={(e) => setName(e.target.value)}
              autoFocus
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="cg-desc">Description</Label>
            <Input
              id="cg-desc"
              placeholder="Optional — e.g. Primary home at 14 Oak Street"
              value={description}
              onChange={(e) => setDescription(e.target.value)}
            />
          </div>
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="cg-currency">Main currency</Label>
            <CurrencyCombobox value={currency} onValueChange={setCurrency} />
            <p className="text-xs text-muted-foreground">All item prices will be shown in this currency. You can change it later.</p>
          </div>
        </div>
        <DialogFooter className="gap-2">
          <Button variant="outline" onClick={onClose}>Cancel</Button>
          <Button onClick={handleCreate} disabled={!name.trim()} className="gap-2">
            <Plus className="size-4" />
            Create group
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
