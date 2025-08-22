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

  // File upload methods - now using generic file entity system
  uploadImages(id: string, files: File[]) {
    const formData = new FormData()
    files.forEach(file => {
      formData.append('files', file)
    })

    return api.post(`/api/v1/uploads/commodities/${id}/images`, formData, {
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

    return api.post(`/api/v1/uploads/commodities/${id}/manuals`, formData, {
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

    return api.post(`/api/v1/uploads/commodities/${id}/invoices`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
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
