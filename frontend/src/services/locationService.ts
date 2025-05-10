import axios from 'axios'

const API_URL = '/api/v1'

const locationService = {
  getLocations() {
    return axios.get(`${API_URL}/locations`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  getLocation(id: string) {
    return axios.get(`${API_URL}/locations/${id}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  createLocation(locationData: any) {
    console.log('Creating location with data:', JSON.stringify(locationData, null, 2))
    return axios.post(`${API_URL}/locations`, locationData, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  updateLocation(id: string, locationData: any) {
    return axios.put(`${API_URL}/locations/${id}`, locationData, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  deleteLocation(id: string) {
    return axios.delete(`${API_URL}/locations/${id}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  // Image handling methods
  getImages(id: string) {
    return axios.get(`${API_URL}/locations/${id}/images`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  uploadImages(id: string, files: File[]) {
    const formData = new FormData()
    files.forEach(file => {
      formData.append('files', file)
    })

    return axios.post(`/api/v1/uploads/locations/${id}/images`, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      }
    })
  },

  deleteImage(locationId: string, imageId: string) {
    return axios.delete(`${API_URL}/locations/${locationId}/images/${imageId}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  }
}

export default locationService
