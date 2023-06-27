import axios from 'axios';

const API_BASE_URL = 'http://localhost:3333/api/v1'; // Replace with your API base URL

const api = axios.create({
  baseURL: API_BASE_URL,
  headers: {
    'Content-Type': 'application/json',
  },
});

export default api;
