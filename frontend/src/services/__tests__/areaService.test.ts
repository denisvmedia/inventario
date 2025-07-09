import { describe, it, expect, vi, beforeEach } from 'vitest'
import axios from 'axios'
import areaService from '../areaService'
import { getErrorMessage } from '../../utils/errorUtils'

// Mock axios
vi.mock('axios')
const mockedAxios = vi.mocked(axios)

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

      mockedAxios.delete.mockRejectedValue(mockErrorResponse)

      try {
        await areaService.deleteArea('test-area-id')
        expect.fail('Expected deleteArea to throw an error')
      } catch (err: any) {
        // Test that our error utility can extract the meaningful message
        const userFriendlyMessage = getErrorMessage(err, 'area')
        expect(userFriendlyMessage).toBe('Cannot delete area because it contains commodities. Please remove all commodities first.')
      }

      expect(mockedAxios.delete).toHaveBeenCalledWith('/api/v1/areas/test-area-id', {
        headers: {
          'Accept': 'application/vnd.api+json'
        }
      })
    })

    it('should handle network errors gracefully', async () => {
      const networkError = new Error('Network Error')
      mockedAxios.delete.mockRejectedValue(networkError)

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

      mockedAxios.delete.mockRejectedValue(genericError)

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
