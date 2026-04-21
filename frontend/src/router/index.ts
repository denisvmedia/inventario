import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import NotFoundView from '../views/NotFoundView.vue'
import LoginView from '../views/LoginView.vue'
import ForgotPasswordView from '../views/ForgotPasswordView.vue'
import ResetPasswordView from '../views/ResetPasswordView.vue'
import RegisterView from '../views/RegisterView.vue'
import VerifyEmailView from '../views/VerifyEmailView.vue'
import { useAuthStore } from '../stores/authStore'
import { useGroupStore } from '../stores/groupStore'

// GROUP_EXEMPT_ROUTE_NAMES lists routes that an authenticated user with zero
// groups is allowed to reach without being bounced to /no-group. Everything
// else is gated because the UI assumes a selected group (locations,
// commodities, files, exports, system…) and would otherwise render empty /
// 403 states with no guidance for the user.
// NOTE: 'login' / 'register' / etc. are also listed so a logged-in no-group
// user who explicitly types /login gets the usual "already authenticated"
// redirect to '/' instead of being clobbered into /no-group twice.
const GROUP_EXEMPT_ROUTE_NAMES = new Set([
  'login',
  'register',
  'forgot-password',
  'reset-password',
  'verify-email',
  'invite-accept',
  'no-group',
  'group-create',
  'profile',
])

// Legacy flat data paths (/locations, /commodities, …) are redirected to
// their /g/:groupSlug/... counterpart by the router guard (spec #1219 §8,
// issue #1289 Gap C) using the `legacyFlatDataRoute` meta on each stub
// route declared below. Having the slug on the URL is what makes two tabs
// with two different groups actually independent — the legacy
// localStorage-only scheme let tab 1 silently start hitting tab 2's group
// on the next navigation.

