import api from './api'

const API_URL = '/api/v1/files'

export interface FileEntity {
  id: string
  title: string
  description: string
  type: 'image' | 'document' | 'video' | 'audio' | 'archive' | 'other'
  tags: string[]
  path: string
  original_path: string
  ext: string
  mime_type: string
  linked_entity_type?: string
  linked_entity_id?: string
  linked_entity_meta?: string
  created_at?: string
  updated_at?: string
}

export interface FileListParams {
  type?: string
  search?: string
  tags?: string
  page?: number
  limit?: number
}

export interface FileCreateData {
  title: string
  description: string
  tags: string[]
}

export interface FileUpdateData {
  title: string
  description: string
  tags: string[]
  path: string
  linked_entity_type?: string
  linked_entity_id?: string
  linked_entity_meta?: string
}



const fileService = {
  /**
   * Get list of files with optional filtering and pagination
   */
  getFiles(params: FileListParams = {}) {
    return api.get(API_URL, {
      params,
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  /**
   * Get a single file by ID
   */
  getFile(id: string) {
    console.log(`Fetching file with ID: ${id}`)
    return api.get(`${API_URL}/${id}`).then(response => {
      console.log('File fetch successful:', response.data)
      return response
    }).catch(error => {
      console.error('Error fetching file:', error)
      throw error
    })
  },

  /**
   * Upload a single file and create file entity
   */
  async uploadFile(file: File, _onProgress?: (number, number, string) => void) {
    const formData = new FormData()
    formData.append('file', file)

    try {
      const response = await api.post('/api/v1/uploads/file', formData, {
        headers: {
          'Content-Type': 'multipart/form-data'
        },
        onUploadProgress: (progressEvent) => {
          if (onProgress && progressEvent.total) {
            const percentage = (progressEvent.loaded / progressEvent.total) * 100
            onProgress(percentage < 100 ? 0 : 1, 1, file.name)
          }
        }
      })

      console.log('File upload successful with signed URLs:', response.data)
      return response
    } catch (error) {
      console.error('Error uploading file:', error)
      throw error
    }
  },

  /**
   * Create a new file entity with metadata only
   */
  createFile(metadata: FileCreateData) {
    return api.post(API_URL, {
      data: {
        type: 'files',
        attributes: metadata
      }
    }).then(response => {
      console.log('File creation successful:', response.data)
      return response
    }).catch(error => {
      console.error('Error creating file:', error)
      throw error
    })
  },

  /**
   * Update file metadata
   */
  updateFile(id: string, data: FileUpdateData) {
    return api.put(`${API_URL}/${id}`, {
      data: {
        id: id,
        type: 'files',
        attributes: data
      }
    }).then(response => {
      console.log('File update successful:', response.data)
      return response
    }).catch(error => {
      console.error('Error updating file:', error)
      throw error
    })
  },

  /**
   * Delete a file
   */
  deleteFile(id: string) {
    return api.delete(`${API_URL}/${id}`).then(response => {
      console.log('File deletion successful')
      return response
    }).catch(error => {
      console.error('Error deleting file:', error)
      throw error
    })
  },

  /**
   * Generate a signed URL for file download
   */
  async generateSignedUrl(file: FileEntity): Promise<string> {
    try {
      const response = await api.post(`${API_URL}/${file.id}/signed-url`)
      // Parse JSON:API response format
      return response.data.attributes.url
    } catch (error) {
      console.error('Failed to generate signed URL:', error)
      throw error
    }
  },

  /**
   * Generate signed URLs with thumbnails for a file
   */
  async generateSignedUrlWithThumbnails(file: FileEntity): Promise<{ url: string; thumbnails?: Record<string, string> }> {
    try {
      const response = await api.post(`${API_URL}/${file.id}/signed-url`)
      // Parse JSON:API response format
      return {
        url: response.data.attributes.url,
        thumbnails: response.data.attributes.thumbnails
      }
    } catch (error) {
      console.error('Failed to generate signed URL with thumbnails:', error)
      throw error
    }
  },

  /**
   * Get download URL for a file (generates signed URL)
   */
  async getDownloadUrl(file: FileEntity): Promise<string> {
    return this.generateSignedUrl(file)
  },

  /**
   * Download a file
   */
  async downloadFile(file: FileEntity) {
    try {
      const url = await this.getDownloadUrl(file)
      const link = document.createElement('a')
      link.href = url
      link.download = file.path + file.ext
      document.body.appendChild(link)
      link.click()
      document.body.removeChild(link)
    } catch (error) {
      console.error('Failed to download file:', error)
      throw error
    }
  },



  /**
   * Check if a file is an image based on its MIME type
   */
  isImageFile(file: { mime_type?: string; type?: string }): boolean {
    if (file.mime_type) {
      return file.mime_type.startsWith('image/')
    }
    if (file.type) {
      return file.type === 'image'
    }
    return false
  },

  /**
   * Get file type options for forms
   */
  getFileTypeOptions() {
    return [
      { value: 'image', label: 'Image' },
      { value: 'document', label: 'Document' },
      { value: 'video', label: 'Video' },
      { value: 'audio', label: 'Audio' },
      { value: 'archive', label: 'Archive' },
      { value: 'other', label: 'Other' }
    ]
  },

  /**
   * Get file icon based on file type
   */
  getFileIcon(file: FileEntity): string {
    switch (file.type) {
      case 'image':
        return 'image'
      case 'document':
        if (file.mime_type === 'application/pdf') {
          return 'file-pdf'
        }
        return 'file-alt'
      case 'video':
        return 'video'
      case 'audio':
        return 'music'
      case 'archive':
        return 'archive'
      default:
        return 'file'
    }
  },

  /**
   * Check if file can be previewed
   */
  canPreview(file: FileEntity): boolean {
    return file.type === 'image' || file.mime_type === 'application/pdf'
  },

  /**
   * Format file size for display
   */
  formatFileSize(bytes: number): string {
    if (bytes === 0) return '0 Bytes'

    const k = 1024
    const sizes = ['Bytes', 'KB', 'MB', 'GB']
    const i = Math.floor(Math.log(bytes) / Math.log(k))

    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
  },

  /**
   * Get display title for a file (title or filename fallback)
   */
  getDisplayTitle(file: FileEntity): string {
    if (file.title && file.title.trim()) {
      return file.title
    }
    if (file.path && file.path.trim()) {
      return file.path
    }
    return 'Untitled'
  },

  /**
   * Check if file is linked to an entity
   */
  isLinked(file: FileEntity): boolean {
    return !!(file.linked_entity_type && file.linked_entity_id)
  },

  /**
   * Get linked entity display name
   */
  getLinkedEntityDisplay(file: FileEntity): string {
    if (!this.isLinked(file)) {
      return ''
    }

    const type = file.linked_entity_type
    const meta = file.linked_entity_meta

    if (type === 'commodity') {
      switch (meta) {
        case 'images': return 'Commodity Images'
        case 'invoices': return 'Commodity Invoices'
        case 'manuals': return 'Commodity Manuals'
        default: return 'Commodity'
      }
    } else if (type === 'export') {
      return `Export (${meta})`
    }

    return type || ''
  },

  /**
   * Get navigation URL for linked entity
   */
  getLinkedEntityUrl(file: FileEntity, currentRoute?: any): string {
    if (!this.isLinked(file)) {
      return ''
    }

    if (file.linked_entity_type === 'commodity') {
      return `/commodities/${file.linked_entity_id}`
    } else if (file.linked_entity_type === 'export') {
      // Determine the source page context
      let fromPage = 'file-view'
      if (currentRoute) {
        if (currentRoute.name === 'files') {
          fromPage = 'file-list'
        } else if (currentRoute.name === 'file-edit') {
          fromPage = 'file-edit'
        } else if (currentRoute.name === 'file-detail') {
          fromPage = 'file-view'
        }
      }
      return `/exports/${file.linked_entity_id}?from=${fromPage}&fileId=${file.id}`
    }

    return ''
  },

  /**
   * Check if a file is linked to an export (readonly)
   */
  isExportFile(file: FileEntity): boolean {
    return file.linked_entity_type === 'export'
  },

  /**
   * Check if a file can be manually deleted
   */
  canDelete(file: FileEntity): boolean {
    return !this.isExportFile(file)
  },

  /**
   * Check if a file can be manually unlinked from its entity
   */
  canUnlink(file: FileEntity): boolean {
    return !this.isExportFile(file)
  },

  /**
   * Get explanation for why a file cannot be deleted
   */
  getDeleteRestrictionReason(file: FileEntity): string {
    if (this.isExportFile(file)) {
      return 'Export files cannot be manually deleted. Delete the export to remove this file.'
    }
    return ''
  }
}

export default fileService
