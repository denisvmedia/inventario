import axios from 'axios'

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
    return axios.get(`${API_URL}/areas`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  getArea(id: string) {
    return axios.get(`${API_URL}/areas/${id}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  createArea(area: AreaPayload) {
    console.log('Creating area with data:', JSON.stringify(area, null, 2))
    // Use the standard areas endpoint
    return axios.post(`${API_URL}/areas`, area, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  updateArea(id: string, area: AreaPayload) {
    return axios.put(`${API_URL}/areas/${id}`, area, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  deleteArea(id: string) {
    return axios.delete(`${API_URL}/areas/${id}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  }
}
