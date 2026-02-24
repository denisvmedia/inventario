import { Page } from '@playwright/test';
import { TestRecorder, log } from '../../utils/test-recorder.js';

/**
 * CSRF token storage for E2E tests
 * This stores the CSRF token extracted during login
 */
let csrfToken: string | null = null;

/**
 * Set the CSRF token
 */
export function setCsrfToken(token: string | null): void {
  csrfToken = token;
  if (token) {
    log(undefined, `🔑 CSRF token stored: ${token.substring(0, 10)}...`);
  }
}

/**
 * Get the current CSRF token
 */
export function getCsrfToken(): string | null {
  return csrfToken;
}

/**
 * Clear the CSRF token
 */
export function clearCsrfToken(): void {
  csrfToken = null;
}

/**
 * Extract CSRF token from login response
 * This should be called after a successful login
 */
export async function extractCsrfTokenFromResponse(page: Page, recorder?: TestRecorder): Promise<string | null> {
  let token: string | null = null;
  
  // Set up a one-time listener for the login response
  const responseHandler = async (response: any) => {
    if (response.url().includes('/api/v1/auth/login') && response.status() === 200) {
      try {
        const data = await response.json();
        if (data.csrf_token) {
          token = data.csrf_token;
          setCsrfToken(token);
          log(recorder, `🔑 CSRF token extracted: ${token.substring(0, 10)}...`);
        }
      } catch (err) {
        // Ignore JSON parse errors
      }
    }
  };
  
  page.on('response', responseHandler);
  
  // Wait a moment for the response to be captured
  await page.waitForTimeout(500);
  
  // Remove the listener
  page.off('response', responseHandler);
  
  return token;
}

/**
 * Get CSRF token from page's localStorage or session
 * This attempts to retrieve the token from the frontend's storage
 */
export async function getCsrfTokenFromPage(page: Page, recorder?: TestRecorder): Promise<string | null> {
  try {
    // Try to get the token from the page's evaluation context
    const token = await page.evaluate(() => {
      // Check if there's a global CSRF token variable
      if ((window as any).csrfToken) {
        return (window as any).csrfToken;
      }
      
      // Try to get it from localStorage
      const storedToken = localStorage.getItem('csrf_token');
      if (storedToken) {
        return storedToken;
      }
      
      return null;
    });
    
    if (token) {
      setCsrfToken(token);
      log(recorder, `🔑 CSRF token retrieved from page: ${token.substring(0, 10)}...`);
    }
    
    return token;
  } catch (err) {
    log(recorder, '⚠️ Failed to retrieve CSRF token from page');
    return null;
  }
}

/**
 * Add CSRF token to request headers
 * This should be used for all state-changing API requests (POST/PUT/PATCH/DELETE)
 */
export function addCsrfHeader(headers: Record<string, string>): Record<string, string> {
  const token = getCsrfToken();
  if (token) {
    return {
      ...headers,
      'X-CSRF-Token': token
    };
  }
  return headers;
}

/**
 * Check if a request method requires CSRF token
 */
export function requiresCsrfToken(method: string): boolean {
  const mutatingMethods = ['POST', 'PUT', 'PATCH', 'DELETE'];
  return mutatingMethods.includes(method.toUpperCase());
}

