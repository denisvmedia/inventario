import api from './api'

const API_URL = '/api/v1'

const locationService = {
  getLocations(params?: { page?: number; per_page?: number }) {
    return api.get(`${API_URL}/locations`, { params })
  },

  getLocation(id: string) {
    return api.get(`${API_URL}/locations/${id}`)
  },

  createLocation(locationData: any) {
    console.log('Creating location with data:', JSON.stringify(locationData, null, 2))
    return api.post(`${API_URL}/locations`, locationData)
  },

  updateLocation(id: string, locationData: any) {
    return api.put(`${API_URL}/locations/${id}`, locationData)
  },

  deleteLocation(id: string) {
    return api.delete(`${API_URL}/locations/${id}`)
  },

  // Image handling methods
  getImages(id: string) {
    return api.get(`${API_URL}/locations/${id}/images`)
  },

  uploadImage(id: string, file: File) {
    const formData = new FormData()
    formData.append('file', file)
    return api.post(`${API_URL}/uploads/locations/${id}/image`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })
  },

  deleteImage(locationId: string, imageId: string) {
    return api.delete(`${API_URL}/locations/${locationId}/images/${imageId}`)
  },

  // File handling methods
  getFiles(id: string) {
    return api.get(`${API_URL}/locations/${id}/files`)
  },

  uploadFile(id: string, file: File) {
    const formData = new FormData()
    formData.append('file', file)
    return api.post(`${API_URL}/uploads/locations/${id}/file`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' }
    })
  },

  deleteFile(locationId: string, fileId: string) {
    return api.delete(`${API_URL}/locations/${locationId}/files/${fileId}`)
  }
}

export default locationService
