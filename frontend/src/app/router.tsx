import { Suspense, lazy } from "react"
import { Navigate, Outlet, Route, Routes, useLocation, useParams } from "react-router-dom"
import type { Location } from "react-router-dom"

import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ProtectedRoute } from "@/components/routing/ProtectedRoute"
import { GroupRequiredRoute } from "@/components/routing/GroupRequiredRoute"
import { UngroupedRedirect } from "@/components/routing/UngroupedRedirect"
import { ComingSoonPage } from "@/components/coming-soon"
import { Shell } from "@/app/Shell"
import { ConfirmProvider } from "@/hooks/useConfirm"

// Real pages are code-split — each one becomes its own chunk so adding a
// new real page later doesn't grow the entry bundle.
const DashboardPage = lazy(() =>
  import("@/pages/Dashboard").then((m) => ({ default: m.DashboardPage }))
)
const LocationsListPage = lazy(() =>
  import("@/pages/locations/LocationsListPage").then((m) => ({ default: m.LocationsListPage }))
)
const LocationDetailPage = lazy(() =>
  import("@/pages/locations/LocationDetailPage").then((m) => ({ default: m.LocationDetailPage }))
)
const AreaDetailPage = lazy(() =>
  import("@/pages/areas/AreaDetailPage").then((m) => ({ default: m.AreaDetailPage }))
)
const NotFoundPage = lazy(() =>
  import("@/pages/NotFound").then((m) => ({ default: m.NotFoundPage }))
)
const MaintenancePage = lazy(() =>
  import("@/pages/MaintenancePage").then((m) => ({ default: m.MaintenancePage }))
)
// UI Showcase is a dev-only design-system reference (#1542). Only chunked
// in when the build is a dev build — `import.meta.env.DEV` is statically
// dead-code-eliminable by Vite, so production bundles don't carry the
// showcase code at all.
const UIShowcasePage = import.meta.env.DEV
  ? lazy(() => import("@/pages/UIShowcasePage").then((m) => ({ default: m.UIShowcasePage })))
  : null
