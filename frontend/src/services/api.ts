import axios from 'axios'

// Extend AxiosRequestConfig to support retry tracking without mutating the
// typed config object via a plain property assignment.
declare module 'axios' {
  interface InternalAxiosRequestConfig {
    _retry?: boolean
  }
}

// Navigation function that can be mocked in tests
// eslint-disable-next-line no-unused-vars
export let navigateToLogin: (currentPath: string, reason?: string) => void = (currentPath: string, reason?: string) => {
  // Default implementation uses window.location
  const params = new URLSearchParams({ redirect: currentPath })
  if (reason) params.set('reason', reason)
  window.location.href = `/login?${params.toString()}`
}

// Set up router navigation after module loads to avoid circular dependency
if (typeof window !== 'undefined') {
  // Use dynamic import to avoid circular dependency
  import('../router').then(({ default: router }) => {
    // Update the exported function to use router
    navigateToLogin = (currentPath: string, reason?: string) => {
      const query: Record<string, string> = { redirect: currentPath }
      if (reason) query.reason = reason
      router.push({ path: '/login', query })
    }
  }).catch((error) => {
    console.warn('Router import failed, using window.location fallback:', error)
  })
}

const api = axios.create({
  baseURL: '',  // Empty because we're using Vite's proxy
  headers: {
    'Content-Type': 'application/vnd.api+json',
    'Accept': 'application/vnd.api+json'
  }
})

// -----------------------------------------------------------------------
// CSRF token management
// The token is kept in both memory (for fast access) and sessionStorage
// (so it survives a page reload while being automatically cleared when
// the tab or browser window is closed).
// -----------------------------------------------------------------------
const CSRF_SESSION_KEY = 'inventario_csrf_token'
let csrfToken: string | null = null

export function setCsrfToken(token: string): void {
  csrfToken = token
  sessionStorage.setItem(CSRF_SESSION_KEY, token)
}

export function getCsrfToken(): string | null {
  if (!csrfToken) {
    csrfToken = sessionStorage.getItem(CSRF_SESSION_KEY)
  }
  return csrfToken
}

export function clearCsrfToken(): void {
  csrfToken = null
  sessionStorage.removeItem(CSRF_SESSION_KEY)
}

// Function to get token from localStorage
function getAuthToken(): string | null {
  return localStorage.getItem('inventario_token')
}

// State-changing methods that require a CSRF token.
const mutatingMethods = new Set(['post', 'put', 'patch', 'delete'])

// Add request interceptor for authentication and debugging
api.interceptors.request.use(
  config => {
    // Add JWT token to requests if available
    const token = getAuthToken()
    console.log('🔑 Token check for', config.url, ':', !!token, token ? `(${token.length} chars)` : '(no token)')

    if (token) {
      config.headers.Authorization = `Bearer ${token}`
      console.log('✅ Authorization header added')
    } else {
      console.log('❌ No token available for request')
    }

    // Rewrite data API URLs to include the group slug when a group is active.
    // This transparently routes requests through /api/v1/g/{slug}/... without
    // requiring changes to individual service files.
    //
    // `encodeURIComponent` is intentional: today slugs are base64url (safe
    // for URLs without encoding), but the slug is routed through user storage
    // and a schema change could introduce reserved characters. Encoding here
    // is cheap insurance against that class of bug — it's also what the rest
    // of the codebase that builds `/api/v1/g/{slug}/...` URLs does (e.g. the
    // raw `fetch()` in ExportImportView).
    if (config.url) {
      const groupSlug = localStorage.getItem('currentGroupSlug')
      if (groupSlug) {
        const groupScopedPrefixes = [
          '/api/v1/locations',
          '/api/v1/areas',
          '/api/v1/commodities',
          '/api/v1/files',
          '/api/v1/exports',
          '/api/v1/upload-slots',
          '/api/v1/uploads',
          '/api/v1/settings',
          '/api/v1/search',
        ]
        for (const prefix of groupScopedPrefixes) {
          if (config.url.startsWith(prefix)) {
            const suffix = config.url.slice('/api/v1'.length)
            config.url = `/api/v1/g/${encodeURIComponent(groupSlug)}${suffix}`
            break
          }
        }
      }
    }

    // Add CSRF token to state-changing requests (also checks sessionStorage after a reload).
    const currentCsrfToken = getCsrfToken()
    if (config.method && mutatingMethods.has(config.method.toLowerCase()) && currentCsrfToken) {
      config.headers['X-CSRF-Token'] = currentCsrfToken
    }

    console.log('API Request URL:', config.url)
    console.log('API Request Method:', config.method?.toUpperCase())
    console.log('API Request Headers:', JSON.stringify(config.headers, null, 2))
    console.log('API Request Data:', JSON.stringify(config.data, null, 2))
    return config
  },
  error => {
    console.error('API Request Error:', error)
    return Promise.reject(error)
  }
)

// Track whether a token refresh is already in progress to avoid loops
let isRefreshing = false
let refreshSubscribers: Array<(_token: string) => void> = []
let refreshSubscriberRejects: Array<(_reason: unknown) => void> = []

