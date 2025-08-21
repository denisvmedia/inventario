import api from './api'

const API_URL = '/api/v1/exports'

const exportService = {
  getExports(includeDeleted = false) {
    const params = includeDeleted ? { include_deleted: 'true' } : {}
    return api.get(API_URL, { params })
  },

  getExport(id: string) {
    console.log(`Fetching export with ID: ${id}`)
    return api.get(`${API_URL}/${id}`).then(response => {
      console.log('Export fetch successful:', response.data)
      return response
    }).catch(error => {
      console.error('Error fetching export:', error)
      throw error
    })
  },

  createExport(data: any) {
    console.log('exportService: createExport called with data:', JSON.stringify(data, null, 2))
    return api.post(API_URL, data).then(response => {
      console.log('exportService: createExport success response:', response)
      return response
    }).catch(error => {
      console.error('exportService: createExport error:', error)
      throw error
    })
  },

  updateExport(id: string, data: any) {
    return api.patch(`${API_URL}/${id}`, data)
  },

  deleteExport(id: string) {
    return api.delete(`${API_URL}/${id}`)
  },

  downloadExport(id: string) {
    return api.get(`${API_URL}/${id}/download`, {
      responseType: 'blob',
      headers: {
        'Accept': 'application/xml'
      }
    })
  },

  importExport(data: any) {
    console.log('exportService: importExport called with data:', JSON.stringify(data, null, 2))
    return api.post(`${API_URL}/import`, data).then(response => {
      console.log('exportService: importExport success response:', response)
      return response
    }).catch(error => {
      console.error('exportService: importExport error:', error)
      throw error
    })
  },

  // Restore operations for exports
  getRestoreOperations(exportId: string) {
    return axios.get(`${API_URL}/${exportId}/restores`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  createRestore(exportId: string, data: any) {
    console.log('exportService: createRestore called with data:', JSON.stringify(data, null, 2))
    return axios.post(`${API_URL}/${exportId}/restores`, data, {
      headers: {
        'Content-Type': 'application/vnd.api+json',
        'Accept': 'application/vnd.api+json'
      }
    }).then(response => {
      console.log('exportService: createRestore success response:', response)
      return response
    }).catch(error => {
      console.error('exportService: createRestore error:', error)
      throw error
    })
  },

  getRestoreOperation(exportId: string, restoreId: string) {
    return axios.get(`${API_URL}/${exportId}/restores/${restoreId}`, {
      headers: {
        'Accept': 'application/vnd.api+json'
      }
    })
  },

  // Poll restore status until completion or failure
  async pollRestoreStatus(
    exportId: string,
    restoreId: string,
    // eslint-disable-next-line
    onUpdate?: (restore: any) => void,
    intervalMs: number = 2000,
    maxAttempts: number = 300 // 10 minutes with 2s intervals
  ): Promise<any> {
    let attempts = 0;

    return new Promise((resolve, reject) => {
      const poll = async () => {
        try {
          attempts++;
          const response = await this.getRestoreOperation(exportId, restoreId);
          const restore = response.data.data.attributes;

          // Call update callback if provided
          if (onUpdate) {
            onUpdate(restore);
          }

          // Check if restore is complete
          if (restore.status === 'completed' || restore.status === 'failed') {
            resolve(restore);
            return;
          }

          // Check if we've exceeded max attempts
          if (attempts >= maxAttempts) {
            reject(new Error('Restore polling timeout'));
            return;
          }

          // Schedule next poll
          setTimeout(poll, intervalMs);
        } catch (error) {
          reject(error);
        }
      };

      // Start polling
      poll();
    });
  }
}

export default exportService