const RootRedirect = lazy(() =>
  import("@/pages/RootRedirect").then((m) => ({ default: m.RootRedirect }))
)
const LoginPage = lazy(() =>
  import("@/pages/auth/LoginPage").then((m) => ({ default: m.LoginPage }))
)
const RegisterPage = lazy(() =>
  import("@/pages/auth/RegisterPage").then((m) => ({ default: m.RegisterPage }))
)
const ForgotPasswordPage = lazy(() =>
  import("@/pages/auth/ForgotPasswordPage").then((m) => ({ default: m.ForgotPasswordPage }))
)
const ResetPasswordPage = lazy(() =>
  import("@/pages/auth/ResetPasswordPage").then((m) => ({ default: m.ResetPasswordPage }))
)
const VerifyEmailPage = lazy(() =>
  import("@/pages/auth/VerifyEmailPage").then((m) => ({ default: m.VerifyEmailPage }))
)
const InviteAcceptPage = lazy(() =>
  import("@/pages/auth/InviteAcceptPage").then((m) => ({ default: m.InviteAcceptPage }))
)
const NoGroupPage = lazy(() =>
  import("@/pages/NoGroupPage").then((m) => ({ default: m.NoGroupPage }))
)
const ProfilePage = lazy(() =>
  import("@/pages/ProfilePage").then((m) => ({ default: m.ProfilePage }))
)
const EditProfilePage = lazy(() =>
  import("@/pages/EditProfilePage").then((m) => ({ default: m.EditProfilePage }))
)
const SettingsPage = lazy(() =>
  import("@/pages/SettingsPage").then((m) => ({ default: m.SettingsPage }))
)
const SessionsPage = lazy(() =>
  import("@/pages/SessionsPage").then((m) => ({ default: m.SessionsPage }))
)
const LoginHistoryPage = lazy(() =>
  import("@/pages/LoginHistoryPage").then((m) => ({ default: m.LoginHistoryPage }))
)
const CreateGroupPage = lazy(() =>
  import("@/pages/groups/CreateGroupPage").then((m) => ({ default: m.CreateGroupPage }))
)
const GroupSettingsPage = lazy(() =>
  import("@/pages/groups/GroupSettingsPage").then((m) => ({ default: m.GroupSettingsPage }))
)
const MembersPage = lazy(() =>
  import("@/pages/groups/MembersPage").then((m) => ({ default: m.MembersPage }))
)
const SearchPage = lazy(() => import("@/pages/SearchPage").then((m) => ({ default: m.SearchPage })))
const CommoditiesListPage = lazy(() =>
  import("@/pages/commodities/CommoditiesListPage").then((m) => ({
    default: m.CommoditiesListPage,
  }))
)
const CommodityDetailPage = lazy(() =>
  import("@/pages/commodities/CommodityDetailPage").then((m) => ({
    default: m.CommodityDetailPage,
  }))
)
// CommodityDetailSheet is the overlay variant of the same surface
// (#1546). Mounted as a *modal* route — i.e. rendered alongside the
// list backdrop when the navigation carried `state.background`.
const CommodityDetailSheet = lazy(() =>
  import("@/pages/commodities/CommodityDetailPage").then((m) => ({
    default: m.CommodityDetailSheet,
  }))
)
// CommodityCreateModalRoute renders the create dialog as a modal
// overlay over whatever page the user was on (Dashboard, Locations,
// etc.). Mounted only when navigation to /commodities/new carried
// `state.background`; direct deep-links to /commodities/new fall
// through to CommoditiesListPage.
const CommodityCreateModalRoute = lazy(() =>
  import("@/pages/commodities/CommodityCreateModal").then((m) => ({
    default: m.CommodityCreateModalRoute,
  }))
)
const CommodityPrintPage = lazy(() =>
  import("@/pages/commodities/CommodityPrintPage").then((m) => ({
    default: m.CommodityPrintPage,
  }))
)
const FilesListPage = lazy(() =>
  import("@/pages/files/FilesListPage").then((m) => ({ default: m.FilesListPage }))
)
const FileEditPage = lazy(() =>
  import("@/pages/files/FileEditPage").then((m) => ({ default: m.FileEditPage }))
)
const TagsListPage = lazy(() =>
  import("@/pages/tags/TagsListPage").then((m) => ({ default: m.TagsListPage }))
)
const LoansListPage = lazy(() =>
  import("@/pages/loans/LoansListPage").then((m) => ({ default: m.LoansListPage }))
)
const ServicesListPage = lazy(() =>
  import("@/pages/services/ServicesListPage").then((m) => ({ default: m.ServicesListPage }))
)
const WarrantiesListPage = lazy(() =>
  import("@/pages/warranties/WarrantiesListPage").then((m) => ({ default: m.WarrantiesListPage }))
)
const ExportsListPage = lazy(() =>
  import("@/pages/exports/ExportsListPage").then((m) => ({ default: m.ExportsListPage }))
)
const ExportNewPage = lazy(() =>
  import("@/pages/exports/ExportNewPage").then((m) => ({ default: m.ExportNewPage }))
)
const ExportDetailPage = lazy(() =>
  import("@/pages/exports/ExportDetailPage").then((m) => ({ default: m.ExportDetailPage }))
)
const ExportImportPage = lazy(() =>
  import("@/pages/exports/ExportImportPage").then((m) => ({ default: m.ExportImportPage }))
)
const ExportRestorePage = lazy(() =>
  import("@/pages/exports/ExportRestorePage").then((m) => ({ default: m.ExportRestorePage }))
)

