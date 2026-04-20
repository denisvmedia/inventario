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
  // Locations (now includes areas)
  {
    path: '/locations',
    name: 'locations',
    component: () => import('../views/locations/LocationListView.vue')
  },
  {
    path: '/locations/:id',
    name: 'location-detail',
    component: () => import('../views/locations/LocationDetailView.vue')
  },
  {
    path: '/locations/:id/edit',
    name: 'location-edit',
    component: () => import('../views/locations/LocationEditView.vue')
  },
  {
    path: '/areas/:id',
    name: 'area-detail',
    component: () => import('../views/areas/AreaDetailView.vue')
  },
  {
    path: '/areas/:id/edit',
    name: 'area-edit',
    component: () => import('../views/areas/AreaEditView.vue')
  },
  // Commodities
  {
    path: '/commodities',
    name: 'commodities',
    component: () => import('../views/commodities/CommodityListView.vue')
  },
  {
    path: '/commodities/new',
    name: 'commodity-create',
    component: () => import('../views/commodities/CommodityCreateView.vue')
  },
  {
    path: '/commodities/:id',
    name: 'commodity-detail',
    component: () => import('../views/commodities/CommodityDetailView.vue')
  },
  {
    path: '/commodities/:id/edit',
    name: 'commodity-edit',
    component: () => import('../views/commodities/CommodityEditView.vue')
  },
  {
    path: '/commodities/:id/print',
    name: 'commodity-print',
    component: () => import('../views/commodities/CommodityPrintView.vue')
  },
  // Exports
  {
    path: '/exports',
    name: 'exports',
    component: () => import('../views/exports/ExportListView.vue')
  },
  {
    path: '/exports/new',
    name: 'export-create',
    component: () => import('../views/exports/ExportCreateView.vue')
  },
  {
    path: '/exports/import',
    name: 'export-import',
    component: () => import('../views/exports/ExportImportView.vue')
  },
  {
    path: '/exports/:id',
    name: 'export-detail',
    component: () => import('../views/exports/ExportDetailView.vue')
  },
  {
    path: '/exports/:id/restore',
    name: 'export-restore',
    component: () => import('../views/exports/restore/RestoreCreateView.vue')
  },
  // Files
  {
    path: '/files',
    name: 'files',
    component: () => import('../views/files/FileListView.vue')
  },
  {
    path: '/files/create',
    name: 'file-create',
    component: () => import('../views/files/FileCreateView.vue')
  },
  {
    path: '/files/:id',
    name: 'file-detail',
    component: () => import('../views/files/FileDetailView.vue')
  },
  {
    path: '/files/:id/edit',
    name: 'file-edit',
    component: () => import('../views/files/FileEditView.vue')
  },
  // Profile (authenticated users)
  {
    path: '/profile',
    name: 'profile',
    component: () => import('../views/ProfileView.vue'),
    meta: { requiresAuth: true }
  },
  // System (formerly Settings)
  {
    path: '/system',
    name: 'system',
    component: () => import('../views/system/SystemView.vue')
  },
  {
    path: '/system/settings/:id',
    name: 'system-setting-detail',
    component: () => import('../views/system/SystemSettingDetailView.vue')
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
