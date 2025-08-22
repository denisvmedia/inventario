import api from './api'

const API_URL = '/api/v1'

const locationService = {
  getLocations() {
    return api.get(`${API_URL}/locations`)
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

  uploadImages(id: string, files: File[]) {
    const formData = new FormData()
    files.forEach(file => {
      formData.append('files', file)
    })

    return api.post(`/api/v1/uploads/locations/${id}/images`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
  },

  deleteImage(locationId: string, imageId: string) {
    return api.delete(`${API_URL}/locations/${locationId}/images/${imageId}`)
  }
}

export default locationService
