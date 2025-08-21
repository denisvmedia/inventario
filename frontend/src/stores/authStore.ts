import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import authService, { type User, type LoginRequest } from '../services/authService'

export const useAuthStore = defineStore('auth', () => {
  // State
  const user = ref<User | null>(null)
  const isLoading = ref(false)
  const error = ref<string | null>(null)
  const isInitialized = ref(false)

  // Getters
  const isAuthenticated = computed(() => !!user.value)
  const userRole = computed(() => user.value?.role || null)
  const userName = computed(() => user.value?.name || null)
  const userEmail = computed(() => user.value?.email || null)

  // Actions
  async function login(credentials: LoginRequest): Promise<void> {
    isLoading.value = true
    error.value = null

    try {
      const response = await authService.login(credentials)
      user.value = response.user
    } catch (err: any) {
      error.value = err.response?.data?.message || 'Login failed'
      throw err
    } finally {
      isLoading.value = false
    }
  }

  async function logout(): Promise<void> {
    isLoading.value = true
    error.value = null

    try {
      await authService.logout()
    } catch (err: any) {
      console.warn('Logout error:', err)
    } finally {
      user.value = null
      isLoading.value = false
    }
  }

  async function initializeAuth(): Promise<void> {
    // Prevent multiple simultaneous initializations
    if (isLoading.value || isInitialized.value) {
      console.log('Auth initialization already in progress or completed')
      return
    }

    isLoading.value = true
    error.value = null

    try {
      // First, restore user data from localStorage immediately (synchronous)
      const storedUser = authService.getUser()
      const storedToken = authService.getToken()

      if (storedToken && storedUser) {
        // Restore user state immediately
        user.value = storedUser
        console.log('Auth state restored from localStorage:', storedUser.email)

        // Verify token in background without clearing auth on failure
        try {
          const freshUserData = await authService.getCurrentUser(true) // Mark as background check
          user.value = freshUserData
          authService.setUser(freshUserData)
          console.log('Token verified and user data refreshed')
        } catch (verifyError) {
          console.warn('Token verification failed, keeping stored auth:', verifyError)
          // Keep the stored auth data - API interceptor will handle 401s
          // This prevents session loss due to temporary network issues
        }
      } else {
        console.log('No stored authentication found')
        user.value = null
      }
    } catch (err: any) {
      console.warn('Auth initialization error:', err)
      user.value = null
    } finally {
      isLoading.value = false
      isInitialized.value = true
    }
  }

  async function refreshUser(): Promise<void> {
    if (!authService.isAuthenticated()) {
      user.value = null
      return
    }

    try {
      const userData = await authService.getCurrentUser()
      user.value = userData
    } catch (err: any) {
      console.warn('User refresh error:', err)
      user.value = null
      authService.clearAuth()
    }
  }

  function clearError(): void {
    error.value = null
  }

  return {
    // State
    user,
    isLoading,
    error,
    isInitialized,

    // Getters
    isAuthenticated,
    userRole,
    userName,
    userEmail,

    // Actions
    login,
    logout,
    initializeAuth,
    refreshUser,
    clearError
  }
})
