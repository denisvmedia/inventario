import axios from 'axios'

const API_URL = '/api/v1/commodities/values'

const valueService = {
  /**
   * Get total values of commodities (global, by location, and by area)
   * @returns Promise with the response containing value data
   */
  getValues() {
    return axios.get(API_URL, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  }
}

export default valueService
