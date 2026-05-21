import { useMemo } from "react"
import { useTranslation } from "react-i18next"

import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
  PaginationLink,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination"

// Maximum numbered page links rendered before the list collapses to a
// windowed view with ellipses. 7 keeps the control to one line at the
// table's max width.
const MAX_PAGE_LINKS = 7

interface AdminPaginationProps {
  page: number
  totalPages: number
  total: number
  pageSize: number
  onPageChange: (page: number) => void
}

// Builds the visible page-number sequence. Short lists render every
// page; long lists window around the current page with `-1` sentinels
// standing in for ellipsis gaps (first + last always visible).
function buildPages(page: number, totalPages: number): number[] {
  if (totalPages <= MAX_PAGE_LINKS) {
    return Array.from({ length: totalPages }, (_, i) => i + 1)
  }
  const pages = new Set<number>([1, totalPages, page, page - 1, page + 1])
  const sorted = [...pages].filter((p) => p >= 1 && p <= totalPages).sort((a, b) => a - b)
  const out: number[] = []
  let prev = 0
  for (const p of sorted) {
    if (p - prev > 1) out.push(-1)
    out.push(p)
    prev = p
  }
  return out
}

// AdminPagination is the shared footer control for the paginated admin
// tables (tenants list, tenant-detail Users / Groups tabs). It wraps the
// shadcn <Pagination> primitive: a "showing X–Y of N" summary on the left
// and prev / numbered / next links on the right. Navigation is delegated
// to `onPageChange` — the caller persists the page in the URL.
export function AdminPagination({
  page,
  totalPages,
  total,
  pageSize,
  onPageChange,
}: AdminPaginationProps) {
  const { t } = useTranslation("admin")
  const pages = useMemo(() => buildPages(page, totalPages), [page, totalPages])

  const from = total === 0 ? 0 : (page - 1) * pageSize + 1
  const to = Math.min(page * pageSize, total)

  return (
    <div className="flex items-center justify-between gap-4" data-testid="admin-pagination">
      <p className="text-xs text-muted-foreground">
        {t("pagination.showing", {
          from,
          to,
          total,
        })}
      </p>
      <Pagination className="mx-0 w-auto justify-end">
        <PaginationContent>
          <PaginationItem>
            <PaginationPrevious
              href="#"
              aria-label={t("pagination.previous")}
              label={t("pagination.previousLabel")}
              onClick={(e) => {
                e.preventDefault()
                if (page > 1) onPageChange(page - 1)
              }}
              className={page <= 1 ? "pointer-events-none opacity-50" : undefined}
            />
          </PaginationItem>
          {pages.map((p, i) =>
            p === -1 ? (
              <PaginationItem key={`gap-${i}`}>
                <PaginationEllipsis />
              </PaginationItem>
            ) : (
              <PaginationItem key={p}>
                <PaginationLink
                  href="#"
                  isActive={p === page}
                  onClick={(e) => {
                    e.preventDefault()
                    onPageChange(p)
                  }}
                >
                  {p}
                </PaginationLink>
              </PaginationItem>
            )
          )}
          <PaginationItem>
            <PaginationNext
              href="#"
              aria-label={t("pagination.next")}
              label={t("pagination.nextLabel")}
              onClick={(e) => {
                e.preventDefault()
                if (page < totalPages) onPageChange(page + 1)
              }}
              className={page >= totalPages ? "pointer-events-none opacity-50" : undefined}
            />
          </PaginationItem>
        </PaginationContent>
      </Pagination>
    </div>
  )
}
