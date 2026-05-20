import { useMemo, useState } from "react"
import { Layers, Search, X } from "lucide-react"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
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
import { MOCK_ADMIN_GROUPS, MOCK_TENANTS } from "@/data/mock"
import { TenantChip, GroupStatusBadge, fmtDate } from "./admin-shared"

const PAGE_SIZE = 8

interface GroupsViewProps {
  onSelectGroup?: (groupId: string) => void
}

export function GroupsView({ onSelectGroup }: GroupsViewProps) {
  const [search, setSearch] = useState("")
  const [tenantFilter, setTenantFilter] = useState<string>("all")
  const [statusFilter, setStatusFilter] = useState<string>("all")
  const [page, setPage] = useState(1)

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase()
    return MOCK_ADMIN_GROUPS.filter((g) => {
      if (q && !g.name.toLowerCase().includes(q)) return false
      if (tenantFilter !== "all" && g.tenantId !== tenantFilter) return false
      if (statusFilter !== "all" && g.status !== statusFilter) return false
      return true
    })
  }, [search, tenantFilter, statusFilter])

  const pageCount = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const currentPage = Math.min(page, pageCount)
  const rows = filtered.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE)

  function resetPage() {
    setPage(1)
  }

  return (
    <div className="flex flex-col gap-6 p-6 max-w-4xl mx-auto w-full">
      {/* Header */}
      <div className="flex items-start gap-3">
        <div className="flex size-9 items-center justify-center rounded-lg bg-primary/10 shrink-0">
          <Layers className="size-5 text-primary" />
        </div>
        <div>
          <h1 className="scroll-m-20 text-3xl font-semibold tracking-tight">Groups</h1>
          <p className="mt-1 text-muted-foreground">
            Every inventory group across all tenants.
          </p>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-col gap-3 sm:flex-row">
        <div className="relative flex-1">
          <Search className="absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            value={search}
            onChange={(e) => {
              setSearch(e.target.value)
              resetPage()
            }}
            placeholder="Search groups…"
            className="pl-8"
          />
          {search && (
            <button
              type="button"
              onClick={() => {
                setSearch("")
                resetPage()
              }}
              className="absolute right-2.5 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
              aria-label="Clear search"
            >
              <X className="size-3.5" />
            </button>
          )}
        </div>
        <Select
          value={tenantFilter}
          onValueChange={(v) => {
            setTenantFilter(v)
            resetPage()
          }}
        >
          <SelectTrigger className="sm:w-52">
            <SelectValue placeholder="All tenants" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All tenants</SelectItem>
            {MOCK_TENANTS.map((t) => (
              <SelectItem key={t.id} value={t.id}>
                {t.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={statusFilter}
          onValueChange={(v) => {
            setStatusFilter(v)
            resetPage()
          }}
        >
          <SelectTrigger className="sm:w-44">
            <SelectValue placeholder="All statuses" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">All statuses</SelectItem>
            <SelectItem value="active">Active</SelectItem>
            <SelectItem value="pending_deletion">Pending Deletion</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {/* Table */}
      <div className="rounded-xl border border-border bg-card overflow-hidden">
        <Table>
          <TableHeader>
            <TableRow className="hover:bg-transparent">
              <TableHead className="pl-4">Group</TableHead>
              <TableHead>Tenant</TableHead>
              <TableHead>Status</TableHead>
              <TableHead>Currency</TableHead>
              <TableHead className="text-right">Members</TableHead>
              <TableHead className="pr-4">Created</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.length === 0 && (
              <TableRow className="hover:bg-transparent">
                <TableCell colSpan={6} className="h-32 text-center">
                  <div className="flex flex-col items-center justify-center gap-2">
                    <Layers className="size-8 text-muted-foreground/30" />
                    <p className="text-sm text-muted-foreground">No groups match your filters.</p>
                  </div>
                </TableCell>
              </TableRow>
            )}
            {rows.map((group) => (
              <TableRow
                key={group.id}
                className="cursor-pointer"
                onClick={() => onSelectGroup?.(group.id)}
              >
                <TableCell className="pl-4 py-3.5 text-sm font-medium">{group.name}</TableCell>
                <TableCell className="py-3.5">
                  <TenantChip tenantId={group.tenantId} />
                </TableCell>
                <TableCell className="py-3.5">
                  <GroupStatusBadge status={group.status} />
                </TableCell>
                <TableCell className="py-3.5 text-sm text-muted-foreground">{group.currency}</TableCell>
                <TableCell className="py-3.5 text-right text-sm tabular-nums">{group.memberCount}</TableCell>
                <TableCell className="pr-4 py-3.5 text-sm text-muted-foreground">
                  {fmtDate(group.createdAt)}
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
