import { describe, it, expect, vi, beforeEach } from 'vitest'
import areaService from '../areaService'
import api from '../api'
import { getErrorMessage } from '../../utils/errorUtils'
vi.mock('../api', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn()
  }
}))

const mockedApi = vi.mocked(api)

describe('areaService error handling', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('deleteArea', () => {
    it('should handle area has commodities error correctly', async () => {
      // Mock the API response that matches the actual backend error structure
      const mockErrorResponse = {
        response: {
          status: 422,
          data: {
            errors: [
              {
                status: 'Unprocessable Entity',
                error: {
                  msg: 'area has commodities',
                  message: 'area has commodities',
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

      mockedApi.delete.mockRejectedValue(mockErrorResponse)

      try {
        await areaService.deleteArea('test-area-id')
        expect.fail('Expected deleteArea to throw an error')
      } catch (err: any) {
        // Test that our error utility can extract the meaningful message
        const userFriendlyMessage = getErrorMessage(err, 'area')
        expect(userFriendlyMessage).toBe('Cannot delete area because it contains commodities. Please remove all commodities first.')
      }

      expect(mockedApi.delete).toHaveBeenCalledWith('/api/v1/areas/test-area-id')
    })

    it('should handle network errors gracefully', async () => {
      const networkError = new Error('Network Error')
      mockedApi.delete.mockRejectedValue(networkError)

      try {
        await areaService.deleteArea('test-area-id')
        expect.fail('Expected deleteArea to throw an error')
      } catch (err: any) {
        const userFriendlyMessage = getErrorMessage(err, 'area')
        expect(userFriendlyMessage).toBe('Network Error')
      }
    })

    it('should handle generic API errors', async () => {
      const genericError = {
        response: {
          status: 500,
          data: {
            errors: [
              {
                status: 'Internal Server Error'
              }
            ]
          }
        }
      }

      mockedApi.delete.mockRejectedValue(genericError)

      try {
        await areaService.deleteArea('test-area-id')
        expect.fail('Expected deleteArea to throw an error')
      } catch (err: any) {
        const userFriendlyMessage = getErrorMessage(err, 'area')
        expect(userFriendlyMessage).toBe('Internal Server Error')
      }
    })
  })
})
