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

  // File upload methods
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

  // File update methods
  updateImage(commodityId: string, imageId: string, data: any) {
    return axios.put(`${API_URL}/${commodityId}/images/${imageId}`, {
      data: {
        id: imageId,
        type: 'images',
        attributes: {
          path: data.path
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
    return axios.put(`${API_URL}/${commodityId}/manuals/${manualId}`, {
      data: {
        id: manualId,
        type: 'manuals',
        attributes: {
          path: data.path
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
    return axios.put(`${API_URL}/${commodityId}/invoices/${invoiceId}`, {
      data: {
        id: invoiceId,
        type: 'invoices',
        attributes: {
          path: data.path
        }
      }
    }, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  // File deletion methods
  deleteImage(commodityId: string, imageId: string) {
    return axios.delete(`${API_URL}/${commodityId}/images/${imageId}`)
  },

  deleteManual(commodityId: string, manualId: string) {
    return axios.delete(`${API_URL}/${commodityId}/manuals/${manualId}`)
  },

  deleteInvoice(commodityId: string, invoiceId: string) {
    return axios.delete(`${API_URL}/${commodityId}/invoices/${invoiceId}`)
  }
}

export default commodityService
