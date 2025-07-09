import axios from 'axios'

const API_URL = '/api/v1/commodities'

const commodityService = {
  getCommodities() {
    return axios.get(API_URL, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  getCommodity(id: string) {
    console.log(`Fetching commodity with ID: ${id}`)
    return axios.get(`${API_URL}/${id}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    }).then(response => {
      console.log('Commodity fetch successful:', response.data)
      return response
    }).catch(error => {
      console.error('Error fetching commodity:', error)
      throw error
    })
  },

  createCommodity(data: any) {
    console.log('commodityService: createCommodity called with data:', JSON.stringify(data, null, 2))
    return axios.post(API_URL, data, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    }).then(response => {
      console.log('commodityService: createCommodity success response:', response)
      return response
    }).catch(error => {
      console.error('commodityService: createCommodity error:', error)
      throw error
    })
  },

  updateCommodity(id: string, data: any) {
    return axios.put(`${API_URL}/${id}`, data, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  deleteCommodity(id: string) {
    return axios.delete(`${API_URL}/${id}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  // File upload methods - now using generic file entity system
  uploadImages(id: string, files: File[]) {
    const formData = new FormData()
    files.forEach(file => {
      formData.append('files', file)
    })

    return axios.post(`/api/v1/uploads/commodities/${id}/images`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
  },

  uploadManuals(id: string, files: File[]) {
    const formData = new FormData()
    files.forEach(file => {
      formData.append('files', file)
    })

    return axios.post(`/api/v1/uploads/commodities/${id}/manuals`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
  },

  uploadInvoices(id: string, files: File[]) {
    const formData = new FormData()
    files.forEach(file => {
      formData.append('files', file)
    })

    return axios.post(`/api/v1/uploads/commodities/${id}/invoices`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
  },

  // File retrieval methods
  getImages(id: string) {
    return axios.get(`${API_URL}/${id}/images`)
  },

  getManuals(id: string) {
    return axios.get(`${API_URL}/${id}/manuals`)
  },

  getInvoices(id: string) {
    return axios.get(`${API_URL}/${id}/invoices`)
  },

  // File update methods - now using generic file entity system
  updateImage(commodityId: string, imageId: string, data: any) {
    // Use the generic file service for updates
    return axios.put(`/api/v1/files/${imageId}`, {
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
    }, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  updateManual(commodityId: string, manualId: string, data: any) {
    // Use the generic file service for updates
    return axios.put(`/api/v1/files/${manualId}`, {
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
    }, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  updateInvoice(commodityId: string, invoiceId: string, data: any) {
    // Use the generic file service for updates
    return axios.put(`/api/v1/files/${invoiceId}`, {
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
    }, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  // File deletion methods - now using generic file entity system
  deleteImage(commodityId: string, imageId: string) {
    // Use the generic file service for deletion
    return axios.delete(`/api/v1/files/${imageId}`)
  },

  deleteManual(commodityId: string, manualId: string) {
    // Use the generic file service for deletion
    return axios.delete(`/api/v1/files/${manualId}`)
  },

  deleteInvoice(commodityId: string, invoiceId: string) {
    // Use the generic file service for deletion
    return axios.delete(`/api/v1/files/${invoiceId}`)
  }
}

export default commodityService
