// WARRANTY_STATUS_CONFIG is the single source of truth for the
// per-status colour binding + icon used by every warranty surface
// (badge, list rows, dashboard panel, item-detail tab). Pulled out of
// `features/commodities/constants.ts` so non-commodity surfaces can
// import from a stable path without dragging the list-page filter
// helpers along. Class strings reference the design tokens declared
// in src/index.css (`--status-active`, `--status-expiring`,
// `--status-expired`, `--status-none`); `bgSolid` is the unbordered
// background variant the dashboard's progress bar fills with — the
// regular `bg` is the tinted card surface.
import { Shield, ShieldAlert, ShieldCheck, ShieldOff, type LucideIcon } from "lucide-react"

import type { CommodityWarrantyStatus } from "@/features/commodities/constants"

export interface WarrantyStatusVisual {
  // i18n key that resolves to the short human label (e.g. "Active",
  // "Expiring soon"). The `commodities:warranty.*` namespace already
  // ships the short forms — `warrantyStatus.*` exists too but its
  // values are sentence-y ("Warranty active") which reads off inside
  // a chip.
  i18nKey: `commodities:warranty.${CommodityWarrantyStatus}`
  // Lucide icon for the status — shield variants per the mock.
  icon: LucideIcon
  // Foreground colour class. `text-status-*` tokens already live in
  // index.css for both light + dark themes.
  text: string
  // Tinted background class used by cards / chip backdrops.
  bg: string
  // Solid `bg-status-*` (no /N opacity). Used by the dashboard's
  // proportional progress bars where a tinted background reads as
  // washed-out.
  bgSolid: string
  // Border colour class for outlined badges.
  border: string
}

export const WARRANTY_STATUS_CONFIG: Record<CommodityWarrantyStatus, WarrantyStatusVisual> = {
  active: {
    i18nKey: "commodities:warranty.active",
    icon: ShieldCheck,
    text: "text-status-active",
    bg: "bg-status-active/10",
    bgSolid: "bg-status-active",
    border: "border-status-active/30",
  },
  expiring: {
    i18nKey: "commodities:warranty.expiring",
    icon: ShieldAlert,
    text: "text-status-expiring",
    bg: "bg-status-expiring/10",
    bgSolid: "bg-status-expiring",
    border: "border-status-expiring/30",
  },
  expired: {
    i18nKey: "commodities:warranty.expired",
    icon: ShieldOff,
    text: "text-status-expired",
    bg: "bg-status-expired/10",
    bgSolid: "bg-status-expired",
    border: "border-status-expired/30",
  },
  none: {
    i18nKey: "commodities:warranty.none",
    icon: Shield,
    text: "text-status-none",
    bg: "bg-status-none/10",
    bgSolid: "bg-status-none",
    border: "border-status-none/30",
  },
}
