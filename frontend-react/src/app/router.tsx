import { Suspense, lazy } from "react"
import { Outlet, Route, Routes } from "react-router-dom"

import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ProtectedRoute } from "@/components/routing/ProtectedRoute"
import { GroupRequiredRoute } from "@/components/routing/GroupRequiredRoute"
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
              <Route
                path="locations"
                element={
                  <PlaceholderPage titleKey="locations" testId="page-locations" trackedBy="#1409" />
                }
              />
              <Route
                path="locations/new"
                element={
                  <PlaceholderPage
                    titleKey="locationNew"
                    testId="page-location-new"
                    trackedBy="#1409"
                  />
                }
              />
              <Route
                path="locations/:id"
                element={
                  <PlaceholderPage
                    titleKey="locationDetail"
                    testId="page-location-detail"
                    trackedBy="#1409"
                  />
                }
              />
              <Route
                path="locations/:id/edit"
                element={
                  <PlaceholderPage
                    titleKey="locationEdit"
                    testId="page-location-edit"
                    trackedBy="#1409"
                  />
                }
              />
              <Route
                path="areas/:id"
                element={
                  <PlaceholderPage
                    titleKey="areaDetail"
                    testId="page-area-detail"
                    trackedBy="#1409"
                  />
                }
              />
              <Route
                path="areas/:id/edit"
                element={
                  <PlaceholderPage titleKey="areaEdit" testId="page-area-edit" trackedBy="#1409" />
                }
              />
              <Route
                path="commodities"
                element={
                  <PlaceholderPage
                    titleKey="commodities"
                    testId="page-commodities"
                    trackedBy="#1410"
                  />
                }
              />
              <Route
                path="commodities/new"
                element={
                  <PlaceholderPage
                    titleKey="commodityNew"
                    testId="page-commodity-new"
                    trackedBy="#1410"
                  />
                }
              />
              <Route
                path="commodities/:id"
                element={
                  <PlaceholderPage
                    titleKey="commodityDetail"
                    testId="page-commodity-detail"
                    trackedBy="#1410"
                  />
                }
              />
              <Route
                path="commodities/:id/edit"
                element={
                  <PlaceholderPage
                    titleKey="commodityEdit"
                    testId="page-commodity-edit"
                    trackedBy="#1410"
                  />
                }
              />
              <Route
                path="commodities/:id/print"
                element={
                  <PlaceholderPage
                    titleKey="commodityPrint"
                    testId="page-commodity-print"
                    trackedBy="#1410"
                  />
                }
              />
              <Route
                path="files"
                element={<PlaceholderPage titleKey="files" testId="page-files" trackedBy="#1411" />}
              />
              <Route
                path="files/new"
                element={
                  <PlaceholderPage titleKey="fileNew" testId="page-file-new" trackedBy="#1411" />
                }
              />
              <Route
                path="files/:id"
                element={
                  <PlaceholderPage
                    titleKey="fileDetail"
                    testId="page-file-detail"
                    trackedBy="#1411"
                  />
                }
              />
              <Route
                path="files/:id/edit"
                element={
                  <PlaceholderPage titleKey="fileEdit" testId="page-file-edit" trackedBy="#1411" />
                }
              />
              <Route
                path="tags"
                element={<PlaceholderPage titleKey="tags" testId="page-tags" trackedBy="#1412" />}
              />
              <Route path="members" element={<MembersPage />} />
              <Route
                path="exports"
                element={
                  <PlaceholderPage titleKey="exports" testId="page-exports" trackedBy="#1415" />
                }
              />
              <Route
                path="exports/new"
                element={
                  <PlaceholderPage
                    titleKey="exportNew"
                    testId="page-export-new"
                    trackedBy="#1415"
                  />
                }
              />
              <Route
                path="exports/import"
                element={
                  <PlaceholderPage
                    titleKey="exportImport"
                    testId="page-export-import"
                    trackedBy="#1415"
                  />
                }
              />
              <Route
                path="exports/:id"
                element={
                  <PlaceholderPage
                    titleKey="exportDetail"
                    testId="page-export-detail"
                    trackedBy="#1415"
                  />
                }
              />
              <Route
                path="exports/:id/restore"
                element={
                  <PlaceholderPage
                    titleKey="exportRestore"
                    testId="page-export-restore"
                    trackedBy="#1415"
                  />
                }
              />
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
              {/* Coming-soon group-scoped pages (#1417). Warranties / insurance
                  ship as inline panels on items / commodities detail per the
                  design mock; the top-level routes stay as full-page stubs
                  so deep links don't 404 in the meantime. Backup stays a
                  generic PlaceholderPage — it's a real feature awaiting its
                  owning issue (not tracked under #1417). */}
              <Route path="warranties" element={<ComingSoonPage surface="warranties" />} />
              <Route
                path="insurance/:itemId"
                element={<ComingSoonPage surface="insuranceReport" />}
              />
              <Route
                path="backup"
                element={<PlaceholderPage titleKey="backup" testId="page-backup" />}
              />
            </Route>
          </Route>
        </Route>

        {/* Catch-all */}
        <Route path="*" element={<NotFoundPage />} />
      </Routes>
    </Suspense>
  )
}

// Re-export AuthProvider so providers.tsx wires it once at the app root.
export { AuthProvider }