// Define routes without using RouteRecordRaw type
const routes = [
  // Public routes
  {
    path: '/login',
    name: 'login',
    component: LoginView,
    meta: { requiresAuth: false }
  },
  {
    path: '/forgot-password',
    name: 'forgot-password',
    component: ForgotPasswordView,
    meta: { requiresAuth: false }
  },
  {
    path: '/reset-password',
    name: 'reset-password',
    component: ResetPasswordView,
    meta: { requiresAuth: false }
  },
  {
    path: '/register',
    name: 'register',
    component: RegisterView,
    meta: { requiresAuth: false }
  },
  {
    path: '/verify-email',
    name: 'verify-email',
    component: VerifyEmailView,
    meta: { requiresAuth: false }
  },
  // Protected routes
  {
    path: '/',
    name: 'home',
    component: HomeView,
    meta: { requiresAuth: true }
  },
  // Group-scoped data routes: /g/:groupSlug/... — see issue #1289 Gap C.
  // Keeping the slug in the URL (not in localStorage) is what makes two
  // browser tabs with two different groups actually independent. Flat
  // equivalents are preserved below as legacy redirects.
  {
    path: '/g/:groupSlug',
    meta: { requiresAuth: true, groupScoped: true },
    children: [
      { path: 'locations',           name: 'locations',            component: () => import('../views/locations/LocationListView.vue') },
      { path: 'locations/:id',       name: 'location-detail',      component: () => import('../views/locations/LocationDetailView.vue') },
      { path: 'locations/:id/edit', name: 'location-edit',        component: () => import('../views/locations/LocationEditView.vue') },
      { path: 'areas/:id',           name: 'area-detail',          component: () => import('../views/areas/AreaDetailView.vue') },
      { path: 'areas/:id/edit',      name: 'area-edit',            component: () => import('../views/areas/AreaEditView.vue') },
      { path: 'commodities',         name: 'commodities',          component: () => import('../views/commodities/CommodityListView.vue') },
      { path: 'commodities/new',     name: 'commodity-create',     component: () => import('../views/commodities/CommodityCreateView.vue') },
      { path: 'commodities/:id',     name: 'commodity-detail',     component: () => import('../views/commodities/CommodityDetailView.vue') },
      { path: 'commodities/:id/edit',  name: 'commodity-edit',     component: () => import('../views/commodities/CommodityEditView.vue') },
      { path: 'commodities/:id/print', name: 'commodity-print',    component: () => import('../views/commodities/CommodityPrintView.vue') },
      { path: 'exports',             name: 'exports',              component: () => import('../views/exports/ExportListView.vue') },
      { path: 'exports/new',         name: 'export-create',        component: () => import('../views/exports/ExportCreateView.vue') },
      { path: 'exports/import',      name: 'export-import',        component: () => import('../views/exports/ExportImportView.vue') },
      { path: 'exports/:id',         name: 'export-detail',        component: () => import('../views/exports/ExportDetailView.vue') },
      { path: 'exports/:id/restore', name: 'export-restore',       component: () => import('../views/exports/restore/RestoreCreateView.vue') },
      { path: 'files',               name: 'files',                component: () => import('../views/files/FileListView.vue') },
      { path: 'files/create',        name: 'file-create',          component: () => import('../views/files/FileCreateView.vue') },
      { path: 'files/:id',           name: 'file-detail',          component: () => import('../views/files/FileDetailView.vue') },
      { path: 'files/:id/edit',      name: 'file-edit',            component: () => import('../views/files/FileEditView.vue') },
      { path: 'system',              name: 'system',               component: () => import('../views/system/SystemView.vue') },
      { path: 'system/settings/:id', name: 'system-setting-detail', component: () => import('../views/system/SystemSettingDetailView.vue') },
    ]
  },
  // Legacy flat routes — kept as redirect stubs so any stray
  // `router.push('/locations')` or bookmarked pre-#1289 URL still works.
  // The router-level beforeEach guard rewrites them to /g/<current-slug>/...
  // using groupStore.currentGroup.
  { path: '/locations/:pathMatch(.*)*',    meta: { legacyFlatDataRoute: '/locations' },   component: () => import('../views/locations/LocationListView.vue') },
  { path: '/areas/:pathMatch(.*)*',        meta: { legacyFlatDataRoute: '/areas' },       component: () => import('../views/areas/AreaDetailView.vue') },
  { path: '/commodities/:pathMatch(.*)*',  meta: { legacyFlatDataRoute: '/commodities' }, component: () => import('../views/commodities/CommodityListView.vue') },
  { path: '/files/:pathMatch(.*)*',        meta: { legacyFlatDataRoute: '/files' },       component: () => import('../views/files/FileListView.vue') },
  { path: '/exports/:pathMatch(.*)*',      meta: { legacyFlatDataRoute: '/exports' },     component: () => import('../views/exports/ExportListView.vue') },
  { path: '/system/:pathMatch(.*)*',       meta: { legacyFlatDataRoute: '/system' },      component: () => import('../views/system/SystemView.vue') },
  // Profile (authenticated users, not group-scoped)
  {
    path: '/profile',
    name: 'profile',
    component: () => import('../views/ProfileView.vue'),
    meta: { requiresAuth: true }
  },
  // Group management
  {
    path: '/groups/new',
    name: 'group-create',
    component: () => import('../views/groups/GroupCreateView.vue'),
    meta: { requiresAuth: true }
  },
  {
    path: '/groups/:groupId/settings',
    name: 'group-settings',
    component: () => import('../views/groups/GroupSettingsView.vue'),
    meta: { requiresAuth: true }
  },
  // No-group landing
  {
    path: '/no-group',
    name: 'no-group',
    component: () => import('../views/groups/NoGroupView.vue'),
    meta: { requiresAuth: true }
  },
  // Invite acceptance
  {
    path: '/invite/:token',
    name: 'invite-accept',
    component: () => import('../views/InviteAcceptView.vue'),
    meta: { requiresAuth: false }
  },
  // 404 - Keep this as the last route
  {
    path: '/:pathMatch(.*)*',
    name: 'not-found',
    component: NotFoundView
  }
]

// Add debugging
const router = createRouter({
  // Use createWebHistory with a base URL that works with Vite
  history: createWebHistory(import.meta.env.BASE_URL || '/'),
  routes
})

// Debug all routes
console.log('All registered routes:')
routes.forEach(route => {
  console.log(`- ${route.path} (${route.name})`)
})

