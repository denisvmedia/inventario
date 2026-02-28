import { describe, it, expect, vi, beforeEach } from 'vitest'
import exportService from '../exportService'
// Mock shared API client used by exportService
vi.mock('../api', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn()
  }
}))

describe('exportService', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('pollRestoreStatus', () => {
    it('should poll until restore completes', async () => {
      const mockResponses = [
        { data: { data: { attributes: { id: 'restore-1', status: 'pending' } } } },
        { data: { data: { attributes: { id: 'restore-1', status: 'running' } } } },
        { data: { data: { attributes: { id: 'restore-1', status: 'completed' } } } }
      ]

      let callCount = 0
      vi.spyOn(exportService, 'getRestoreOperation').mockImplementation(() => {
        const response = mockResponses[callCount]
        callCount++
        return Promise.resolve(response)
      })

      const onUpdate = vi.fn()
      
      const result = await exportService.pollRestoreStatus(
        'export-1', 
        'restore-1', 
        onUpdate,
        100, // 100ms interval for fast test
        10   // max 10 attempts
      )

      expect(result.status).toBe('completed')
      expect(onUpdate).toHaveBeenCalledTimes(3)
      expect(onUpdate).toHaveBeenNthCalledWith(1, { id: 'restore-1', status: 'pending' })
      expect(onUpdate).toHaveBeenNthCalledWith(2, { id: 'restore-1', status: 'running' })
      expect(onUpdate).toHaveBeenNthCalledWith(3, { id: 'restore-1', status: 'completed' })
    })

    it('should handle failed restores', async () => {
      const mockResponses = [
        { data: { data: { attributes: { id: 'restore-1', status: 'running' } } } },
        { data: { data: { attributes: { id: 'restore-1', status: 'failed', error_message: 'Test error' } } } }
      ]

      let callCount = 0
      vi.spyOn(exportService, 'getRestoreOperation').mockImplementation(() => {
        const response = mockResponses[callCount]
        callCount++
        return Promise.resolve(response)
      })

      const result = await exportService.pollRestoreStatus(
        'export-1', 
        'restore-1', 
        undefined,
        100, // 100ms interval for fast test
        10   // max 10 attempts
      )

      expect(result.status).toBe('failed')
      expect(result.error_message).toBe('Test error')
    })

    it('should timeout after max attempts', async () => {
      vi.spyOn(exportService, 'getRestoreOperation').mockResolvedValue({
        data: { data: { attributes: { id: 'restore-1', status: 'running' } } }
      })

      await expect(
        exportService.pollRestoreStatus(
          'export-1', 
          'restore-1', 
          undefined,
          50,  // 50ms interval
          3    // max 3 attempts
        )
      ).rejects.toThrow('Restore polling timeout')
    })

    it('should handle network errors', async () => {
      vi.spyOn(exportService, 'getRestoreOperation').mockRejectedValue(
        new Error('Network error')
      )

      await expect(
        exportService.pollRestoreStatus('export-1', 'restore-1')
      ).rejects.toThrow('Network error')
    })
  })
})
