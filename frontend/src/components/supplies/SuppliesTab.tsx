import { ArrowDown, ArrowUp, ExternalLink, Pencil, Plus, Trash2 } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"

import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  useCreateSupplyLink,
  useDeleteSupplyLink,
  useReorderSupplyLinks,
  useSupplyLinksForCommodity,
  useUpdateSupplyLink,
} from "@/features/supplies/hooks"
import type { SupplyLinkEntity } from "@/features/supplies/api"
import { useAppToast } from "@/hooks/useAppToast"
import { useConfirm } from "@/hooks/useConfirm"
import { parseServerError } from "@/lib/server-error"

import { SupplyLinkDialog, type SupplyLinkFormValues } from "./SupplyLinkDialog"

interface SuppliesTabProps {
  commodityId: string
}

// SuppliesTab is the per-commodity "where do I re-buy the consumable"
// surface (#1369). Renders the supply links in sort_order with
// inline reorder up/down + edit + delete, plus a top-level "Add link"
// CTA that opens SupplyLinkDialog.
//
// Reordering uses array.splice + reorderSupplyLinks(ids) — the BE
// densely renumbers; the FE only needs to know the new visible order.
// We deliberately use up/down icon buttons rather than drag-and-drop
// to keep keyboard accessibility free and avoid a third dependency.
export function SuppliesTab({ commodityId }: SuppliesTabProps) {
  const { t } = useTranslation(["supplies", "common"])
  const toast = useAppToast()
  const confirm = useConfirm()
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<(SupplyLinkEntity & { id: string }) | null>(null)

  const list = useSupplyLinksForCommodity(commodityId)
  const create = useCreateSupplyLink()
  const update = useUpdateSupplyLink()
  const remove = useDeleteSupplyLink()
  const reorder = useReorderSupplyLinks()

  const links = useMemo(() => list.data?.links ?? [], [list.data])
  const orderedIDs = useMemo(() => links.map((l) => l.id), [links])

  async function handleCreate(values: SupplyLinkFormValues) {
    try {
      await create.mutateAsync({
        commodity_id: commodityId,
        label: values.label,
        url: values.url,
        notes: values.notes || undefined,
      })
      toast.success(
        t("supplies:toasts.created", { defaultValue: "Supply link added." })
      )
      setOpen(false)
    } catch (err) {
      toast.error(
        parseServerError(err) ??
          t("supplies:toasts.createError", { defaultValue: "Couldn't add the supply link." })
      )
    }
  }

  async function handleUpdate(values: SupplyLinkFormValues) {
    if (!editing) return
    try {
      await update.mutateAsync({
        commodity_id: commodityId,
        supply_id: editing.id,
        label: values.label,
        url: values.url,
        notes: values.notes,
      })
      toast.success(
        t("supplies:toasts.updated", { defaultValue: "Supply link updated." })
      )
      setEditing(null)
    } catch (err) {
      toast.error(
        parseServerError(err) ??
          t("supplies:toasts.updateError", { defaultValue: "Couldn't update the supply link." })
      )
    }
  }

  async function handleDelete(link: SupplyLinkEntity & { id: string }) {
    const ok = await confirm({
      title: t("supplies:delete.title", { defaultValue: "Delete supply link?" }),
      description: t("supplies:delete.description", {
        label: link.label,
        defaultValue: 'Remove "{{label}}" from this item\'s supply links?',
      }),
      confirmLabel: t("supplies:delete.confirm", { defaultValue: "Delete" }),
      destructive: true,
    })
    if (!ok) return
    try {
      await remove.mutateAsync({ commodity_id: commodityId, supply_id: link.id })
      toast.success(
        t("supplies:toasts.deleted", { defaultValue: "Supply link deleted." })
      )
    } catch (err) {
      toast.error(
        parseServerError(err) ??
          t("supplies:toasts.deleteError", { defaultValue: "Couldn't delete the supply link." })
      )
    }
  }

  async function move(index: number, delta: -1 | 1) {
    const next = index + delta
    if (next < 0 || next >= orderedIDs.length) return
    const ids = [...orderedIDs]
    ;[ids[index], ids[next]] = [ids[next], ids[index]]
    try {
      await reorder.mutateAsync({ commodity_id: commodityId, ids })
    } catch (err) {
      toast.error(
        parseServerError(err) ??
          t("supplies:toasts.reorderError", { defaultValue: "Couldn't reorder the supply links." })
      )
    }
  }

  return (
    <div className="space-y-4" data-testid="supplies-tab">
      <Card>
        <CardHeader className="flex flex-row items-center justify-between space-y-0">
          <CardTitle>{t("supplies:title", { defaultValue: "Supply links" })}</CardTitle>
          <Button
            type="button"
            size="sm"
            onClick={() => setOpen(true)}
            data-testid="supplies-add"
          >
            <Plus className="mr-1 size-4" aria-hidden="true" />
            {t("supplies:add", { defaultValue: "Add link" })}
          </Button>
        </CardHeader>
        <CardContent>
          {list.isLoading ? (
            <p className="text-sm text-muted-foreground">
              {t("common:loading", { defaultValue: "Loading…" })}
            </p>
          ) : links.length === 0 ? (
            <p className="text-sm text-muted-foreground" data-testid="supplies-empty">
              {t("supplies:empty", {
                defaultValue:
                  "No supply links yet. Add the URL where you re-buy this item's consumables.",
              })}
            </p>
          ) : (
            <ul className="divide-y" data-testid="supplies-list">
              {links.map((link, idx) => (
                <li
                  key={link.id}
                  className="flex items-center justify-between gap-3 py-3"
                  data-testid="supplies-row"
                  data-supply-id={link.id}
                >
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-sm font-medium" data-testid="supplies-row-label">
                      {link.label}
                    </p>
                    {link.notes ? (
                      <p
                        className="truncate text-xs text-muted-foreground"
                        data-testid="supplies-row-notes"
                      >
                        {link.notes}
                      </p>
                    ) : null}
                  </div>
                  <div className="flex shrink-0 items-center gap-1">
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      aria-label={t("supplies:moveUp", { defaultValue: "Move up" })}
                      disabled={idx === 0 || reorder.isPending}
                      onClick={() => move(idx, -1)}
                      data-testid="supplies-move-up"
                    >
                      <ArrowUp className="size-4" aria-hidden="true" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      aria-label={t("supplies:moveDown", { defaultValue: "Move down" })}
                      disabled={idx === links.length - 1 || reorder.isPending}
                      onClick={() => move(idx, 1)}
                      data-testid="supplies-move-down"
                    >
                      <ArrowDown className="size-4" aria-hidden="true" />
                    </Button>
                    <Button asChild type="button" variant="secondary" size="sm">
                      <a
                        href={link.url ?? "#"}
                        target="_blank"
                        rel="noopener noreferrer"
                        data-testid="supplies-row-open"
                      >
                        <ExternalLink className="mr-1 size-4" aria-hidden="true" />
                        {t("supplies:open", { defaultValue: "Open" })}
                      </a>
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      aria-label={t("supplies:edit", { defaultValue: "Edit supply link" })}
                      onClick={() => setEditing(link)}
                      data-testid="supplies-edit"
                    >
                      <Pencil className="size-4" aria-hidden="true" />
                    </Button>
                    <Button
                      type="button"
                      variant="ghost"
                      size="icon"
                      aria-label={t("supplies:delete.confirm", { defaultValue: "Delete" })}
                      onClick={() => handleDelete(link)}
                      data-testid="supplies-delete"
                    >
                      <Trash2 className="size-4" aria-hidden="true" />
                    </Button>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </CardContent>
      </Card>

      <SupplyLinkDialog
        open={open}
        onOpenChange={setOpen}
        title={t("supplies:add", { defaultValue: "Add link" })}
        onSubmit={handleCreate}
        busy={create.isPending}
      />
      <SupplyLinkDialog
        open={!!editing}
        onOpenChange={(v) => {
          if (!v) setEditing(null)
        }}
        title={t("supplies:edit", { defaultValue: "Edit supply link" })}
        initial={editing ?? undefined}
        onSubmit={handleUpdate}
        busy={update.isPending}
      />
    </div>
  )
}
