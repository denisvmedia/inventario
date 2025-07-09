import { describe, it, expect } from 'vitest'
import { extractErrorMessage, createUserFriendlyMessage, getErrorMessage, useErrorState } from '../errorUtils'

describe('errorUtils', () => {
  describe('extractErrorMessage', () => {
    it('should extract message from nested error structure', () => {
      const err = {
        response: {
          status: 422,
          data: {
            errors: [
              {
                status: 'Unprocessable Entity',
                error: {
                  msg: 'area has commodities',
                  error: {
                    error: {
                      error: {
                        msg: 'cannot delete',
                        type: '*errors.errorString'
                      },
                      stackTrace: {
                        funcName: 'github.com/denisvmedia/inventario/internal/errkit.WithStack',
                        filePos: 'stacktracederr.go:30'
                      }
                    },
                    type: '*errkit.stackTracedError'
                  }
                }
              }
            ]
          }
        }
      }

      const result = extractErrorMessage(err, 'fallback')
      expect(result).toBe('area has commodities')
    })

    it('should return fallback when no errors array', () => {
      const err = {
        response: {
          status: 422,
          data: {}
        }
      }

      const result = extractErrorMessage(err, 'fallback message')
      expect(result).toBe('fallback message')
    })

    it('should return error message when no response', () => {
      const err = {
        message: 'Network error'
      }

      const result = extractErrorMessage(err, 'fallback')
      expect(result).toBe('Network error')
    })

    it('should return fallback when no response and no message', () => {
      const err = {}

      const result = extractErrorMessage(err, 'fallback')
      expect(result).toBe('fallback')
    })
  })

  describe('createUserFriendlyMessage', () => {
    it('should create user-friendly message for area has commodities', () => {
      const result = createUserFriendlyMessage('area has commodities')
      expect(result).toBe('Cannot delete area because it contains commodities. Please remove all commodities first.')
    })

    it('should create user-friendly message for location has areas', () => {
      const result = createUserFriendlyMessage('location has areas')
      expect(result).toBe('Cannot delete location because it contains areas. Please remove all areas first.')
    })

    it('should create user-friendly message for general cannot delete', () => {
      const result = createUserFriendlyMessage('cannot delete', 'item')
      expect(result).toBe('Cannot delete item. It may contain related data that must be removed first.')
    })

    it('should create user-friendly message for already exists', () => {
      const result = createUserFriendlyMessage('name already exists')
      expect(result).toBe('This name is already in use. Please choose a different name.')
    })

    it('should create user-friendly message for not found', () => {
      const result = createUserFriendlyMessage('not found', 'area')
      expect(result).toBe('The area was not found. It may have been deleted by another user.')
    })

    it('should return original message when no specific handling', () => {
      const result = createUserFriendlyMessage('some other error')
      expect(result).toBe('some other error')
    })
  })

  describe('getErrorMessage', () => {
    it('should extract and format error message from API error', () => {
      const err = {
        response: {
          status: 422,
          data: {
            errors: [
              {
                status: 'Unprocessable Entity',
                error: {
                  msg: 'area has commodities'
                }
              }
            ]
          }
        }
      }

      const result = getErrorMessage(err, 'area')
      expect(result).toBe('Cannot delete area because it contains commodities. Please remove all commodities first.')
    })

    it('should use fallback message when provided', () => {
      const err = {}

      const result = getErrorMessage(err, 'area', 'Custom fallback')
      expect(result).toBe('Custom fallback')
    })

    it('should generate default fallback with context', () => {
      const err = {}

      const result = getErrorMessage(err, 'area')
      expect(result).toBe('Failed to perform operation on area')
    })
  })

  describe('useErrorState', () => {
    it('should manage multiple error states correctly', () => {
      const { errors, showErrors, addError, removeError, clearAllErrors } = useErrorState()

      // Initially no errors
      expect(errors.value).toEqual([])
      expect(showErrors.value).toBe(false)

      // Add first error
      addError('First error message', 'area')
      expect(errors.value).toHaveLength(1)
      expect(errors.value[0].message).toBe('First error message')
      expect(errors.value[0].context).toBe('area')
      expect(showErrors.value).toBe(true)

      // Add second error
      addError('Second error message', 'location')
      expect(errors.value).toHaveLength(2)
      expect(errors.value[1].message).toBe('Second error message')
      expect(errors.value[1].context).toBe('location')

      // Remove first error
      const firstErrorId = errors.value[0].id
      removeError(firstErrorId)
      expect(errors.value).toHaveLength(1)
      expect(errors.value[0].message).toBe('Second error message')

      // Clear all errors
      clearAllErrors()
      expect(errors.value).toEqual([])
      expect(showErrors.value).toBe(false)
    })

    it('should handle API errors with handleError', () => {
      const { errors, showErrors, handleError } = useErrorState()

      const apiError = {
        response: {
          status: 422,
          data: {
            errors: [
              {
                status: 'Unprocessable Entity',
                error: {
                  msg: 'area has commodities'
                }
              }
            ]
          }
        }
      }

      handleError(apiError, 'area')
      expect(errors.value).toHaveLength(1)
      expect(errors.value[0].message).toBe('Cannot delete area because it contains commodities. Please remove all commodities first.')
      expect(errors.value[0].context).toBe('area')
      expect(showErrors.value).toBe(true)
    })

    it('should stack multiple errors from handleError', () => {
      const { errors, handleError } = useErrorState()

      const apiError1 = {
        response: {
          status: 422,
          data: {
            errors: [
              {
                status: 'Unprocessable Entity',
                error: {
                  msg: 'area has commodities'
                }
              }
            ]
          }
        }
      }

      const apiError2 = {
        response: {
          status: 422,
          data: {
            errors: [
              {
                status: 'Unprocessable Entity',
                error: {
                  msg: 'location has areas'
                }
              }
            ]
          }
        }
      }

      handleError(apiError1, 'area')
      handleError(apiError2, 'location')

      expect(errors.value).toHaveLength(2)
      expect(errors.value[0].message).toBe('Cannot delete area because it contains commodities. Please remove all commodities first.')
      expect(errors.value[1].message).toBe('Cannot delete location because it contains areas. Please remove all areas first.')
    })

    it('should generate unique IDs for each error', () => {
      const { errors, addError } = useErrorState()

      addError('Error 1')
      addError('Error 2')

      expect(errors.value).toHaveLength(2)
      expect(errors.value[0].id).not.toBe(errors.value[1].id)
      expect(errors.value[0].id).toMatch(/^error-\d+-[a-z0-9]+$/)
      expect(errors.value[1].id).toMatch(/^error-\d+-[a-z0-9]+$/)
    })
  })
})
