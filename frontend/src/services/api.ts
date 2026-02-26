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
export let navigateToLogin: (currentPath: string) => void = (currentPath: string) => {
  // Default implementation uses window.location
  window.location.href = `/login?redirect=${encodeURIComponent(currentPath)}`
}

// Set up router navigation after module loads to avoid circular dependency
if (typeof window !== 'undefined') {
  // Use dynamic import to avoid circular dependency
  import('../router').then(({ default: router }) => {
    // Update the exported function to use router
    navigateToLogin = (currentPath: string) => {
      router.push({ path: '/login', query: { redirect: currentPath } })
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

      // Skip refresh retry for auth endpoints to avoid loops
      const isAuthEndpoint = error.config?.url?.startsWith('/api/v1/auth/')
      const originalRequest = error.config

      if (!isAuthEndpoint && !originalRequest?._retry) {
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

      // Redirect to login page if not already there
      if (window.location.pathname !== '/login') {
        const currentPath = window.location.pathname + window.location.search
        navigateToLogin(currentPath)
      }
    }

    return Promise.reject(error)
  }
)

export default api
