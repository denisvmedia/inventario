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
  }
}

export default locationService
