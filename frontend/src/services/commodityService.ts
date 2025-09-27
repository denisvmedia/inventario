import api from './api'

const API_URL = '/api/v1/commodities'

const commodityService = {
  getCommodities() {
    return api.get(API_URL)
  },

  getCommodity(id: string) {
    console.log(`Fetching commodity with ID: ${id}`)
    return api.get(`${API_URL}/${id}`).then(response => {
      console.log('Commodity fetch successful:', response.data)
      return response
    }).catch(error => {
      console.error('Error fetching commodity:', error)
      throw error
    })
  },

  createCommodity(data: any) {
    console.log('commodityService: createCommodity called with data:', JSON.stringify(data, null, 2))
    return api.post(API_URL, data).then(response => {
      console.log('commodityService: createCommodity success response:', response)
      return response
    }).catch(error => {
      console.error('commodityService: createCommodity error:', error)
      throw error
    })
  },

  updateCommodity(id: string, data: any) {
    return api.put(`${API_URL}/${id}`, data)
  },

  deleteCommodity(id: string) {
    return api.delete(`${API_URL}/${id}`)
  },

  // Single file upload methods - now using generic file entity system
  async uploadImage(id: string, file: File, _onProgress?: (_current: number, _total: number, _currentFile: string) => void): Promise<any> {
    const formData = new FormData()
    formData.append('file', file)

    const response = await api.post(`/api/v1/uploads/commodities/${id}/image`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      },
      onUploadProgress: (progressEvent) => {
        if (_onProgress && progressEvent.total) {
          const percentage = (progressEvent.loaded / progressEvent.total) * 100
          _onProgress(percentage < 100 ? 0 : 1, 1, file.name)
        }
      }
    })

    // Log the response to see the structure with signed URLs and thumbnails
    console.log('Image upload response:', response.data)
    return response
  },

  // Multiple file upload with proper slot-based concurrency control
  // TODO: Refactor to support per-file progress tracking and cancellation:
  // - Return upload job IDs for each file to enable cancellation
  // - Implement AbortController for each file upload
  // - Add file status tracking (queued, uploading, completed, failed, cancelled)
  // - Allow cancellation of queued files before they start
  // - Provide per-file progress callbacks instead of aggregate progress
  // - Remove completed files from UI automatically
  async uploadImages(id: string, files: File[], _onProgress?: (_current: number, _total: number, _currentFile: string) => void): Promise<any[]> {
    // Get max concurrent uploads from API
    const uploadSlotService = (await import('./uploadSlotService')).default
    const statusResponse = await uploadSlotService.getUploadStatus('image_upload')
    const maxConcurrent = statusResponse.data.attributes.max_uploads

    const results: any[] = []
    let completed = 0
    let activeUploads = 0
    const fileQueue = [...files] // Copy array to avoid mutation

    console.log(`📸 Starting upload of ${files.length} images with slot-based concurrency control (max: ${maxConcurrent})`)

    return new Promise((resolve) => {
      const processNext = async () => {
        // If no more files and no active uploads, we're done
        if (fileQueue.length === 0 && activeUploads === 0) {
          resolve(results)
          return
        }

        // If no more files but still have active uploads, wait
        if (fileQueue.length === 0) {
          return
        }

        // Try to start next upload
        const file = fileQueue.shift()!
        activeUploads++

        try {
          const result = await this.uploadImageWithRetry(id, file, (current, total, currentFile) => {
            if (_onProgress) {
              _onProgress(completed + current, files.length, currentFile)
            }
          })

          results.push(result)
          completed++
          if (_onProgress) {
            _onProgress(completed, files.length, file.name)
          }
        } catch (error) {
          completed++
          console.error(`Failed to upload ${file.name}:`, error)
          // Continue with other files even if one fails
        } finally {
          activeUploads--
          // Try to process next file
          processNext()
        }
      }

      // Start initial uploads using API-provided max concurrency
      const initialConcurrency = Math.min(maxConcurrent, files.length)
      for (let i = 0; i < initialConcurrency; i++) {
        processNext()
      }
    })
  },

  // Upload single image with retry logic for 429 responses
  async uploadImageWithRetry(id: string, file: File, _onProgress?: (_current: number, _total: number, _currentFile: string) => void, maxRetries: number = 10): Promise<any> {
    for (let attempt = 1; attempt <= maxRetries; attempt++) {
      try {
        return await this.uploadImage(id, file, _onProgress)
      } catch (error: any) {
        if (error.response?.status === 429 && attempt < maxRetries) {
          // Wait before retry (exponential backoff with jitter)
          const baseDelay = 100 * Math.pow(1.5, attempt - 1) // 100ms, 150ms, 225ms, etc.
          const jitter = Math.random() * 50 // Add 0-50ms jitter
          const delay = Math.min(baseDelay + jitter, 2000) // Cap at 2 seconds

          console.log(`Upload slot full for ${file.name}, retrying in ${Math.round(delay)}ms (attempt ${attempt}/${maxRetries})`)
          await new Promise(resolve => setTimeout(resolve, delay))
          continue
        }
        throw error
      }
    }
    throw new Error(`Failed to upload ${file.name} after ${maxRetries} attempts`)
  },

  async uploadManual(id: string, file: File, _onProgress?: (_current: number, _total: number, _currentFile: string) => void): Promise<any> {
    const formData = new FormData()
    formData.append('file', file)

    const response = await api.post(`/api/v1/uploads/commodities/${id}/manual`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      },
      onUploadProgress: (progressEvent) => {
        if (_onProgress && progressEvent.total) {
          const percentage = (progressEvent.loaded / progressEvent.total) * 100
          _onProgress(percentage < 100 ? 0 : 1, 1, file.name)
        }
      }
    })

    console.log('Manual upload response:', response.data)
    return response
  },

  // TODO: Refactor to support per-file progress tracking and cancellation:
  // - Return upload job IDs for each file to enable cancellation
  // - Implement AbortController for each file upload
  // - Add file status tracking (queued, uploading, completed, failed, cancelled)
  // - Allow cancellation of queued files before they start
  // - Provide per-file progress callbacks instead of aggregate progress
  // - Remove completed files from UI automatically
  async uploadManuals(id: string, files: File[], _onProgress?: (_current: number, _total: number, _currentFile: string) => void): Promise<any[]> {
    // Get max concurrent uploads from API
    const uploadSlotService = (await import('./uploadSlotService')).default
    const statusResponse = await uploadSlotService.getUploadStatus('document_upload')
    const maxConcurrent = statusResponse.data.attributes.max_uploads

    const results: any[] = []
    let completed = 0
    let activeUploads = 0
    const fileQueue = [...files]

    console.log(`📄 Starting upload of ${files.length} manuals with slot-based concurrency control (max: ${maxConcurrent})`)

    return new Promise((resolve, _reject) => {
      const processNext = async () => {
        if (fileQueue.length === 0 && activeUploads === 0) {
          resolve(results)
          return
        }

        if (fileQueue.length === 0) {
          return
        }

        const file = fileQueue.shift()!
        activeUploads++

        try {
          const result = await this.uploadManualWithRetry(id, file, (current, total, currentFile) => {
            if (_onProgress) {
              _onProgress(completed + current, files.length, currentFile)
            }
          })

          results.push(result)
          completed++
          if (_onProgress) {
            _onProgress(completed, files.length, file.name)
          }
        } catch (error) {
          completed++
          console.error(`Failed to upload ${file.name}:`, error)
        } finally {
          activeUploads--
          processNext()
        }
      }

      const initialConcurrency = Math.min(maxConcurrent, files.length)
      for (let i = 0; i < initialConcurrency; i++) {
        processNext()
      }
    })
  },

  // Upload single manual with retry logic for 429 responses
  async uploadManualWithRetry(id: string, file: File, _onProgress?: (_current: number, _total: number, _currentFile: string) => void, maxRetries: number = 10): Promise<any> {
    for (let attempt = 1; attempt <= maxRetries; attempt++) {
      try {
        return await this.uploadManual(id, file, _onProgress)
      } catch (error: any) {
        if (error.response?.status === 429 && attempt < maxRetries) {
          const baseDelay = 100 * Math.pow(1.5, attempt - 1)
          const jitter = Math.random() * 50
          const delay = Math.min(baseDelay + jitter, 2000)

          console.log(`Upload slot full for ${file.name}, retrying in ${Math.round(delay)}ms (attempt ${attempt}/${maxRetries})`)
          await new Promise(resolve => setTimeout(resolve, delay))
          continue
        }
        throw error
      }
    }
    throw new Error(`Failed to upload ${file.name} after ${maxRetries} attempts`)
  },

  async uploadInvoice(id: string, file: File, _onProgress?: (_current: number, _total: number, _currentFile: string) => void): Promise<any> {
    const formData = new FormData()
    formData.append('file', file)

    const response = await api.post(`/api/v1/uploads/commodities/${id}/invoice`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      },
      onUploadProgress: (progressEvent) => {
        if (_onProgress && progressEvent.total) {
          const percentage = (progressEvent.loaded / progressEvent.total) * 100
          _onProgress(percentage < 100 ? 0 : 1, 1, file.name)
        }
      }
    })

    console.log('Invoice upload response:', response.data)
    return response
  },

  // TODO: Refactor to support per-file progress tracking and cancellation:
  // - Return upload job IDs for each file to enable cancellation
  // - Implement AbortController for each file upload
  // - Add file status tracking (queued, uploading, completed, failed, cancelled)
  // - Allow cancellation of queued files before they start
  // - Provide per-file progress callbacks instead of aggregate progress
  // - Remove completed files from UI automatically
  async uploadInvoices(id: string, files: File[], _onProgress?: (_current: number, _total: number, _currentFile: string) => void): Promise<any[]> {
    // Get max concurrent uploads from API
    const uploadSlotService = (await import('./uploadSlotService')).default
    const statusResponse = await uploadSlotService.getUploadStatus('document_upload')
    const maxConcurrent = statusResponse.data.attributes.max_uploads

    const results: any[] = []
    let completed = 0
    let activeUploads = 0
    const fileQueue = [...files]

    console.log(`🧾 Starting upload of ${files.length} invoices with slot-based concurrency control (max: ${maxConcurrent})`)

    return new Promise((resolve, _reject) => {
      const processNext = async () => {
        if (fileQueue.length === 0 && activeUploads === 0) {
          resolve(results)
          return
        }

        if (fileQueue.length === 0) {
          return
        }

        const file = fileQueue.shift()!
        activeUploads++

        try {
          const result = await this.uploadInvoiceWithRetry(id, file, (current, total, currentFile) => {
            if (_onProgress) {
              _onProgress(completed + current, files.length, currentFile)
            }
          })

          results.push(result)
          completed++
          if (_onProgress) {
            _onProgress(completed, files.length, file.name)
          }
        } catch (error) {
          completed++
          console.error(`Failed to upload ${file.name}:`, error)
        } finally {
          activeUploads--
          processNext()
        }
      }

      const initialConcurrency = Math.min(maxConcurrent, files.length)
      for (let i = 0; i < initialConcurrency; i++) {
        processNext()
      }
    })
  },

  // Upload single invoice with retry logic for 429 responses
  async uploadInvoiceWithRetry(id: string, file: File, _onProgress?: (_current: number, _total: number, _currentFile: string) => void, maxRetries: number = 10): Promise<any> {
    for (let attempt = 1; attempt <= maxRetries; attempt++) {
      try {
        return await this.uploadInvoice(id, file, _onProgress)
      } catch (error: any) {
        if (error.response?.status === 429 && attempt < maxRetries) {
          const baseDelay = 100 * Math.pow(1.5, attempt - 1)
          const jitter = Math.random() * 50
          const delay = Math.min(baseDelay + jitter, 2000)

          console.log(`Upload slot full for ${file.name}, retrying in ${Math.round(delay)}ms (attempt ${attempt}/${maxRetries})`)
          await new Promise(resolve => setTimeout(resolve, delay))
          continue
        }
        throw error
      }
    }
    throw new Error(`Failed to upload ${file.name} after ${maxRetries} attempts`)
  },

  // File retrieval methods
  getImages(id: string) {
    return api.get(`${API_URL}/${id}/images`)
  },

  getManuals(id: string) {
    return api.get(`${API_URL}/${id}/manuals`)
  },

  getInvoices(id: string) {
    return api.get(`${API_URL}/${id}/invoices`)
  },

  // File update methods - now using generic file entity system
  updateImage(commodityId: string, imageId: string, data: any) {
    // Use the generic file service for updates
    return api.put(`/api/v1/files/${imageId}`, {
      data: {
        id: imageId,
        type: 'files',
        attributes: {
          title: data.title || data.path,
          description: data.description || '',
          tags: data.tags || [],
          path: data.path,
          linked_entity_type: 'commodity',
          linked_entity_id: commodityId,
          linked_entity_meta: 'images'
        }
      }
    })
  },

  updateManual(commodityId: string, manualId: string, data: any) {
    // Use the generic file service for updates
    return api.put(`/api/v1/files/${manualId}`, {
      data: {
        id: manualId,
        type: 'files',
        attributes: {
          title: data.title || data.path,
          description: data.description || '',
          tags: data.tags || [],
          path: data.path,
          linked_entity_type: 'commodity',
          linked_entity_id: commodityId,
          linked_entity_meta: 'manuals'
        }
      }
    })
  },

  updateInvoice(commodityId: string, invoiceId: string, data: any) {
    // Use the generic file service for updates
    return api.put(`/api/v1/files/${invoiceId}`, {
      data: {
        id: invoiceId,
        type: 'files',
        attributes: {
          title: data.title || data.path,
          description: data.description || '',
          tags: data.tags || [],
          path: data.path,
          linked_entity_type: 'commodity',
          linked_entity_id: commodityId,
          linked_entity_meta: 'invoices'
        }
      }
    })
  },

  // File deletion methods - now using generic file entity system
  deleteImage(commodityId: string, imageId: string) {
    // Use the generic file service for deletion
    return api.delete(`/api/v1/files/${imageId}`)
  },

  deleteManual(commodityId: string, manualId: string) {
    // Use the generic file service for deletion
    return api.delete(`/api/v1/files/${manualId}`)
  },

  deleteInvoice(commodityId: string, invoiceId: string) {
    // Use the generic file service for deletion
    return api.delete(`/api/v1/files/${invoiceId}`)
  }
}

export default commodityService
