import apiClient from './api'

export default {
  getCommodities() {
    return apiClient.get('/commodities')
  },
  getCommodity(id) {
    return apiClient.get(`/commodities/${id}`)
  },
  createCommodity(commodity) {
    return apiClient.post('/commodities', {
      data: {
        type: 'commodities',
        attributes: commodity
      }
    })
  },
  updateCommodity(id, commodity) {
    return apiClient.put(`/commodities/${id}`, {
      data: {
        id,
        type: 'commodities',
        attributes: commodity
      }
    })
  },
  deleteCommodity(id) {
    return apiClient.delete(`/commodities/${id}`)
  },
  getCommodityImages(commodityId) {
    return apiClient.get(`/commodities/${commodityId}/images`)
  },
  getCommodityInvoices(commodityId) {
    return apiClient.get(`/commodities/${commodityId}/invoices`)
  },
  getCommodityManuals(commodityId) {
    return apiClient.get(`/commodities/${commodityId}/manuals`)
  }
}