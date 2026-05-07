import { Suspense, lazy } from "react"
import { Navigate, Outlet, Route, Routes, useParams } from "react-router-dom"

import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ProtectedRoute } from "@/components/routing/ProtectedRoute"
import { GroupRequiredRoute } from "@/components/routing/GroupRequiredRoute"
import { UngroupedRedirect } from "@/components/routing/UngroupedRedirect"
import { PlaceholderPage } from "@/pages/Placeholder"
import { ComingSoonPage } from "@/components/coming-soon"
import { Shell } from "@/app/Shell"

// Real pages are code-split — each one becomes its own chunk so adding the
// real implementation later (in #1407–#1417) doesn't grow the entry bundle.
// Placeholder pages share a single component (PlaceholderPage) and are
// imported eagerly: there is nothing to split until the real page lands.
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

// AppRoutes is the full route tree for the new React frontend. Most of the
// pages still mount the shared PlaceholderPage stub: the routing skeleton is
// what unblocks the per-page issues (#1407–#1417), and each of those drops
// the real component in once it lands.
//
// Each placeholder route passes a `titleKey` rather than a literal title.
// The component looks the key up in the `stubs` namespace (#1405), so the
// page name is locale-aware from day one without changes to the route tree
// when a translation lands.
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
  return (
    <Suspense fallback={null}>
      <Routes>
        {/* Public — auth pages own their own full-screen layout (#1407). */}
        <Route path="/login" element={<LoginPage />} />
        <Route path="/register" element={<RegisterPage />} />
        <Route path="/forgot-password" element={<ForgotPasswordPage />} />
        <Route path="/reset-password" element={<ResetPasswordPage />} />
        <Route path="/verify-email" element={<VerifyEmailPage />} />
        <Route path="/invite/:token" element={<InviteAcceptPage />} />

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

          {/* Legacy unscoped paths — Vue era didn't carry /g/:slug. The
              React router only mounts those resources under /g/:slug, so
              hardcoded URLs like /files or /commodities/abc/edit need a
              redirect sentinel. UngroupedRedirect bounces 0-group users to
              /no-group and group-having users to /g/<active-slug>/<path>. */}
          <Route path="/locations" element={<UngroupedRedirect />} />
          <Route path="/locations/*" element={<UngroupedRedirect />} />
          <Route path="/commodities" element={<UngroupedRedirect />} />
          <Route path="/commodities/*" element={<UngroupedRedirect />} />
          <Route path="/files" element={<UngroupedRedirect />} />
          <Route path="/files/*" element={<UngroupedRedirect />} />
          <Route path="/exports" element={<UngroupedRedirect />} />
          <Route path="/exports/*" element={<UngroupedRedirect />} />
          <Route path="/system" element={<UngroupedRedirect />} />
          <Route path="/system/*" element={<UngroupedRedirect />} />

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
              <Route
                path="system"
                element={
                  <PlaceholderPage titleKey="system" testId="page-system" trackedBy="#1414" />
                }
              />
              <Route
                path="system/settings/:id"
                element={
                  <PlaceholderPage
                    titleKey="systemSetting"
                    testId="page-system-setting"
                    trackedBy="#1414"
                  />
                }
              />
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
