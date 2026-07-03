import axios from 'axios'

const API_BASE = '/api/v1'

export const api = {
  // Health check
  getHealth: () => axios.get('/health'),
  
  // Tenants
  getTenants: () => axios.get(`${API_BASE}/tenants`),
  getTenant: (id) => axios.get(`${API_BASE}/tenants/${id}`),
  createTenant: (data) => axios.post(`${API_BASE}/tenants`, data),
  updateTenant: (id, data) => axios.put(`${API_BASE}/tenants/${id}`, data),
  deleteTenant: (id) => axios.delete(`${API_BASE}/tenants/${id}`),
  
  // Metrics
  getMetrics: () => axios.get('/metrics'),
  
  // Cache
  getCacheStatus: () => axios.get(`${API_BASE}/cache/status`),
}

export default api
