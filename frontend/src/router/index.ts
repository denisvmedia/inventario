import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import NotFoundView from '../views/NotFoundView.vue'
import LoginView from '../views/LoginView.vue'
import ForgotPasswordView from '../views/ForgotPasswordView.vue'
import ResetPasswordView from '../views/ResetPasswordView.vue'
import RegisterView from '../views/RegisterView.vue'
import VerifyEmailView from '../views/VerifyEmailView.vue'
import settingsCheckService from '../services/settingsCheckService'
import { useAuthStore } from '../stores/authStore'

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
  // Admin
  {
    path: '/admin/users',
    name: 'admin-users',
    component: () => import('../views/admin/UserListView.vue'),
    meta: { requiresAuth: true, requiresAdmin: true }
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

  // If route requires admin and user is not an admin, redirect to home
  if (to.meta.requiresAdmin && authStore.userRole !== 'admin') {
    console.log('Admin access required, redirecting to home')
    return { path: '/' }
  }

  // If user is authenticated and trying to access login page, redirect to home
  if (to.path === '/login' && authStore.isAuthenticated) {
    console.log('Already authenticated, redirecting to home')
    return { path: '/' }
  }

  // Skip settings check for login, register, verify-email, system pages and print pages
  const isLoginPage = to.path === '/login'
  const isRegisterPage = to.path === '/register' || to.path === '/verify-email'
  const isSystemPage = to.path.startsWith('/system')
  const isPrintPage = to.path.includes('/print')

  // If we're navigating to the system page from another page, don't check settings
  // This prevents the banner from flashing when we already have settings
  if (isSystemPage && from.path !== '/') {
    return true
  }

  if (!isLoginPage && !isRegisterPage && !isSystemPage && !isPrintPage) {
    // Check if settings exist
    const hasSettings = await settingsCheckService.hasSettings()

    if (!hasSettings) {
      console.log('No settings found, redirecting to system page')
      // Add a query parameter to indicate that settings are required
      return { path: '/system', query: { required: 'true' } }
    }
  }

  return true
})

export default router
