import { Suspense, lazy } from "react"
import { Outlet, Route, Routes } from "react-router-dom"

import { AuthProvider } from "@/features/auth/AuthContext"
import { GroupProvider } from "@/features/group/GroupContext"
import { ProtectedRoute } from "@/components/routing/ProtectedRoute"
import { GroupRequiredRoute } from "@/components/routing/GroupRequiredRoute"
import { PlaceholderPage } from "@/pages/Placeholder"

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

// AppRoutes is the full route tree for the new React frontend. Most of the
// pages still mount the shared PlaceholderPage stub: the routing skeleton is
// what unblocks the per-page issues (#1407–#1417), and each of those drops
// the real component in once it lands.
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
        {/* Public */}
        <Route
          path="/login"
          element={<PlaceholderPage title="Sign in" testId="page-login" trackedBy="#1407" />}
        />
        <Route
          path="/register"
          element={
            <PlaceholderPage title="Create account" testId="page-register" trackedBy="#1407" />
          }
        />
        <Route
          path="/forgot-password"
          element={
            <PlaceholderPage
              title="Forgot password"
              testId="page-forgot-password"
              trackedBy="#1407"
            />
          }
        />
        <Route
          path="/reset-password"
          element={
            <PlaceholderPage
              title="Reset password"
              testId="page-reset-password"
              trackedBy="#1407"
            />
          }
        />
        <Route
          path="/verify-email"
          element={
            <PlaceholderPage title="Verify email" testId="page-verify-email" trackedBy="#1407" />
          }
        />
        <Route
          path="/invite/:token"
          element={
            <PlaceholderPage title="Accept invite" testId="page-invite-accept" trackedBy="#1407" />
          }
        />

        {/* Authenticated subtree: GroupProvider mounts once at the top so
            every protected page reads currentGroup from the same source. */}
        <Route
          element={
            <ProtectedRoute>
              <GroupProvider>
                <Outlet />
              </GroupProvider>
            </ProtectedRoute>
          }
        >
          {/* Group-exempt: a logged-in user with zero groups can still reach these. */}
          <Route
            path="/no-group"
            element={<PlaceholderPage title="No group" testId="page-no-group" trackedBy="#1413" />}
          />
          <Route
            path="/profile"
            element={<PlaceholderPage title="Profile" testId="page-profile" trackedBy="#1414" />}
          />
          <Route
            path="/groups/new"
            element={
              <PlaceholderPage title="Create group" testId="page-group-create" trackedBy="#1413" />
            }
          />
          <Route
            path="/groups/:groupId/settings"
            element={
              <PlaceholderPage
                title="Group settings"
                testId="page-group-settings"
                trackedBy="#1413"
              />
            }
          />
          <Route
            path="/plans"
            element={<PlaceholderPage title="Plans" testId="page-plans" trackedBy="#1417" />}
          />
          <Route
            path="/help/shortcuts"
            element={
              <PlaceholderPage
                title="Keyboard shortcuts"
                testId="page-help-shortcuts"
                trackedBy="#1417"
              />
            }
          />
          <Route
            path="/whats-new"
            element={
              <PlaceholderPage title="What's new" testId="page-whats-new" trackedBy="#1417" />
            }
          />

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
                  <PlaceholderPage title="Locations" testId="page-locations" trackedBy="#1409" />
                }
              />
              <Route
                path="locations/new"
                element={
                  <PlaceholderPage
                    title="New location"
                    testId="page-location-new"
                    trackedBy="#1409"
                  />
                }
              />
              <Route
                path="locations/:id"
                element={
                  <PlaceholderPage
                    title="Location"
                    testId="page-location-detail"
                    trackedBy="#1409"
                  />
                }
              />
              <Route
                path="locations/:id/edit"
                element={
                  <PlaceholderPage
                    title="Edit location"
                    testId="page-location-edit"
                    trackedBy="#1409"
                  />
                }
              />
              <Route
                path="areas/:id"
                element={
                  <PlaceholderPage title="Area" testId="page-area-detail" trackedBy="#1409" />
                }
              />
              <Route
                path="areas/:id/edit"
                element={
                  <PlaceholderPage title="Edit area" testId="page-area-edit" trackedBy="#1409" />
                }
              />
              <Route
                path="commodities"
                element={
                  <PlaceholderPage title="Items" testId="page-commodities" trackedBy="#1410" />
                }
              />
              <Route
                path="commodities/new"
                element={
                  <PlaceholderPage title="New item" testId="page-commodity-new" trackedBy="#1410" />
                }
              />
              <Route
                path="commodities/:id"
                element={
                  <PlaceholderPage title="Item" testId="page-commodity-detail" trackedBy="#1410" />
                }
              />
              <Route
                path="commodities/:id/edit"
                element={
                  <PlaceholderPage
                    title="Edit item"
                    testId="page-commodity-edit"
                    trackedBy="#1410"
                  />
                }
              />
              <Route
                path="commodities/:id/print"
                element={
                  <PlaceholderPage
                    title="Print item"
                    testId="page-commodity-print"
                    trackedBy="#1410"
                  />
                }
              />
              <Route
                path="files"
                element={<PlaceholderPage title="Files" testId="page-files" trackedBy="#1411" />}
              />
              <Route
                path="files/new"
                element={
                  <PlaceholderPage title="Upload files" testId="page-file-new" trackedBy="#1411" />
                }
              />
              <Route
                path="files/:id"
                element={
                  <PlaceholderPage title="File" testId="page-file-detail" trackedBy="#1411" />
                }
              />
              <Route
                path="files/:id/edit"
                element={
                  <PlaceholderPage title="Edit file" testId="page-file-edit" trackedBy="#1411" />
                }
              />
              <Route
                path="tags"
                element={<PlaceholderPage title="Tags" testId="page-tags" trackedBy="#1412" />}
              />
              <Route
                path="members"
                element={
                  <PlaceholderPage title="Members" testId="page-members" trackedBy="#1413" />
                }
              />
              <Route
                path="exports"
                element={
                  <PlaceholderPage title="Exports" testId="page-exports" trackedBy="#1415" />
                }
              />
              <Route
                path="exports/new"
                element={
                  <PlaceholderPage title="New export" testId="page-export-new" trackedBy="#1415" />
                }
              />
              <Route
                path="exports/import"
                element={
                  <PlaceholderPage title="Import" testId="page-export-import" trackedBy="#1415" />
                }
              />
              <Route
                path="exports/:id"
                element={
                  <PlaceholderPage title="Export" testId="page-export-detail" trackedBy="#1415" />
                }
              />
              <Route
                path="exports/:id/restore"
                element={
                  <PlaceholderPage title="Restore" testId="page-export-restore" trackedBy="#1415" />
                }
              />
              <Route
                path="search"
                element={<PlaceholderPage title="Search" testId="page-search" trackedBy="#1416" />}
              />
              <Route
                path="system"
                element={<PlaceholderPage title="System" testId="page-system" trackedBy="#1414" />}
              />
              <Route
                path="system/settings/:id"
                element={
                  <PlaceholderPage
                    title="System setting"
                    testId="page-system-setting"
                    trackedBy="#1414"
                  />
                }
              />
              <Route
                path="warranties"
                element={
                  <PlaceholderPage title="Warranties" testId="page-warranties" trackedBy="#1417" />
                }
              />
              <Route
                path="insurance/:itemId"
                element={
                  <PlaceholderPage title="Insurance" testId="page-insurance" trackedBy="#1417" />
                }
              />
              <Route
                path="backup"
                element={<PlaceholderPage title="Backup" testId="page-backup" trackedBy="#1417" />}
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
