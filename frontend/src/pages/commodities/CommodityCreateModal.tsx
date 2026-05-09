import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router-dom"

import { CommodityFormDialog } from "@/components/items/CommodityFormDialog"
import { useAreas } from "@/features/areas/hooks"
import { useCreateCommodity } from "@/features/commodities/hooks"
import type { CreateCommodityRequest } from "@/features/commodities/api"
import { useCurrentGroup } from "@/features/group/GroupContext"
import { useAppToast } from "@/hooks/useAppToast"

// Radix Dialog's close animation (`data-[state=closed]:fade-out-0
// data-[state=closed]:zoom-out-95 duration-200` from ui/dialog.tsx)
// runs while `open` is false but before the content unmounts. The
// modal-overlay route is mounted only as long as React renders us,
// so calling `navigate(-1)` synchronously inside `onOpenChange(false)`
// would tear the DOM down before Radix could play the animation —
// the user sees the dialog disappear instantly. We defer the route
// pop until the animation has finished, matching the duration in the
// Dialog primitive.
const DIALOG_CLOSE_ANIMATION_MS = 200

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
  // Local `open` drives the Radix close animation: flipping to false
  // starts the `data-state=closed` transition, then the setTimeout
  // pops the route once the transition has played. Initial value is
  // true so the open animation runs on mount.
  const [open, setOpen] = useState(true)

  function close() {
    setOpen(false)
    // Pop back to whatever page the user came from. The modal-overlay
    // tree only mounts when `state.background` is present, so there
    // is always a previous entry to return to. `navigate(-1)` keeps
    // the URL exactly as the user had it (filters, query strings)
    // without us guessing the target.
    window.setTimeout(() => navigate(-1), DIALOG_CLOSE_ANIMATION_MS)
  }

  async function handleSubmit(values: CreateCommodityRequest) {
    const created = await create.mutateAsync(values)
    toast.success(t("commodities:toast.created"))
    if (slug && created?.id) {
      // Animate the dialog out before the route changes — matches the
      // close-without-submit path. `replace: true` so the back button
      // doesn't return to the (now stale) /commodities/new entry.
      // Rebind `slug` / `created.id` into closure-stable locals so TS
      // narrowing survives into the setTimeout callback.
      const targetSlug = slug
      const targetId = created.id
      setOpen(false)
      window.setTimeout(() => {
        navigate(
          `/g/${encodeURIComponent(targetSlug)}/commodities/${encodeURIComponent(targetId)}`,
          { replace: true }
        )
      }, DIALOG_CLOSE_ANIMATION_MS)
    } else {
      close()
    }
  }

  return (
    <CommodityFormDialog
      open={open}
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
