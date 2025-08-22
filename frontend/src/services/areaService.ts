import api from './api'

const API_URL = '/api/v1'

interface AreaPayload {
  data: {
    type: string;
    attributes: {
      name: string;
      location_id: string;
    };
  };
}

export default {
  getAreas() {
    return api.get(`${API_URL}/areas`)
  },

  getArea(id: string) {
    return api.get(`${API_URL}/areas/${id}`)
  },

  createArea(area: AreaPayload) {
    console.log('Creating area with data:', JSON.stringify(area, null, 2))
    // Use the standard areas endpoint
    return api.post(`${API_URL}/areas`, area)
  },

  updateArea(id: string, area: AreaPayload) {
    return api.put(`${API_URL}/areas/${id}`, area)
  },

  deleteArea(id: string) {
    return api.delete(`${API_URL}/areas/${id}`)
  }
}
