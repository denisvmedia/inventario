import apiClient from './api'

export default {
  getLocations() {
    return apiClient.get('/locations')
  },
  getLocation(id) {
    return apiClient.get(`/locations/${id}`)
  },
  createLocation(location) {
    return apiClient.post('/locations', {
      data: {
        type: 'locations',
        attributes: location
      }
    })
  },
  updateLocation(id, location) {
    return apiClient.put(`/locations/${id}`, {
      data: {
        id,
        type: 'locations',
        attributes: location
      }
    })
  },
  deleteLocation(id) {
    return apiClient.delete(`/locations/${id}`)
  }
}