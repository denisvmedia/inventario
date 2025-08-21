import api from './api'

const API_URL = '/api/v1/commodities/values'

const valueService = {
  /**
   * Get total values of commodities (global, by location, and by area)
   * @returns Promise with the response containing value data
   */
  getValues() {
    return api.get(API_URL)
  }
}

export default valueService
