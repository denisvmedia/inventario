import { createRouter, createWebHistory } from 'vue-router'
import HomeView from '../views/HomeView.vue'
import NotFoundView from '../views/NotFoundView.vue'
import settingsCheckService from '../services/settingsCheckService'

// Define routes without using RouteRecordRaw type
const routes = [
  {
    path: '/',
    name: 'home',
    component: HomeView
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
  // Keep area routes for backward compatibility
  {
    path: '/areas/new',
    name: 'area-create',
    component: () => import('../views/areas/AreaCreateView.vue')
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
  // Settings
  {
    path: '/settings',
    name: 'settings',
    component: () => import('../views/settings/SettingsListView.vue')
  },
  {
    path: '/settings/:id',
    name: 'setting-detail',
    component: () => import('../views/settings/SettingDetailView.vue')
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

// Add navigation guards for debugging and settings check
router.beforeEach(async (to, from) => {
  console.log(`Navigation: ${from.path} -> ${to.path}`)
  console.log('To:', to)
  console.log('From:', from)
  console.log('Matched routes:', to.matched.map(record => record.path))

  // Skip settings check for settings pages and print pages
  const isSettingsPage = to.path.startsWith('/settings')
  const isPrintPage = to.path.includes('/print')

  // If we're navigating to the settings page from another page, don't check settings
  // This prevents the banner from flashing when we already have settings
  if (isSettingsPage && from.path !== '/') {
    return true
  }

  if (!isSettingsPage && !isPrintPage) {
    // Check if settings exist
    const hasSettings = await settingsCheckService.hasSettings()

    if (!hasSettings) {
      console.log('No settings found, redirecting to settings page')
      // Add a query parameter to indicate that settings are required
      return { path: '/settings', query: { required: 'true' } }
    }
  }

  return true
})

export default router
