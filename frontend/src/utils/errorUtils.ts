/**
 * Utility functions for handling API errors and extracting meaningful error messages
 */

import { ref, readonly, computed } from 'vue'

interface APIError {
  response?: {
    status: number;
    data?: {
      errors?: Array<{
        status?: string;
        error?: any;
      }>;
    };
  };
  message?: string;
}

/**
 * Extracts a meaningful error message from an API error response
 * @param err The error object from axios or similar HTTP client
 * @param fallbackMessage Default message to use if no meaningful message can be extracted
 * @returns A user-friendly error message
 */
export function extractErrorMessage(err: APIError, fallbackMessage: string = 'An error occurred'): string {
  // If there's no response, use the error message or fallback
  if (!err.response?.data?.errors) {
    return err.message || fallbackMessage;
  }

  const errors = err.response.data.errors;
  if (!Array.isArray(errors) || errors.length === 0) {
    return fallbackMessage;
  }

  // Get the first error
  const firstError = errors[0];
  
  // Try to extract the meaningful message from the nested error structure
  const errorMessage = extractNestedErrorMessage(firstError.error);
  
  if (errorMessage) {
    return errorMessage;
  }

  // Fallback to status text or generic message
  return firstError.status || fallbackMessage;
}

/**
 * Recursively extracts error message from nested error structure
 * @param errorObj The error object that may contain nested errors
 * @returns The extracted error message or null if not found
 */
function extractNestedErrorMessage(errorObj: any): string | null {
  if (!errorObj) {
    return null;
  }

  // If this level has a msg property, use it
  if (typeof errorObj.msg === 'string' && errorObj.msg.trim()) {
    return errorObj.msg.trim();
  }

  // If this level has an error property, recurse into it
  if (errorObj.error) {
    const nestedMessage = extractNestedErrorMessage(errorObj.error);
    if (nestedMessage) {
      return nestedMessage;
    }
  }

  return null;
}

/**
 * Creates user-friendly error messages for specific business logic errors
 * @param rawMessage The raw error message from the API
 * @param context Additional context about the operation (e.g., 'area', 'location')
 * @returns A user-friendly error message
 */
export function createUserFriendlyMessage(rawMessage: string, context?: string): string {
  const lowerMessage = rawMessage.toLowerCase();
  
  // Handle area deletion errors
  if (lowerMessage.includes('area has commodities')) {
    return 'Cannot delete area because it contains commodities. Please remove all commodities first.';
  }
  
  // Handle location deletion errors
  if (lowerMessage.includes('location has areas')) {
    return 'Cannot delete location because it contains areas. Please remove all areas first.';
  }
  
  // Handle general "cannot delete" errors
  if (lowerMessage.includes('cannot delete')) {
    const entityType = context || 'item';
    return `Cannot delete ${entityType}. It may contain related data that must be removed first.`;
  }
  
  // Handle "already exists" errors
  if (lowerMessage.includes('already exists') || lowerMessage.includes('already used')) {
    return 'This name is already in use. Please choose a different name.';
  }
  
  // Handle "not found" errors
  if (lowerMessage.includes('not found')) {
    const entityType = context || 'item';
    return `The ${entityType} was not found. It may have been deleted by another user.`;
  }
  
  // Return the original message if no specific handling is needed
  return rawMessage;
}

/**
 * Extracts and formats a user-friendly error message from an API error
 * @param err The error object from axios or similar HTTP client
 * @param context Additional context about the operation (e.g., 'area', 'location')
 * @param fallbackMessage Default message to use if no meaningful message can be extracted
 * @returns A user-friendly error message
 */
export function getErrorMessage(err: APIError, context?: string, fallbackMessage?: string): string {
  const defaultFallback = fallbackMessage || `Failed to perform operation${context ? ` on ${context}` : ''}`;
  const rawMessage = extractErrorMessage(err, defaultFallback);
  return createUserFriendlyMessage(rawMessage, context);
}

/**
 * Interface for error objects in the error stack
 */
interface ErrorItem {
  id: string;
  message: string;
  timestamp: number;
  context?: string;
}

/**
 * Creates a composable for managing multiple persistent error states
 * @returns Object with error state management functions
 */
export function useErrorState() {
  const errors = ref<ErrorItem[]>([]);
  const showErrors = computed(() => errors.value.length > 0);

  const addError = (message: string, context?: string) => {
    const errorItem: ErrorItem = {
      id: `error-${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
      message,
      timestamp: Date.now(),
      context
    };

    // Add new error to the stack
    errors.value.push(errorItem);
  };

  const removeError = (errorId: string) => {
    errors.value = errors.value.filter(error => error.id !== errorId);
  };

  const clearAllErrors = () => {
    errors.value = [];
  };

  const handleError = (err: APIError, context?: string, fallbackMessage?: string) => {
    const message = getErrorMessage(err, context, fallbackMessage);
    addError(message, context);
  };

  // No cleanup needed since we removed auto-dismiss timeouts
  const cleanup = () => {
    // Reserved for future cleanup if needed
  };

  return {
    errors: readonly(errors),
    showErrors: readonly(showErrors),
    addError,
    removeError,
    clearAllErrors,
    handleError,
    cleanup
  };
}
