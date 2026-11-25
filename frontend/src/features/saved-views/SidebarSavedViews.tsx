import { useTranslation } from "react-i18next"
import { Link, useLocation } from "react-router-dom"
import { AlertTriangle, Settings2 } from "lucide-react"

import {
  SidebarGroup,
  SidebarGroupAction,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuBadge,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar"
import { useCurrentGroup } from "@/features/group/GroupContext"

import { useSavedViewCounts, useSavedViews } from "./hooks"
import { SavedViewIcon } from "./SavedViewIcon"
import { SAVED_VIEW_PARAM, savedViewHref } from "./view"

interface SidebarSavedViewsProps {
  onNavigate?: () => void
}

// The sidebar section listing the saved views a member pinned there, each
// with the number of items it selects. The count comes from the server's
// count of the same query the list runs, so it matches the opened list.
// A view whose stored query cannot run is listed but cannot be opened, as
// in the items list's menu. Hidden while the member has no view in the
// sidebar.
export function SidebarSavedViews({ onNavigate }: SidebarSavedViewsProps) {
  const { t } = useTranslation()
  const { currentGroup } = useCurrentGroup()
  const slug = currentGroup?.slug ?? ""
  const location = useLocation()
  const savedViews = useSavedViews({ enabled: slug !== "" })
  const pinned = (savedViews.data?.views ?? []).filter((v) => v.showInSidebar)
  const counts = useSavedViewCounts({ enabled: pinned.length > 0 })

  if (!slug || pinned.length === 0) return null

  const onList = location.pathname === `/g/${encodeURIComponent(slug)}/commodities`
  const activeId = onList ? new URLSearchParams(location.search).get(SAVED_VIEW_PARAM) : null

  return (
    <SidebarGroup data-testid="sidebar-saved-views">
      <SidebarGroupLabel>{t("common:nav.groupSavedViews")}</SidebarGroupLabel>
      <SidebarGroupAction asChild title={t("commodities:savedViews.menu.manage")}>
        <Link
          to={`/g/${encodeURIComponent(slug)}/views`}
          onClick={onNavigate}
          aria-label={t("commodities:savedViews.menu.manage")}
          data-testid="sidebar-saved-views-manage"
        >
          <Settings2 aria-hidden="true" />
        </Link>
      </SidebarGroupAction>
      <SidebarGroupContent>
        <SidebarMenu>
          {pinned.map((view) => {
            const count = counts.data?.[view.id]
            const href = savedViewHref(slug, view)
            if (!href) {
              return (
                <SidebarMenuItem key={view.id}>
                  <SidebarMenuButton
                    aria-disabled="true"
                    data-testid="sidebar-saved-view-unavailable"
                  >
                    <SavedViewIcon name={view.icon} className="size-4" />
                    <span className="truncate">{view.name}</span>
                    <AlertTriangle
                      role="img"
                      className="ml-auto text-status-expiring"
                      aria-label={t("commodities:savedViews.queryError.short")}
                    />
                  </SidebarMenuButton>
                </SidebarMenuItem>
              )
            }
            return (
              <SidebarMenuItem key={view.id}>
                <SidebarMenuButton
                  asChild
                  tooltip={view.name}
                  isActive={view.id === activeId}
                  className={count !== undefined ? "pr-8" : undefined}
                >
                  <Link to={href} onClick={onNavigate} data-testid="sidebar-saved-view">
                    <SavedViewIcon name={view.icon} className="size-4" />
                    <span>{view.name}</span>
                  </Link>
                </SidebarMenuButton>
                {count !== undefined ? (
                  <SidebarMenuBadge data-testid="sidebar-saved-view-count">
                    {count}
                  </SidebarMenuBadge>
                ) : null}
              </SidebarMenuItem>
            )
          })}
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )
}