function onRefreshed(token: string) {
  refreshSubscribers.forEach(cb => cb(token))
  refreshSubscribers = []
  refreshSubscriberRejects = []
}

function onRefreshFailed(error: unknown) {
  refreshSubscriberRejects.forEach(cb => cb(error))
  refreshSubscribers = []
  refreshSubscriberRejects = []
}

// Add response interceptor for authentication and debugging
api.interceptors.response.use(
  response => {
    console.log('API Response Status:', response.status)
    console.log('API Response Headers:', JSON.stringify(response.headers, null, 2))
    console.log('API Response Data:', JSON.stringify(response.data, null, 2))
    return response
  },
  async error => {
    console.error('API Response Error Status:', error.response?.status)
    console.error('API Response Error Data:', JSON.stringify(error.response?.data, null, 2))

    // Handle 401 Unauthorized errors
    if (error.response?.status === 401) {
      console.warn('401 Unauthorized - checking if this is during initialization')

      // Don't clear auth if this is a background verification during initialization
      // Only clear auth for user-initiated requests
      const isInitializationRequest = error.config?.url?.includes('/auth/me') &&
                                     error.config?.headers?.['X-Auth-Check'] !== 'user-initiated'

      if (isInitializationRequest) {
        console.warn('401 during background auth verification - not clearing stored auth')
        return Promise.reject(error)
      }

      // Skip refresh retry for specific auth endpoints to avoid loops.
      // For these endpoints, a 401 is treated as an application-level error
      // (e.g. invalid login credentials or invalid/expired refresh token),
      // not a session-expiry event, so we must NOT clear auth or redirect.
      // All other auth endpoints (e.g. /auth/change-password, /auth/me) are
      // intentionally excluded so their 401s can follow the normal
      // refresh/redirect flow when the access token is simply expired.
      const url = error.config?.url ?? ''
      const nonRefreshableAuthEndpoints = [
        '/api/v1/auth/login',
        '/api/v1/auth/register',
        '/api/v1/auth/refresh',
      ]
      const isNonRefreshableAuthEndpoint = nonRefreshableAuthEndpoints.includes(url)
      if (isNonRefreshableAuthEndpoint) {
        return Promise.reject(error)
      }

      const originalRequest = error.config

      if (!originalRequest?._retry) {
        if (isRefreshing) {
          // Queue this request to retry after refresh completes.
          // Both resolve and reject are tracked so that queued promises are
          // always settled when the refresh either succeeds or fails.
          return new Promise((resolve, reject) => {
            refreshSubscribers.push((token: string) => {
              if (originalRequest.headers) {
                originalRequest.headers.Authorization = `Bearer ${token}`
              }
              // Mark as retried so a subsequent 401 does not trigger another refresh.
              originalRequest._retry = true
              resolve(api(originalRequest))
            })
            refreshSubscriberRejects.push(reject)
          })
        }

        originalRequest._retry = true
        isRefreshing = true

        try {
          // Attempt token refresh using httpOnly cookie
          const refreshResponse = await api.post('/api/v1/auth/refresh', {}, {
            headers: { 'Content-Type': 'application/json', 'Accept': 'application/json' }
          })
          const newToken = refreshResponse.data?.access_token
          if (newToken) {
            localStorage.setItem('inventario_token', newToken)
            api.defaults.headers.common['Authorization'] = `Bearer ${newToken}`
            if (originalRequest.headers) {
              originalRequest.headers.Authorization = `Bearer ${newToken}`
            }
            // Update the CSRF token from the refresh response (persisted to sessionStorage).
            const newCsrfToken = refreshResponse.data?.csrf_token
            if (newCsrfToken) {
              setCsrfToken(newCsrfToken)
            }
            onRefreshed(newToken)
            return api(originalRequest)
          }
          // No token returned — treat as failed refresh
          onRefreshFailed(new Error('Refresh returned no token'))
        } catch (refreshError) {
          console.warn('Token refresh failed:', refreshError)
          onRefreshFailed(refreshError)
        } finally {
          isRefreshing = false
        }
      }

      console.warn('401 on user request - clearing auth and redirecting to login')

      // Clear stored auth data
      localStorage.removeItem('inventario_token')
      localStorage.removeItem('inventario_user')

      // Clear auth store state if available
      try {
        import('@/stores/authStore').then(({ useAuthStore }) => {
          const authStore = useAuthStore()
          authStore.user = null
          authStore.isInitialized = false
        }).catch(e => {
          console.warn('Could not clear auth store:', e)
        })
      } catch (e) {
        console.warn('Could not import auth store:', e)
      }

      // Redirect to login page if not already on a public page
      const publicPaths = ['/login', '/register', '/verify-email']
      if (!publicPaths.some(p => window.location.pathname.startsWith(p))) {
        const currentPath = window.location.pathname + window.location.search
        navigateToLogin(currentPath, 'session_expired')
      }
    }

    return Promise.reject(error)
  }
)

export default api
