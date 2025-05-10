import axios from 'axios'

const api = axios.create({
  baseURL: '',  // Empty because we're using Vite's proxy
  headers: {
    'Content-Type': 'application/vnd.api+json',
    'Accept': 'application/vnd.api+json'
  }
})

// Add request interceptor for detailed debugging
api.interceptors.request.use(
  config => {
    console.log('API Request URL:', config.url)
    console.log('API Request Method:', config.method?.toUpperCase())
    console.log('API Request Headers:', JSON.stringify(config.headers, null, 2))
    console.log('API Request Data:', JSON.stringify(config.data, null, 2))
    return config
  },
  error => {
    console.error('API Request Error:', error)
    return Promise.reject(error)
  }
)

// Add response interceptor for detailed debugging
api.interceptors.response.use(
  response => {
    console.log('API Response Status:', response.status)
    console.log('API Response Headers:', JSON.stringify(response.headers, null, 2))
    console.log('API Response Data:', JSON.stringify(response.data, null, 2))
    return response
  },
  error => {
    console.error('API Response Error Status:', error.response?.status)
    console.error('API Response Error Data:', JSON.stringify(error.response?.data, null, 2))
    return Promise.reject(error)
  }
)

export default api
