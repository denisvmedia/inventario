import apiClient from './api'

export default {
  getAreas() {
    return apiClient.get('/areas')
  },
  getArea(id) {
    return apiClient.get(`/areas/${id}`)
  },
  createArea(area) {
    return apiClient.post('/areas', {
      data: {
        type: 'areas',
        attributes: area
      }
    })
  },
  updateArea(id, area) {
    return apiClient.put(`/areas/${id}`, {
      data: {
        id,
        type: 'areas',
        attributes: area
      }
    })
  },
  deleteArea(id) {
    return apiClient.delete(`/areas/${id}`)
  }
}