import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import authService, { type User, type LoginRequest } from '../services/authService'

export const useAuthStore = defineStore('auth', () => {
  // State
  const user = ref<User | null>(null)
  const isLoading = ref(false)
  const error = ref<string | null>(null)

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
    isLoading.value = true
    error.value = null

    try {
      const userData = await authService.initializeAuth()
      user.value = userData
    } catch (err: any) {
      console.warn('Auth initialization error:', err)
      user.value = null
    } finally {
      isLoading.value = false
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
