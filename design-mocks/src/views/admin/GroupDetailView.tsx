import { useMemo, useState } from "react"
import {
  Layers,
  UserPlus,
  Trash2,
  Ellipsis,
  TriangleAlert,
} from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
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
import {
  adminGroupById,
  adminUserById,
  CURRENCIES,
  type GroupStatus,
  type MemberRole,
} from "@/data/mock"
import {
  AdminBackButton,
  TenantChip,
  GroupStatusBadge,
  ADMIN_ROLE_CONFIG,
} from "./admin-shared"

interface GroupMemberRow {
  id: string
  name: string
  email: string
  avatarInitials: string
  role: MemberRole
}

interface GroupDetailViewProps {
  groupId: string
  onBack?: () => void
}

const ROLE_OPTIONS: MemberRole[] = ["viewer", "user", "admin", "owner"]

export function GroupDetailView({ groupId, onBack }: GroupDetailViewProps) {
  const group = adminGroupById(groupId)

  const initialMembers = useMemo<GroupMemberRow[]>(() => {
    if (!group) return []
    return group.members.flatMap((m) => {
      const u = adminUserById(m.userId)
      return u
        ? [{ id: u.id, name: u.name, email: u.email, avatarInitials: u.avatarInitials, role: m.role }]
        : []
    })
  }, [group])

  const [members, setMembers] = useState<GroupMemberRow[]>(initialMembers)
  const [status, setStatus] = useState<GroupStatus>(group?.status ?? "active")
  const [addOpen, setAddOpen] = useState(false)
  const [addName, setAddName] = useState("")
  const [addEmail, setAddEmail] = useState("")
  const [addRole, setAddRole] = useState<MemberRole>("user")
  const [removeTarget, setRemoveTarget] = useState<GroupMemberRow | null>(null)
  const [confirmDelete, setConfirmDelete] = useState(false)

  if (!group) {
    return (
      <div className="flex flex-col gap-6 p-6 max-w-3xl mx-auto w-full">
        <AdminBackButton label="Back" onClick={onBack} />
        <div className="flex flex-col items-center justify-center gap-3 py-24">
          <Layers className="size-8 text-muted-foreground/30" />
          <p className="text-sm text-muted-foreground">Group not found.</p>
        </div>
      </div>
    )
  }

  const currency = CURRENCIES.find((c) => c.code === group.currency)

  function addMember() {
    if (!addName.trim() || !addEmail.trim()) return
    const initials = addName
      .trim()
      .split(/\s+/)
      .slice(0, 2)
      .map((w) => w[0]?.toUpperCase() ?? "")
      .join("")
    setMembers((prev) => [
      ...prev,
      {
        id: `new-${Date.now()}`,
        name: addName.trim(),
        email: addEmail.trim(),
        avatarInitials: initials || "?",
        role: addRole,
      },
    ])
    setAddName("")
    setAddEmail("")
    setAddRole("user")
    setAddOpen(false)
  }

  function changeRole(memberId: string, role: MemberRole) {
    setMembers((prev) => prev.map((m) => (m.id === memberId ? { ...m, role } : m)))
  }

  return (
    <div className="flex flex-col gap-6 p-6 max-w-3xl mx-auto w-full">
      <AdminBackButton label="Back" onClick={onBack} />

      {/* Header card */}
      <div className="rounded-xl border border-border bg-card p-6">
        <div className="flex items-start gap-4">
          <div className="flex size-12 items-center justify-center rounded-xl bg-primary/10 shrink-0">
            <Layers className="size-6 text-primary" />
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight">{group.name}</h1>
              <GroupStatusBadge status={status} />
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
              <TenantChip tenantId={group.tenantId} />
              <span>
                {group.currency}
                {currency ? ` · ${currency.name}` : ""}
              </span>
              <span>· {members.length} members</span>
            </div>
          </div>
        </div>
      </div>

      {/* Members */}
      <div>
        <div className="mb-3 flex items-center justify-between gap-3">
          <h2 className="text-base font-semibold">Members</h2>
          <Button size="sm" className="gap-1.5" onClick={() => setAddOpen(true)}>
            <UserPlus className="size-3.5" />
            Add member
          </Button>
        </div>
        <div className="rounded-xl border border-border bg-card overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow className="hover:bg-transparent">
                <TableHead className="pl-4">Member</TableHead>
                <TableHead>Role</TableHead>
                <TableHead className="w-10 pr-4" />
              </TableRow>
            </TableHeader>
            <TableBody>
              {members.length === 0 && (
                <TableRow className="hover:bg-transparent">
                  <TableCell colSpan={3} className="h-24 text-center text-sm text-muted-foreground">
                    No members in this group.
                  </TableCell>
                </TableRow>
              )}
              {members.map((member) => (
                <TableRow key={member.id} className="hover:bg-transparent">
                  <TableCell className="pl-4 py-3.5">
                    <div className="flex items-center gap-2.5">
                      <div className="flex size-8 items-center justify-center rounded-full bg-muted text-xs font-semibold text-muted-foreground shrink-0">
                        {member.avatarInitials}
                      </div>
                      <div className="min-w-0">
                        <p className="text-sm font-medium truncate">{member.name}</p>
                        <p className="text-xs text-muted-foreground truncate">{member.email}</p>
                      </div>
                    </div>
                  </TableCell>
                  <TableCell className="py-3.5">
                    <Select
                      value={member.role}
                      onValueChange={(v) => changeRole(member.id, v as MemberRole)}
                    >
                      <SelectTrigger size="sm" className="w-40">
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        {ROLE_OPTIONS.map((role) => {
                          const cfg = ADMIN_ROLE_CONFIG[role]
                          const Icon = cfg.icon
                          return (
                            <SelectItem key={role} value={role}>
                              <Icon className="size-3.5 text-muted-foreground shrink-0" />
                              {cfg.label}
                            </SelectItem>
                          )
                        })}
                      </SelectContent>
                    </Select>
                  </TableCell>
                  <TableCell className="pr-4 py-3.5 text-right">
                    <DropdownMenu>
                      <DropdownMenuTrigger asChild>
                        <Button variant="ghost" size="icon" className="size-7" aria-label="Member actions">
                          <Ellipsis className="size-4" />
                        </Button>
                      </DropdownMenuTrigger>
                      <DropdownMenuContent align="end">
                        <DropdownMenuItem
                          className="text-destructive focus:text-destructive"
                          onClick={() => setRemoveTarget(member)}
                        >
                          <Trash2 className="size-4 mr-2" />
                          Remove from group
                        </DropdownMenuItem>
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      </div>

      {/* Danger zone */}
      <div className="rounded-xl border border-destructive/30 bg-destructive/5 p-5">
        <div className="flex items-start gap-3">
          <div className="flex size-9 items-center justify-center rounded-lg bg-destructive/10 shrink-0">
            <TriangleAlert className="size-4 text-destructive" />
          </div>
          <div className="flex-1 min-w-0">
            <h3 className="text-sm font-semibold">Danger zone</h3>
            <p className="mt-0.5 text-sm text-muted-foreground">
              {status === "pending_deletion"
                ? "This group is queued for deletion. It will be permanently removed after the retention window."
                : "Soft-delete this group. It enters a pending-deletion state and is removed after the retention window."}
            </p>
          </div>
        </div>
        <div className="mt-4">
          <Button
            size="sm"
            variant="destructive"
            className="gap-1.5"
            disabled={status === "pending_deletion"}
            onClick={() => setConfirmDelete(true)}
          >
            <Trash2 className="size-3.5" />
            {status === "pending_deletion" ? "Deletion pending" : "Delete group"}
          </Button>
        </div>
      </div>

      {/* Add member dialog */}
      <Dialog open={addOpen} onOpenChange={setAddOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle className="flex items-center gap-2">
              <div className="flex size-7 items-center justify-center rounded-lg bg-primary/10">
                <UserPlus className="size-4 text-primary" />
              </div>
              Add member
            </DialogTitle>
            <DialogDescription>Add a person to {group.name}.</DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 py-2">
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="add-member-name">Full name</Label>
              <Input
                id="add-member-name"
                placeholder="e.g. Jordan Velez"
                value={addName}
                onChange={(e) => setAddName(e.target.value)}
                autoFocus
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="add-member-email">Email address</Label>
              <Input
                id="add-member-email"
                type="email"
                placeholder="colleague@example.com"
                value={addEmail}
                onChange={(e) => setAddEmail(e.target.value)}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label htmlFor="add-member-role">Role</Label>
              <Select value={addRole} onValueChange={(v) => setAddRole(v as MemberRole)}>
                <SelectTrigger id="add-member-role">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {(["viewer", "user", "admin"] as MemberRole[]).map((role) => {
                    const cfg = ADMIN_ROLE_CONFIG[role]
                    const Icon = cfg.icon
                    return (
                      <SelectItem key={role} value={role}>
                        <Icon className="size-3.5 text-muted-foreground shrink-0" />
                        {cfg.label}
                      </SelectItem>
                    )
                  })}
                </SelectContent>
              </Select>
            </div>
          </div>
          <DialogFooter className="gap-2">
            <Button variant="outline" onClick={() => setAddOpen(false)}>
              Cancel
            </Button>
            <Button
              className="gap-2"
              onClick={addMember}
              disabled={!addName.trim() || !addEmail.trim()}
            >
              <UserPlus className="size-4" />
              Add member
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Remove member confirmation */}
      <AlertDialog open={!!removeTarget} onOpenChange={(open) => !open && setRemoveTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Remove member?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-medium text-foreground">{removeTarget?.name}</span> will lose access to{" "}
              <span className="font-medium text-foreground">{group.name}</span>. They can be re-added later.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className={cn("bg-destructive text-destructive-foreground hover:bg-destructive/90")}
              onClick={() => {
                if (removeTarget) {
                  setMembers((prev) => prev.filter((m) => m.id !== removeTarget.id))
                  setRemoveTarget(null)
                }
              }}
            >
              Remove
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Soft-delete confirmation */}
      <AlertDialog open={confirmDelete} onOpenChange={setConfirmDelete}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Delete this group?</AlertDialogTitle>
            <AlertDialogDescription>
              <span className="font-medium text-foreground">{group.name}</span> will be moved to a
              pending-deletion state. Members lose access immediately and the group is permanently
              removed after the retention window.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              className={cn("bg-destructive text-destructive-foreground hover:bg-destructive/90")}
              onClick={() => {
                setStatus("pending_deletion")
                setConfirmDelete(false)
              }}
            >
              Delete group
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
