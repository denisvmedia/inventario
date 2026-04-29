// Inventory of every "coming soon" surface in the new React frontend.
// One entry per stubbed feature: which lucide icon to show, which GitHub
// issue tracks the real implementation, and whether the surface is
// page-level or inline. Adding a new stub means adding one row here +
// the matching `surfaces.<key>.{title,description}` keys in stubs.json —
// component code never hard-codes a tracker number or icon.
//
// The TS union derived from this object also gives us a compile-time
// guarantee that every consumer references a known surface.
import type { LucideIcon } from "lucide-react"
import {
  BarChart3,
  Bell,
  CreditCard,
  Database,
  FileEdit,
  FileText,
  HardDrive,
  HelpCircle,
  History,
  Keyboard,
  Link2,
  MessageSquare,
  Monitor,
  ScrollText,
  Shield,
  ShieldCheck,
  Sparkles,
  User,
  Wrench,
} from "lucide-react"

export interface StubSurface {
  // Icon shown in the panel.
  icon: LucideIcon
  // GitHub issue tracking the real implementation. Rendered as
  // "Tracked under #NNNN" with a link to the issue.
  tracker: number
  // "page" — surface is a full route (`/plans`, `/help`, …).
  // "inline" — surface is a card/banner embedded on a real page
  //   (e.g. 2FA panel on /login, OAuth row, connected-accounts row).
  // "both" — used in either context.
  kind: "page" | "inline" | "both"
}

// SURFACES is the single source of truth. Order roughly mirrors the
// inventory in issue #1417 so the diff stays easy to scan.
export const SURFACES = {
  // Page-level stubs (full routes).
  plans: { icon: CreditCard, tracker: 1389, kind: "page" },
  helpCenter: { icon: HelpCircle, tracker: 1384, kind: "page" },
  helpShortcuts: { icon: Keyboard, tracker: 1385, kind: "page" },
  whatsNew: { icon: Sparkles, tracker: 1386, kind: "page" },
  insuranceReport: { icon: FileText, tracker: 1370, kind: "page" },

  // Inline-only stubs (used inside other pages once those pages land).
  twoFactor: { icon: Shield, tracker: 1380, kind: "inline" },
  oauth: { icon: Database, tracker: 1394, kind: "inline" },
  loginHistory: { icon: ScrollText, tracker: 1379, kind: "both" },
  activeSessions: { icon: Monitor, tracker: 1378, kind: "inline" },
  notificationPreferences: { icon: Bell, tracker: 1373, kind: "inline" },
  maintenanceReminders: { icon: Wrench, tracker: 1368, kind: "inline" },
  storageQuota: { icon: HardDrive, tracker: 1388, kind: "inline" },
  connectedAccounts: { icon: Link2, tracker: 1395, kind: "inline" },
  profilePhoto: { icon: User, tracker: 1382, kind: "inline" },
  multiStepDraft: { icon: FileEdit, tracker: 1383, kind: "inline" },
  sendFeedback: { icon: MessageSquare, tracker: 1387, kind: "both" },
  authStatsTeaser: { icon: BarChart3, tracker: 1390, kind: "inline" },
  warranties: { icon: ShieldCheck, tracker: 1367, kind: "inline" },
} as const satisfies Record<string, StubSurface>

export type SurfaceKey = keyof typeof SURFACES

// Repository the trackers live in. Hard-coded here rather than env-var'd
// because the frontend is bound to a single GitHub project; if that ever
// changes, this is the one place to update.
const TRACKER_BASE_URL = "https://github.com/denisvmedia/inventario/issues"

export function trackerUrl(surface: SurfaceKey): string {
  return `${TRACKER_BASE_URL}/${SURFACES[surface].tracker}`
}