// AppRoutes is the full route tree for the new React frontend.
//
// Tree layout, top-down:
//   - public (no auth required): login, register, forgot-password,
//     reset-password, verify-email, invite/:token.
//   - auth-required, with the GroupProvider mounted so any descendant can
//     use useCurrentGroup and the http client picks up the URL slug:
//     - group-exempt onboarding-friendly routes (mirrors the legacy
//       GROUP_EXEMPT_ROUTE_NAMES list): no-group, profile, groups/new,
//       groups/:id/settings, plus the mock-only "coming soon" stubs that
//       don't need a group (plans, help/shortcuts, whats-new).
//     - group-required routes: / (redirects to /g/<first-slug> or /no-group)
//       and the entire /g/:groupSlug/* subtree.
//   - catch-all: NotFoundPage.
//
// AuthProvider sits above <Routes> in providers.tsx; mounting it inside
// <Routes> would re-create the auth context every navigation.
export function AppRoutes() {
  // #1546 modal-routes pattern. When the navigation that landed us
  // here carried `state.background` (the items list pushes one when a
  // row is opened), we render TWO route trees: the underlying page
  // tree resolves against the saved `background` location so the list
  // stays mounted, and a separate "modal" tree resolves against the
  // current location to mount `<CommodityDetailSheet>` on top. On
  // direct landings — refresh, share, "open in new tab" — `background`
  // is undefined and the page tree resolves against the current
  // location as today, falling through to the full-page detail.
  const location = useLocation()
  const background = (location.state as { background?: Location } | null)?.background
  return (
    <Suspense fallback={null}>
      <Routes location={background ?? location}>
        {/* Public — auth pages own their own full-screen layout (#1407). */}
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route path="/reset-password" element={<ResetPasswordPage />} />
        <Route path="/verify-email" element={<VerifyEmailPage />} />
        <Route path="/invite/:token" element={<InviteAcceptPage />} />

        {/* Maintenance is a public route — when the API returns 503 the
            http client bounces here before the auth probe even fires.
            Anything that fetches from inside this page would re-trigger
            the 503 bounce, which is why the page reads its context from
            URL params instead. #1542 / design-audit #1527. */}
        <Route path="/maintenance" element={<MaintenancePage />} />

        {/* Authenticated subtree: GroupProvider mounts once at the top so
            every protected page reads currentGroup from the same source.
            Shell (#1406) is the chrome — sidebar, top bar, palette,
            toaster, confirm — that wraps every authenticated page. Public
            routes above don't go through Shell so /login can be its own
            full-screen takeover when #1407 ships. */}
        <Route
          element={
            <ProtectedRoute>
              <GroupProvider>
                <Shell />
              </GroupProvider>
            </ProtectedRoute>
          }
        >
          {/* Group-exempt: a logged-in user with zero groups can still reach these. */}
          <Route path="/no-group" element={<NoGroupPage />} />
          <Route path="/profile" element={<ProfilePage />} />
          <Route path="/profile/edit" element={<EditProfilePage />} />
          <Route path="/profile/sessions" element={<SessionsPage />} />
          <Route path="/profile/login-history" element={<LoginHistoryPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/groups/new" element={<CreateGroupPage />} />
          <Route path="/groups/:groupId/settings" element={<GroupSettingsPage />} />
          {/* Permanent "coming soon" pages — features whose backend isn't
              implemented yet. Each links to its own per-surface tracker
              (resolved from the registry); #1417 is the umbrella aggregator
              issue, not the destination of these links. */}
          <Route path="/plans" element={<ComingSoonPage surface="plans" />} />
          <Route path="/help" element={<ComingSoonPage surface="helpCenter" />} />
          <Route path="/help/shortcuts" element={<ComingSoonPage surface="helpShortcuts" />} />
          <Route path="/whats-new" element={<ComingSoonPage surface="whatsNew" />} />
          {/* UI Showcase — dev-only design-system reference (#1542). The
              chunk only ships in dev builds; in prod the route is
              omitted entirely. */}
          {UIShowcasePage ? <Route path="/_dev/ui-showcase" element={<UIShowcasePage />} /> : null}

          {/* Legacy unscoped paths — Vue era didn't carry /g/:slug. The
              React router only mounts those resources under /g/:slug, so
              hardcoded URLs like /files or /commodities/abc/edit need a
              redirect sentinel. UngroupedRedirect bounces 0-group users to
              /no-group and group-having users to /g/<active-slug>/<path>. */}
          <Route path="/locations" element={<UngroupedRedirect />} />
          <Route path="/locations/*" element={<UngroupedRedirect />} />
          <Route path="/commodities" element={<UngroupedRedirect />} />
          <Route path="/commodities/*" element={<UngroupedRedirect />} />
          <Route path="/warranties" element={<UngroupedRedirect />} />
          <Route path="/warranties/*" element={<UngroupedRedirect />} />
          <Route path="/files" element={<UngroupedRedirect />} />
          <Route path="/files/*" element={<UngroupedRedirect />} />
          <Route path="/exports" element={<UngroupedRedirect />} />
          <Route path="/exports/*" element={<UngroupedRedirect />} />

          {/* Group-required: any path here either is /g/:slug/* itself or is
              "/" (the redirect sentinel). GroupRequiredRoute bounces 0-group
              users to /no-group. */}
          <Route
            element={
              <GroupRequiredRoute>
                <Outlet />
              </GroupRequiredRoute>
            }
          >
            <Route path="/" element={<RootRedirect />} />
            <Route path="/g/:groupSlug">
              <Route index element={<DashboardPage />} />
              <Route path="locations" element={<LocationsListPage />} />
              <Route path="locations/new" element={<LocationsListPage initialMode="create" />} />
              <Route path="locations/:id" element={<LocationDetailPage />} />
              <Route
                path="locations/:id/edit"
                element={<LocationDetailPage initialMode="edit" />}
              />
              <Route path="areas/:id" element={<AreaDetailPage />} />
              <Route path="areas/:id/edit" element={<AreaDetailPage initialMode="edit" />} />
              <Route path="commodities" element={<CommoditiesListPage />} />
              {/* /commodities/new mounts the same list page; the create
                  dialog opens by side effect when the URL matches. The
                  separate /edit route is folded into the detail page —
                  the user clicks "Edit" there to open the same dialog
                  in edit mode. */}
              <Route path="commodities/new" element={<CommoditiesListPage />} />
              <Route path="commodities/:id" element={<CommodityDetailPage />} />
              <Route path="commodities/:id/edit" element={<CommodityDetailPage />} />
              <Route path="commodities/:id/print" element={<CommodityPrintPage />} />
              <Route path="files" element={<FilesListPage />} />
              <Route path="files/:id" element={<FilesListPage />} />
              <Route path="files/:id/edit" element={<FileEditPage />} />
              <Route path="tags" element={<TagsListPage />} />
              <Route path="lent" element={<LoansListPage />} />
              <Route path="in-service" element={<ServicesListPage />} />
              <Route path="members" element={<MembersPage />} />
              <Route path="exports" element={<ExportsListPage />} />
              <Route path="exports/new" element={<ExportNewPage />} />
              <Route path="exports/import" element={<ExportImportPage />} />
              <Route path="exports/:id" element={<ExportDetailPage />} />
              <Route path="exports/:id/restore" element={<ExportRestorePage />} />
              <Route path="search" element={<SearchPage />} />
              {/* First-class warranties shipped under #1367 — dedicated
                  list view with All/Active/Expiring/Expired tabs. The
                  inline panels on commodities detail still surface
                  per-item status alongside this group-wide page. */}
              <Route path="warranties" element={<WarrantiesListPage />} />
              <Route
                path="insurance/:itemId"
                element={<ComingSoonPage surface="insuranceReport" />}
              />
              {/* /backup is a soft alias for /exports — the design mock's
                  "BackupView" landing page. Sidebar still labels the entry
                  "Backup" but the user lands on the unified Backup &
                  Exports page. */}
              <Route path="backup" element={<BackupRedirect />} />
            </Route>
          </Route>
        </Route>

        {/* Catch-all */}
        <Route path="*" element={<NotFoundPage />} />
      </Routes>

      {/* Modal overlay tree (#1546). Mounted only when the
          navigation that pushed us here carried `state.background` —
          i.e. an in-session drill-in from the items list. The print
          subroute is intentionally left out: it's a full-page-only
          surface (the user explicitly went to the print view, not a
          quick peek). The /edit subroute renders the same sheet —
          the edit dialog opens by side effect once mounted, mirroring
          the full-page behaviour. */}
      {background ? (
        <Routes>
          <Route
            element={
              <ProtectedRoute>
                <GroupProvider>
                  {/* The page tree's <Shell /> already mounts a
                      ConfirmProvider, but the modal tree is a
                      sibling that bypasses Shell — without its own
                      provider, components inside the sheet that
                      call useConfirm() (e.g. the Delete button)
                      throw "must be used within a ConfirmProvider".
                      Per-tree provider instances are fine — the
                      sheet's confirms are scoped to the sheet, the
                      page's to the page. */}
                  <ConfirmProvider>
                    <Outlet />
                  </ConfirmProvider>
                </GroupProvider>
              </ProtectedRoute>
            }
          >
            <Route
              element={
                <GroupRequiredRoute>
                  <Outlet />
                </GroupRequiredRoute>
              }
            >
              <Route path="/g/:groupSlug/commodities/:id" element={<CommodityDetailSheet />} />
              <Route path="/g/:groupSlug/commodities/:id/edit" element={<CommodityDetailSheet />} />
              <Route path="/g/:groupSlug/commodities/new" element={<CommodityCreateModalRoute />} />
            </Route>
          </Route>
        </Routes>
      ) : null}
    </Suspense>
  )
}

// /g/:slug/backup is a soft alias for /g/:slug/exports — the sidebar
// still calls the section "Backup" (matching the design mock) but the
// page itself lives at /exports. Kept as a redirect rather than deleted
// so deep links from older builds keep working.
function BackupRedirect() {
  const params = useParams()
  const slug = params.groupSlug ?? ""
  return <Navigate to={`/g/${encodeURIComponent(slug)}/exports`} replace />
}

// Re-export AuthProvider so providers.tsx wires it once at the app root.
export { AuthProvider }
