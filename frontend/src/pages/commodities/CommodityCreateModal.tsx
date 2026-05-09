import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { useAreas } from "@/features/areas/hooks"
import { useCreateCommodity } from "@/features/commodities/hooks"
import type { CreateCommodityRequest } from "@/features/commodities/api"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"

// CommodityCreateModalRoute mounts the create dialog as a standalone
// modal-overlay route at /g/:slug/commodities/new (#1546 modal-routes
// pattern). The router renders this only when the navigation that
// pushed us here carried `state.background` — i.e. the user clicked
// the sidebar Add-item / Dashboard mobile CTA from any other page.
// In that case the main <Routes> tree resolves against `background`
// (the page they were on) and stays mounted as the backdrop, while
// this component renders just the dialog on top.
//
// Direct deep-links to /commodities/new (refresh, "open in new tab",
// shared URL) carry no `background`; the router falls back to mounting
// CommoditiesListPage which then opens the same dialog as a side
// effect (see CommoditiesListPage's `isCreateRoute` block). Both
// surfaces share `CommodityFormDialog`.
export function CommodityCreateModalRoute() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug
  const areas = useAreas()
  const create = useCreateCommodity()
  const toast = useAppToast()

  function close() {
    // Pop back to whatever page the user came from. The modal-overlay
    // tree only mounts when `state.background` is present, so there
    // is always a previous entry to return to. `replace: false` would
    // also work, but `navigate(-1)` keeps the URL exactly as the user
    // had it (filters, query strings) without us guessing the target.
    navigate(-1)
  }

  async function handleSubmit(values: CreateCommodityRequest) {
    const created = await create.mutateAsync(values)
    toast.success(t("commodities:toast.created"))
    if (slug && created?.id) {
      // Drop the user on the new item's detail. `replace: true` so the
      // back button doesn't return to the (now stale) /commodities/new
      // entry — they go back to the page they were on before the
      // dialog opened.
      navigate(`/g/${encodeURIComponent(slug)}/commodities/${encodeURIComponent(created.id)}`, {
        replace: true,
      })
    } else {
      close()
    }
  }

  return (
    <CommodityFormDialog
      open
      onOpenChange={(o) => {
        if (!o) close()
      }}
      mode="create"
      areas={areas.data ?? []}
      defaultCurrency={currentGroup?.group_currency ?? "USD"}
      onSubmit={handleSubmit}
      isPending={create.isPending}
      draftKey={slug ? `commodity-draft:${slug}:create` : undefined}
    />
  )
}
