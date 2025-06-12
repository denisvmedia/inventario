import axios from 'axios'

const API_URL = '/api/v1/exports'

const exportService = {
  getExports(includeDeleted = false) {
    const params = includeDeleted ? { include_deleted: 'true' } : {}
    return axios.get(API_URL, {
      params,
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  getExport(id: string) {
    console.log(`Fetching export with ID: ${id}`)
    return axios.get(`${API_URL}/${id}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    }).then(response => {
      console.log('Export fetch successful:', response.data)
      return response
    }).catch(error => {
      console.error('Error fetching export:', error)
      throw error
    })
  },

  createExport(data: any) {
    console.log('exportService: createExport called with data:', JSON.stringify(data, null, 2))
    return axios.post(API_URL, data, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    }).then(response => {
      console.log('exportService: createExport success response:', response)
      return response
    }).catch(error => {
      console.error('exportService: createExport error:', error)
      throw error
    })
  },

  updateExport(id: string, data: any) {
    return axios.patch(`${API_URL}/${id}`, data, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  deleteExport(id: string) {
    return axios.delete(`${API_URL}/${id}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  downloadExport(id: string) {
    return axios.get(`${API_URL}/${id}/download`, {
      responseType: 'blob',
      headers: {
        'Accept': 'application/xml'
      }
    })
  },

  // Restore operations for exports
  getRestoreOperations(exportId: string) {
    return axios.get(`${API_URL}/${exportId}/restores`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  createRestore(exportId: string, data: any) {
    console.log('exportService: createRestore called with data:', JSON.stringify(data, null, 2))
    return axios.post(`${API_URL}/${exportId}/restores`, data, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    }).then(response => {
      console.log('exportService: createRestore success response:', response)
      return response
    }).catch(error => {
      console.error('exportService: createRestore error:', error)
      throw error
    })
  },

  getRestoreOperation(exportId: string, restoreId: string) {
    return axios.get(`${API_URL}/${exportId}/restores/${restoreId}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  }
}

export default exportService
