import api from './api'

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
    const response = await api.post('/api/v1/auth/login', credentials)
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
      await api.post('/api/v1/auth/logout')
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
  async getCurrentUser(): Promise<User> {
    const response = await api.get('/api/v1/auth/me')
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
   */
  async initializeAuth(): Promise<User | null> {
    const token = this.getToken()
    if (!token) {
      return null
    }

    try {
      // Verify token is still valid by getting current user
      const user = await this.getCurrentUser()
      this.setUser(user)
      return user
    } catch (error) {
      console.warn('Token validation failed:', error)
      this.clearAuth()
      return null
    }
  }
}

export default new AuthService()
