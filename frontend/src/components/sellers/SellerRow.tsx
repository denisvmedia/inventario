import { ExternalLink, Pencil, Store, Trash2 } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router-dom"

import { Button } from "@/components/ui/button"
import type { Seller } from "@/features/sellers/api"
import { cn } from "@/lib/utils"

export interface SellerRowProps {
  seller: Seller
  // The items list of everything bought from the seller.
  href?: string
  // Left out for a member who may not change it; the row then has no
  // edit button. The same holds for onDelete.
  onEdit?: () => void
  onDelete?: () => void
  className?: string
}

// One row of the Sellers page (#2804). The mock has no sellers; the row
// follows TagRow (design-mocks TagsView): an icon, the name with the item
// count beneath, and edit and delete buttons that fade in on hover.
export function SellerRow({ seller, href, onEdit, onDelete, className }: SellerRowProps) {
  const { t } = useTranslation()
  const items = seller.item_count ?? 0
  const name = seller.name ?? ""
  const details = [
    items > 0 ? t("sellers:list.usageItems", { count: items }) : t("sellers:list.usageNone"),
    seller.contact,
  ].filter(Boolean)

  return (
    <div
      data-testid={`seller-row-${seller.id}`}
      className={cn("group flex items-center gap-3 px-4 py-3", className)}
    >
      <Store aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />

      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          {href ? (
            <Link
              to={href}
              className="truncate text-sm font-medium hover:underline"
              data-testid={`seller-row-${seller.id}-link`}
            >
              {name}
            </Link>
          ) : (
            <span className="truncate text-sm font-medium">{name}</span>
          )}
          {seller.website ? (
            <a
              href={seller.website}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex min-w-0 items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
              data-testid={`seller-row-${seller.id}-website`}
            >
              <ExternalLink aria-hidden="true" className="size-3 shrink-0" />
              <span className="truncate">{hostOf(seller.website)}</span>
            </a>
          ) : null}
        </div>
        <p
          className="mt-0.5 truncate text-xs text-muted-foreground"
          data-testid={`seller-row-${seller.id}-usage`}
        >
          {details.join(" · ")}
        </p>
        {seller.note ? (
          <p className="mt-0.5 line-clamp-2 text-xs text-muted-foreground/80">{seller.note}</p>
        ) : null}
      </div>

      {onEdit || onDelete ? (
        <div className="flex shrink-0 gap-0.5 opacity-0 transition-opacity group-hover:opacity-100 focus-within:opacity-100">
          {onEdit ? (
            <Button
              type="button"
              size="icon"
              variant="ghost"
              className="size-7"
              onClick={onEdit}
              aria-label={t("sellers:list.edit", { name })}
              data-testid={`seller-row-${seller.id}-edit`}
            >
              <Pencil aria-hidden="true" className="size-3.5" />
            </Button>
          ) : null}
          {onDelete ? (
            <Button
              type="button"
              size="icon"
              variant="ghost"
              className="size-7 text-destructive hover:bg-destructive/10 hover:text-destructive"
              onClick={onDelete}
              aria-label={t("sellers:list.delete", { name })}
              data-testid={`seller-row-${seller.id}-delete`}
            >
              <Trash2 aria-hidden="true" className="size-3.5" />
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}

// The website's host, which reads better in a row than the whole URL.
function hostOf(url: string): string {
  try {
    return new URL(url).host
  } catch {
    return url
  }
}
