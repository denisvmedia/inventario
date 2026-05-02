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
export * as groupHandlers from "./groups"
export * as commodityHandlers from "./commodities"
export * as areaHandlers from "./areas"
export * as locationHandlers from "./locations"
export * as fileHandlers from "./files"
export * as tagHandlers from "./tags"
export * as exportHandlers from "./exports"
export * as memberHandlers from "./members"
export * as searchHandlers from "./search"

// Helper used across factories: every backend route lives under /api/v1/.
// Re-exported so per-test handlers that aren't worth a factory can build
// URLs the same way the factories do.
export const apiUrl = (path: string) => `${window.location.origin}/api/v1${path}`
