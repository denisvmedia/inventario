// Public surface for the MSW handler factories. Tests opt into the slices
// they need:
//
//     import { authHandlers, groupHandlers } from "@/test/handlers"
//     server.use(...authHandlers.signedIn(), ...groupHandlers.list([g1, g2]))
//
// Each per-feature module exports an object whose methods take focused
// fixtures (a user, a list of groups, a status code) and return an array
// of `msw` handlers — never a single handler — so a test can compose
// multiple slice variants without juggling spreads at the call site.
export * as authHandlers from "./auth"
export * as backofficeAuthHandlers from "./backoffice"
export * as groupHandlers from "./groups"
export * as commodityHandlers from "./commodities"
export * as commodityScanHandlers from "./commodityScan"
export * as areaHandlers from "./areas"
export * as locationHandlers from "./locations"
export * as fileHandlers from "./files"
export * as draftHandlers from "./drafts"
export * as intakeHandlers from "./intake"
export * as inboxHandlers from "./inbox"
export * as tagHandlers from "./tags"
export * as itemTypeHandlers from "./itemTypes"
export * as customFieldHandlers from "./customFields"
export * as loanHandlers from "./loans"
export * as maintenanceHandlers from "./maintenance"
export * as reminderHandlers from "./reminders"
export * as serviceHandlers from "./services"
export * as supplyHandlers from "./supplies"
export * as currencyMigrationHandlers from "./currencyMigrations"
export * as exportHandlers from "./exports"
export * as failedJobHandlers from "./failedJobs"
export * as memberHandlers from "./members"
export * as searchHandlers from "./search"
export * as savedViewHandlers from "./savedViews"
export * as sellerHandlers from "./sellers"
export * as dataExportHandlers from "./dataExports"
export * as settingsHandlers from "./settings"
export * as featureFlagHandlers from "./featureFlags"

// Helper used across factories: every backend route lives under /api/v1/.
// Re-exported so per-test handlers that aren't worth a factory can build
// URLs the same way the factories do.
export const apiUrl = (path: string) => `${window.location.origin}/api/v1${path}`

// The page of `items` a paged list endpoint answers with, read the way the
// server's parsePagination reads the query: page defaults to 1, per_page to
// 50, and a per_page above 100 falls back to 50.
export function serverPage<T>(
  url: string,
  items: T[]
): { rows: T[]; page: number; perPage: number; totalPages: number } {
  const params = new URL(url).searchParams
  const pageParam = Number(params.get("page"))
  const perPageParam = Number(params.get("per_page"))
  const page = Number.isInteger(pageParam) && pageParam > 0 ? pageParam : 1
  const perPage =
    Number.isInteger(perPageParam) && perPageParam > 0 && perPageParam <= 100 ? perPageParam : 50
  const start = (page - 1) * perPage
  return {
    rows: items.slice(start, start + perPage),
    page,
    perPage,
    totalPages: Math.ceil(items.length / perPage),
  }
}
