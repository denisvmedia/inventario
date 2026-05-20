import { useMemo, useState } from "react"
import { Building2, Search, X, Users, Layers, ShieldCheck, Activity } from "lucide-react"
import { Input } from "@/components/ui/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination"
import { MOCK_TENANTS, TENANT_PLAN_CONFIG } from "@/data/mock"
import { TenantStatusBadge, fmtDate } from "./admin-shared"

const PAGE_SIZE = 8

interface TenantsViewProps {
  onSelectTenant?: (tenantId: string) => void
}

export function TenantsView({ onSelectTenant }: TenantsViewProps) {
  const [search, setSearch] = useState("")
  const [page, setPage] = useState(1)

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    if (!q) return MOCK_TENANTS
    return MOCK_TENANTS.filter(
      (t) =>
        t.name.toLowerCase().includes(q) ||
        t.slug.toLowerCase().includes(q) ||
        t.domain.toLowerCase().includes(q)
    )
  }, [search])

  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const currentPage = Math.min(page, pageCount)
  const rows = filtered.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE)

  const stats = useMemo(
    () => [
      { label: "Tenants", value: MOCK_TENANTS.length, icon: Building2 },
      {
        label: "Active",
        value: MOCK_TENANTS.filter((t) => t.status === "active").length,
        icon: Activity,
      },
      { label: "Total users", value: MOCK_TENANTS.reduce((s, t) => s + t.userCount, 0), icon: Users },
      { label: "Total groups", value: MOCK_TENANTS.reduce((s, t) => s + t.groupCount, 0), icon: Layers },
    ],
    []
  )

  return (
    <div className="flex flex-col gap-6 p-6 max-w-4xl mx-auto w-full">
      {/* Header */}
      <div className="flex items-start gap-3">
        <div className="flex size-9 items-center justify-center rounded-lg bg-primary/10 shrink-0">
          <ShieldCheck className="size-5 text-primary" />
        </div>
        <div>
          <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">Tenants</h1>
          <p className="mt-1 text-muted-foreground">
            Organisations using Inventario across the platform.
          </p>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
        {stats.map((s) => (
          <div key={s.label} className="rounded-xl border border-border bg-card px-4 py-3 flex items-center gap-3">
            <div className="flex size-8 items-center justify-center rounded-lg bg-muted shrink-0">
              <s.icon className="size-4 text-muted-foreground" />
            </div>
            <div>
              <p className="text-xs text-muted-foreground">{s.label}</p>
              <p className="text-lg font-semibold leading-tight">{s.value}</p>
            </div>
          </div>
        ))}
      </div>

      {/* Search */}
      <div className="relative">
        <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={search}
          onChange={(e) => {
            setSearch(e.target.value)
            setPage(1)
          }}
          placeholder="Search by name, slug or domain…"
          className="pl-8"
        />
        {search && (
          <button
            type="button"
            onClick={() => {
              setSearch("")
              setPage(1)
            }}
            className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
            aria-label="Clear search"
          >
            <X className="size-3.5" />
          </button>
        )}
      </div>

      {/* Table */}
      <div className="rounded-xl border border-border bg-card overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className="pl-4">Name</TableHead>
              <TableHead>Domain</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Plan</TableHead>
              <TableHead className="text-right">Users</TableHead>
              <TableHead className="text-right">Groups</TableHead>
              <TableHead className="pr-4">Created</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 && (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={7} className="h-32 text-center">
                  <div className="flex flex-col items-center justify-center gap-2">
                    <Building2 className="size-8 text-muted-foreground/30" />
                    <p className="text-sm text-muted-foreground">No tenants match your search.</p>
                  </div>
                </TableCell>
              </TableRow>
            )}
            {rows.map((tenant) => (
              <TableRow
                key={tenant.id}
                className="cursor-pointer"
                onClick={() => onSelectTenant?.(tenant.id)}
              >
                <TableCell className="pl-4 py-3.5">
                  <div className="flex flex-col">
                    <span className="text-sm font-medium">{tenant.name}</span>
                    <span className="font-mono text-xs text-muted-foreground">{tenant.slug}</span>
                  </div>
                </TableCell>
                <TableCell className="py-3.5 text-sm text-muted-foreground">{tenant.domain}</TableCell>
                <TableCell className="py-3.5">
                  <TenantStatusBadge status={tenant.status} />
                </TableCell>
                <TableCell className="py-3.5 text-sm">{TENANT_PLAN_CONFIG[tenant.plan].label}</TableCell>
                <TableCell className="py-3.5 text-right text-sm tabular-nums">{tenant.userCount}</TableCell>
                <TableCell className="py-3.5 text-right text-sm tabular-nums">{tenant.groupCount}</TableCell>
                <TableCell className="pr-4 py-3.5 text-sm text-muted-foreground">
                  {fmtDate(tenant.createdAt)}
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      {filtered.length > 0 && (
        <div className="flex items-center justify-between gap-4">
          <p className="text-xs text-muted-foreground">
            Showing{" "}
            <span className="font-medium text-foreground">
              {(currentPage - 1) * PAGE_SIZE + 1}–{Math.min(currentPage * PAGE_SIZE, filtered.length)}
            </span>{" "}
            of <span className="font-medium text-foreground">{filtered.length}</span>
          </p>
          <Pagination className="mx-0 w-auto justify-end">
            <PaginationContent>
              <PaginationItem>
                <PaginationPrevious
                  href="#"
                  onClick={(e) => {
                    e.preventDefault()
                    setPage((p) => Math.max(1, p - 1))
                  }}
                  className={currentPage === 1 ? "pointer-events-none opacity-50" : undefined}
                />
              </PaginationItem>
              {Array.from({ length: pageCount }, (_, i) => i + 1).map((p) => (
                <PaginationItem key={p}>
                  <PaginationLink
                    href="#"
                    isActive={p === currentPage}
                    onClick={(e) => {
                      e.preventDefault()
                      setPage(p)
                    }}
                  >
                    {p}
                  </PaginationLink>
                </PaginationItem>
              ))}
              <PaginationItem>
                <PaginationNext
                  href="#"
                  onClick={(e) => {
                    e.preventDefault()
                    setPage((p) => Math.min(pageCount, p + 1))
                  }}
                  className={currentPage === pageCount ? "pointer-events-none opacity-50" : undefined}
                />
              </PaginationItem>
            </PaginationContent>
          </Pagination>
        </div>
      )}
    </div>
  )
}
