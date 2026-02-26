import api, { setCsrfToken, clearCsrfToken } from './api'

export interface LoginRequest {
  email: string
  password: string
}

export interface RegisterRequest {
  email: string
  password: string
  name: string
}

export interface RegisterResponse {
  message: string
}

export interface VerifyEmailResponse {
  message: string
}

export interface LoginResponse {
  access_token: string
  token_type: string
  expires_in: number
  csrf_token: string
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
    // Use main api instance with application/json content type for auth endpoints
    const response = await api.post('/api/v1/auth/login', credentials, {
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json'
      }
    })
    const data = response.data

    // Store token and user data immediately and synchronously
    this.setToken(data.access_token)
    this.setUser(data.user)

    // Store the CSRF token for subsequent state-changing requests.
    if (data.csrf_token) {
      setCsrfToken(data.csrf_token)
    }

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
      // Use main api instance with application/json content type for auth endpoints
      const token = this.getToken()
      if (token) {
        await api.post('/api/v1/auth/logout', {}, {
          headers: {
            'Content-Type': 'application/json',
            'Accept': 'application/json',
            'Authorization': `Bearer ${token}`
          }
        })
      }
    } catch (error) {
      console.warn('Logout API call failed:', error)
    } finally {
      // Always clear local storage and the in-memory CSRF token.
      this.clearAuth()
      clearCsrfToken()
    }
  }

  /**
   * Get current user from API
   */
  async getCurrentUser(isBackgroundCheck = false): Promise<User> {
    // Use regular api for protected endpoints (they support vnd.api+json)
    const config = isBackgroundCheck ? {} : { headers: { 'X-Auth-Check': 'user-initiated' } }
    const response = await api.get('/api/v1/auth/me', config)

    // Recover the CSRF token from the response header (populated by the server
    // to allow the frontend to restore it after a page reload).
    const responseCsrfToken = response.headers['x-csrf-token']
    if (responseCsrfToken) {
      setCsrfToken(responseCsrfToken)
    }

    // The API returns user data directly, not wrapped in a .user property
    const userData = response.data
    console.log('getCurrentUser - Raw API response:', userData)

    // Map the API response to our User interface
    const user: User = {
      id: userData.id,
      email: userData.email,
      name: userData.name,
      role: userData.role
    }

    console.log('getCurrentUser - Mapped user data:', user)
    return user
  }

  /**
   * Register a new user account.
   * The server always returns success to prevent user enumeration.
   */
  async register(req: RegisterRequest): Promise<RegisterResponse> {
    const response = await api.post('/api/v1/register', req, {
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json'
      }
    })
    return response.data
  }

  /**
   * Verify an email address using the token from the verification link.
   */
  async verifyEmail(token: string): Promise<VerifyEmailResponse> {
    const response = await api.get('/api/v1/verify-email', {
      params: { token },
      headers: { 'Accept': 'application/json' }
    })
    return response.data
  }

  /**
   * Request a new verification email for an unverified account.
   */
  async resendVerification(email: string): Promise<RegisterResponse> {
    const response = await api.post('/api/v1/resend-verification', { email }, {
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json'
      }
    })
    return response.data
  }

  /**
   * Refresh the access token using the httpOnly refresh token cookie.
   * Returns the new access token on success, or null on failure.
   */
  async refreshAccessToken(): Promise<string | null> {
    try {
      const response = await api.post('/api/v1/auth/refresh', {}, {
        headers: {
          'Content-Type': 'application/json',
          'Accept': 'application/json'
        }
      })
      const data = response.data
      if (data.access_token) {
        this.setToken(data.access_token)
        return data.access_token
      }
      return null
    } catch {
      return null
    }
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
