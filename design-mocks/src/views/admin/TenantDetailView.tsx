import { Building2, Globe, Hash, Users, Layers } from "lucide-react"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  tenantById,
  usersByTenant,
  groupsByTenant,
  TENANT_PLAN_CONFIG,
  CURRENCIES,
} from "@/data/mock"
import {
  AdminBackButton,
  TenantStatusBadge,
  GroupStatusBadge,
  AccountStateBadge,
  RoleBadge,
  fmtDateTime,
} from "./admin-shared"

interface TenantDetailViewProps {
  tenantId: string
  onBack?: () => void
  onSelectUser?: (userId: string) => void
  onSelectGroup?: (groupId: string) => void
}

export function TenantDetailView({
  tenantId,
  onBack,
  onSelectUser,
  onSelectGroup,
}: TenantDetailViewProps) {
  const tenant = tenantById(tenantId)

  if (!tenant) {
    return (
      <div className="flex flex-col gap-6 p-6 max-w-4xl mx-auto w-full">
        <AdminBackButton label="Back to tenants" onClick={onBack} />
        <div className="flex flex-col items-center justify-center gap-3 py-24">
          <Building2 className="size-8 text-muted-foreground/30" />
          <p className="text-sm text-muted-foreground">Tenant not found.</p>
        </div>
      </div>
    )
  }

  const users = usersByTenant(tenant.id)
  const groups = groupsByTenant(tenant.id)

  return (
    <div className="flex flex-col gap-6 p-6 max-w-4xl mx-auto w-full">
      <AdminBackButton label="Back to tenants" onClick={onBack} />

      {/* Header card */}
      <div className="rounded-xl border border-border bg-card p-6">
        <div className="flex items-start gap-4">
          <div className="flex size-12 items-center justify-center rounded-xl bg-primary/10 shrink-0">
            <Building2 className="size-6 text-primary" />
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-2xl font-semibold tracking-tight">{tenant.name}</h1>
              <TenantStatusBadge status={tenant.status} />
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-sm text-muted-foreground">
              <span className="inline-flex items-center gap-1.5">
                <Hash className="size-3.5" />
                <span className="font-mono text-xs">{tenant.slug}</span>
              </span>
              <span className="inline-flex items-center gap-1.5">
                <Globe className="size-3.5" />
                {tenant.domain}
              </span>
            </div>
          </div>
        </div>

        <div className="mt-5 grid grid-cols-2 gap-3 sm:grid-cols-4">
          {[
            { label: "Plan", value: TENANT_PLAN_CONFIG[tenant.plan].label },
            { label: "Users", value: String(tenant.userCount) },
            { label: "Groups", value: String(tenant.groupCount) },
            {
              label: "Created",
              value: new Date(tenant.createdAt).toLocaleDateString("en-US", {
                month: "short",
                year: "numeric",
              }),
            },
          ].map((s) => (
            <div key={s.label} className="rounded-lg border border-border bg-muted/40 px-3 py-2.5">
              <p className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
                {s.label}
              </p>
              <p className="mt-0.5 text-sm font-semibold">{s.value}</p>
            </div>
          ))}
        </div>
      </div>

      {/* Tabs */}
      <Tabs defaultValue="users">
        <TabsList>
          <TabsTrigger value="users">
            <Users className="size-3.5" />
            Users
          </TabsTrigger>
          <TabsTrigger value="groups">
            <Layers className="size-3.5" />
            Groups
          </TabsTrigger>
        </TabsList>

        {/* Users tab */}
        <TabsContent value="users">
          <div className="rounded-xl border border-border bg-card overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="pl-4">Name</TableHead>
                  <TableHead>Email</TableHead>
                  <TableHead>Role</TableHead>
                  <TableHead>State</TableHead>
                  <TableHead className="pr-4">Last login</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.length === 0 && (
                  <TableRow className="hover:bg-transparent">
                    <TableCell colSpan={5} className="h-24 text-center text-sm text-muted-foreground">
                      No users in this tenant.
                    </TableCell>
                  </TableRow>
                )}
                {users.map((user) => (
                  <TableRow
                    key={user.id}
                    className="cursor-pointer"
                    onClick={() => onSelectUser?.(user.id)}
                  >
                    <TableCell className="pl-4 py-3.5">
                      <div className="flex items-center gap-2.5">
                        <div className="flex size-7 items-center justify-center rounded-full bg-muted text-xs font-semibold text-muted-foreground shrink-0">
                          {user.avatarInitials}
                        </div>
                        <span className="text-sm font-medium">{user.name}</span>
                      </div>
                    </TableCell>
                    <TableCell className="py-3.5 text-sm text-muted-foreground">{user.email}</TableCell>
                    <TableCell className="py-3.5">
                      <RoleBadge role={user.role} />
                    </TableCell>
                    <TableCell className="py-3.5">
                      <AccountStateBadge active={user.isActive} />
                    </TableCell>
                    <TableCell className="pr-4 py-3.5 text-sm text-muted-foreground">
                      {fmtDateTime(user.lastLogin)}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </TabsContent>

        {/* Groups tab */}
        <TabsContent value="groups">
          <div className="rounded-xl border border-border bg-card overflow-hidden">
            <Table>
              <TableHeader>
                <TableRow className="hover:bg-transparent">
                  <TableHead className="pl-4">Group</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Currency</TableHead>
                  <TableHead className="text-right pr-4">Members</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {groups.length === 0 && (
                  <TableRow className="hover:bg-transparent">
                    <TableCell colSpan={4} className="h-24 text-center text-sm text-muted-foreground">
                      No groups in this tenant.
                    </TableCell>
                  </TableRow>
                )}
                {groups.map((group) => {
                  const currency = CURRENCIES.find((c) => c.code === group.currency)
                  return (
                    <TableRow
                      key={group.id}
                      className="cursor-pointer"
                      onClick={() => onSelectGroup?.(group.id)}
                    >
                      <TableCell className="pl-4 py-3.5 text-sm font-medium">{group.name}</TableCell>
                      <TableCell className="py-3.5">
                        <GroupStatusBadge status={group.status} />
                      </TableCell>
                      <TableCell className="py-3.5 text-sm text-muted-foreground">
                        {group.currency}
                        {currency ? ` · ${currency.name}` : ""}
                      </TableCell>
                      <TableCell className="pr-4 py-3.5 text-right text-sm tabular-nums">
                        {group.memberCount}
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          </div>
        </TabsContent>
      </Tabs>
    </div>
  )
}
