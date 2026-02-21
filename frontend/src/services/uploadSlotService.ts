import api from './api'

const API_URL = '/api/v1/upload-slots'

export interface UploadStatus {
  operation_name: string
  active_uploads: number
  max_uploads: number
  available_uploads: number
  can_start_upload: boolean
  retry_after_seconds?: number
}

export interface UploadStatusResponse {
  data: {
    id: string
    type: 'upload-status'
    attributes: UploadStatus
  }
}

export interface UploadCapacityError {
  retry_after_seconds?: number
  message: string
}

const uploadSlotService = {
  /**
   * Check upload capacity for a specific operation
   */
  async checkUploadCapacity(operationName: string): Promise<UploadStatusResponse> {
    try {
      const response = await api.get(`${API_URL}/check`, {
        params: { operation: operationName },
        headers: {
          'Accept': 'application/vnd.api+json'
        }
      })

      console.log(`✅ Upload capacity check for ${operationName}:`, response.data.data.attributes)
      return response.data
    } catch (error: any) {
      console.error(`❌ Failed to check upload capacity for ${operationName}:`, error)

      // Handle 429 Too Many Requests
      if (error.response?.status === 429) {
        const errorData = error.response.data
        const msg = `Too many concurrent uploads. ${
          errorData.retry_after_seconds
            ? `Retry after ${errorData.retry_after_seconds} seconds.`
            : 'Try again later.'
        }`

        throw new Error(msg, { cause: error })
      }

      throw error
    }
  },

  /**
   * Get current upload status for an operation
   */
  async getUploadStatus(operationName: string): Promise<UploadStatusResponse> {
    try {
      const response = await api.get(`${API_URL}/status`, {
        params: { operation: operationName },
        headers: {
          'Accept': 'application/vnd.api+json'
        }
      })

      console.log(`📊 Upload status for ${operationName}:`, response.data.data.attributes)
      return response.data
    } catch (error) {
      console.error(`❌ Failed to get upload status for ${operationName}:`, error)
      throw error
    }
  },

  /**
   * Check if uploads are available for a specific operation
   */
  async checkAvailability(operationName: string): Promise<boolean> {
    try {
      const status = await this.getUploadStatus(operationName)
      return status.data.attributes.can_start_upload
    } catch (error) {
      console.error(`❌ Failed to check availability for ${operationName}:`, error)
      return false
    }
  },

  /**
   * Wait for upload capacity to become available with retry logic
   */
  async waitForCapacity(
    operationName: string,
    maxRetries: number = 5,
    baseDelay: number = 1000
  ): Promise<UploadStatusResponse> {
    let lastError: Error | null = null

    for (let attempt = 0; attempt < maxRetries; attempt++) {
      try {
        const status = await this.checkUploadCapacity(operationName)
        if (status.data.attributes.can_start_upload) {
          return status
        }

        // If we can't start upload, treat it as a 429 error
        throw new Error(`Too many concurrent uploads for ${operationName}`)
      } catch (error: any) {
        lastError = error

        // Don't retry on client errors (4xx except 429)
        if (error.response?.status >= 400 && error.response?.status < 500 && error.response?.status !== 429) {
          throw error
        }

        // Calculate delay with exponential backoff
        const delay = baseDelay * Math.pow(2, attempt)

        // Use retry_after from server if available
        const retryAfter = error.response?.data?.retry_after_seconds
        const actualDelay = retryAfter ? retryAfter * 1000 : delay

        console.log(`⏳ Retrying upload capacity check in ${actualDelay}ms (attempt ${attempt + 1}/${maxRetries})`)

        if (attempt < maxRetries - 1) {
          await new Promise(resolve => setTimeout(resolve, actualDelay))
        }
      }
    }

    throw lastError || new Error('Failed to get upload capacity after retries')
  },

  /**
   * Wait for upload capacity to become available with polling
   */
  async waitForAvailability(
    operationName: string,
    maxWaitTime: number = 30000,
    pollInterval: number = 2000
  ): Promise<boolean> {
    const startTime = Date.now()

    while (Date.now() - startTime < maxWaitTime) {
      const available = await this.checkAvailability(operationName)
      if (available) {
        return true
      }

      console.log(`⏳ Waiting for ${operationName} upload capacity to become available...`)
      await new Promise(resolve => setTimeout(resolve, pollInterval))
    }

    return false
  }
}

export default uploadSlotService
