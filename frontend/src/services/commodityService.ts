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
    return axios.post(API_URL, data, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
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
  }
}

export default commodityService
