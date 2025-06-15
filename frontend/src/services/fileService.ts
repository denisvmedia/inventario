import axios from 'axios'

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
}

const fileService = {
  /**
   * Get list of files with optional filtering and pagination
   */
  getFiles(params: FileListParams = {}) {
    return axios.get(API_URL, {
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
    return axios.get(`${API_URL}/${id}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    }).then(response => {
      console.log('File fetch successful:', response.data)
      return response
    }).catch(error => {
      console.error('Error fetching file:', error)
      throw error
    })
  },

  /**
   * Upload a file and create file entity
   */
  uploadFile(file: File) {
    const formData = new FormData()
    formData.append('file', file)

    return axios.post('/api/v1/uploads/files', formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    }).then(response => {
      console.log('File upload successful:', response.data)
      return response
    }).catch(error => {
      console.error('Error uploading file:', error)
      throw error
    })
  },

  /**
   * Create a new file entity with metadata only
   */
  createFile(metadata: FileCreateData) {
    return axios.post(API_URL, {
      data: {
        type: 'files',
        attributes: metadata
      }
    }, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
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
    return axios.put(`${API_URL}/${id}`, {
      data: {
        id: id,
        type: 'files',
        attributes: data
      }
    }, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
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
    return axios.delete(`${API_URL}/${id}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    }).then(response => {
      console.log('File deletion successful')
      return response
    }).catch(error => {
      console.error('Error deleting file:', error)
      throw error
    })
  },

  /**
   * Get download URL for a file
   */
  getDownloadUrl(file: FileEntity): string {
    const ext = file.ext.startsWith('.') ? file.ext.substring(1) : file.ext
    return `${API_URL}/${file.id}.${ext}`
  },

  /**
   * Download a file
   */
  downloadFile(file: FileEntity) {
    const url = this.getDownloadUrl(file)
    const link = document.createElement('a')
    link.href = url
    link.download = file.path + file.ext
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
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
        return 'bx-image'
      case 'document':
        if (file.mime_type === 'application/pdf') {
          return 'bx-file-pdf'
        }
        return 'bx-file-doc'
      case 'video':
        return 'bx-video'
      case 'audio':
        return 'bx-music'
      case 'archive':
        return 'bx-archive'
      default:
        return 'bx-file'
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
  }
}

export default fileService
