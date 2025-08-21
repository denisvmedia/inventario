import api from './api'
import axios from 'axios'

// Create a separate axios instance for auth endpoints with application/json
const authApi = axios.create({
  baseURL: '',  // Empty because we're using Vite's proxy
  headers: {
    'Content-Type': 'application/json',
    'Accept': 'application/json'
  }
})

export interface LoginRequest {
  email: string
  password: string
}

export interface LoginResponse {
  token: string
  user: {
    id: string
    email: string
    name: string
    role: string
  }
}

export interface User {
  id: string
  email: string
  name: string
  role: string
}

class AuthService {
  private readonly TOKEN_KEY = 'inventario_token'
  private readonly USER_KEY = 'inventario_user'

  /**
   * Login with email and password
   */
  async login(credentials: LoginRequest): Promise<LoginResponse> {
    // Use authApi with application/json content type for auth endpoints
    const response = await authApi.post('/api/v1/auth/login', credentials)
    const data = response.data

    // Store token and user data immediately and synchronously
    this.setToken(data.token)
    this.setUser(data.user)

    // Verify token was stored correctly
    const storedToken = this.getToken()
    console.log('Token stored successfully:', !!storedToken)
    console.log('Token length:', storedToken?.length || 0)

    return data
  }

  /**
   * Logout user
   */
  async logout(): Promise<void> {
    try {
      // Add authorization header for logout
      const token = this.getToken()
      if (token) {
        await authApi.post('/api/v1/auth/logout', {}, {
          headers: {
            'Authorization': `Bearer ${token}`
          }
        })
      }
    } catch (error) {
      console.warn('Logout API call failed:', error)
    } finally {
      // Always clear local storage
      this.clearAuth()
    }
  }

  /**
   * Get current user from API
   */
  async getCurrentUser(isBackgroundCheck = false): Promise<User> {
    // Use regular api for protected endpoints (they support vnd.api+json)
    const config = isBackgroundCheck ? {} : { headers: { 'X-Auth-Check': 'user-initiated' } }
    const response = await api.get('/api/v1/auth/me', config)
    return response.data.user
  }

  /**
   * Check if user is authenticated
   */
  isAuthenticated(): boolean {
    return !!this.getToken()
  }

  /**
   * Get stored JWT token
   */
  getToken(): string | null {
    return localStorage.getItem(this.TOKEN_KEY)
  }

  /**
   * Set JWT token in localStorage
   */
  setToken(token: string): void {
    localStorage.setItem(this.TOKEN_KEY, token)
  }

  /**
   * Get stored user data
   */
  getUser(): User | null {
    const userData = localStorage.getItem(this.USER_KEY)
    return userData ? JSON.parse(userData) : null
  }

  /**
   * Set user data in localStorage
   */
  setUser(user: User): void {
    localStorage.setItem(this.USER_KEY, JSON.stringify(user))
  }

  /**
   * Clear authentication data
   */
  clearAuth(): void {
    localStorage.removeItem(this.TOKEN_KEY)
    localStorage.removeItem(this.USER_KEY)
  }

  /**
   * Initialize authentication on app startup
   * This method is more conservative and doesn't clear auth on verification failure
   */
  async initializeAuth(): Promise<User | null> {
    const token = this.getToken()
    const storedUser = this.getUser()

    if (!token || !storedUser) {
      return null
    }

    try {
      // Verify token is still valid by getting current user (background check)
      const user = await this.getCurrentUser(true)
      this.setUser(user)
      return user
    } catch (error) {
      console.warn('Token validation failed during initialization:', error)
      // Don't clear auth here - let the API interceptor handle 401s
      // Return the stored user data to maintain session continuity
      return storedUser
    }
  }
}

export default new AuthService()