// Add navigation guards for authentication, debugging and settings check
router.beforeEach(async (to, from) => {
  console.log(`Navigation: ${from.path} -> ${to.path}`)
  console.log('To:', to)
  console.log('From:', from)
  console.log('Matched routes:', to.matched.map(record => record.path))

  // Initialize auth store
  const authStore = useAuthStore()

  console.log('Router guard - Auth state check:', {
    isAuthenticated: authStore.isAuthenticated,
    isInitialized: authStore.isInitialized,
    isLoading: authStore.isLoading,
    user: authStore.user?.email || 'none'
  })

  // Wait for authentication initialization to complete if it's in progress
  if (authStore.isLoading) {
    console.log('Waiting for auth initialization to complete...')
    let attempts = 0
    while (authStore.isLoading && attempts < 100) { // Max 5 seconds
      await new Promise(resolve => setTimeout(resolve, 50))
      attempts++
    }
    console.log('Auth loading wait complete after', attempts * 50, 'ms')
  }

  // Only initialize if not already initialized
  if (!authStore.isInitialized) {
    console.log('Initializing authentication from router guard...')
    await authStore.initializeAuth()
    console.log('Router guard auth initialization complete')
  }

  // Check if route requires authentication (default to true unless explicitly false)
  const requiresAuth = to.meta.requiresAuth !== false

  // If route requires auth and user is not authenticated, redirect to login
  if (requiresAuth && !authStore.isAuthenticated) {
    console.log('Authentication required, redirecting to login')
    return { path: '/login', query: { redirect: to.fullPath } }
  }

  // If user is authenticated and trying to access login page, redirect to home
  if (to.path === '/login' && authStore.isAuthenticated) {
    console.log('Already authenticated, redirecting to home')
    return { path: '/' }
  }

  // Group-membership gate (#1261): an authenticated user with zero groups
  // has nothing to see on /commodities, /locations, /files, etc. — those
  // views assume a selected group and hit /api/v1/g/{slug}/... endpoints
  // that 404 without one. Bounce them to /no-group (the guided first-run
  // view) unless the destination is one of the onboarding-friendly routes.
  if (authStore.isAuthenticated) {
    const routeName = typeof to.name === 'string' ? to.name : ''
    if (!GROUP_EXEMPT_ROUTE_NAMES.has(routeName)) {
      const groupStore = useGroupStore()
      try {
        await groupStore.ensureLoaded()
      } catch (err) {
        // If the groups endpoint errors we don't know the count — let
        // navigation through so the user sees the page's own error handling
        // rather than being stuck in a redirect loop.
        console.warn('Router guard: ensureLoaded failed, allowing navigation', err)
        return true
      }
      if (!groupStore.hasGroups) {
        console.log('No groups — redirecting to /no-group')
        return { path: '/no-group' }
      }

      // Legacy flat data-route handling (issue #1289 Gap C): rewrite
      // /locations, /commodities, etc. to /g/<current-slug>/... so the
      // active group is explicit in the URL and independent per-tab.
      const legacyPrefix = (to.meta as { legacyFlatDataRoute?: string })?.legacyFlatDataRoute
      if (legacyPrefix && groupStore.currentGroup) {
        const slug = groupStore.currentGroup.slug
        // Preserve any subpath after the matched prefix (e.g. /commodities/:id/edit).
        const suffix = to.path.startsWith(legacyPrefix)
          ? to.path.slice(legacyPrefix.length)
          : ''
        const targetPath = `/g/${encodeURIComponent(slug)}${legacyPrefix}${suffix}`
        return { path: targetPath, query: to.query, hash: to.hash, replace: true }
      }

      // When navigating into a /g/:groupSlug/... route, sync the store to
      // whatever slug is on the URL. Two tabs can now hold two different
      // currentGroup values because each tab derives its state from its
      // own URL rather than a shared localStorage key.
      const slugParam = typeof to.params.groupSlug === 'string' ? to.params.groupSlug : ''
      if (slugParam) {
        const match = groupStore.groups.find((g) => g.slug === slugParam)
        if (!match) {
          // Unknown slug for this user (wrong group, revoked membership,
          // or stale URL). Redirect to the user's current group so they
          // land somewhere safe instead of on a 404.
          if (groupStore.currentGroup) {
            return { path: `/g/${encodeURIComponent(groupStore.currentGroup.slug)}/` }
          }
          return { path: '/no-group' }
        }
        if (groupStore.currentGroup?.slug !== slugParam) {
          // Route-driven switch: update the store but don't write to
          // localStorage — per-tab isolation is the whole point.
          await groupStore.setCurrentGroup(slugParam)
        }
      }
    }
  }

  // The former "check that admin's system.main_currency is set, otherwise
  // redirect to /system?required=true" guard is gone — main_currency moved
  // to the location group in #1248 and the schema's NOT NULL DEFAULT 'USD'
  // means every group the user can reach already has a valid currency.
  // Keeping the check (now reading a field the backend no longer emits)
  // would redirect every navigation to /system and hang the app.
  return true
})

export default router
