import axios from 'axios'

const API_URL = '/api/v1/system'

export interface SystemInfo {
  // Version information
  version: string
  commit: string
  build_date: string
  go_version: string
  platform: string

  // System information
  database_backend: string
  file_storage_backend: string
  operating_system: string

  // Runtime metrics
  uptime: string
  memory_usage: string
  num_goroutines: number
  num_cpu: number

  // Settings information
  settings: {
    MainCurrency?: string
    Theme?: string
    ShowDebugInfo?: boolean
    DefaultDateFormat?: string
  }
}

const systemService = {
  getSystemInfo(): Promise<{ data: SystemInfo }> {
    return axios.get(API_URL, {
      headers: {
        'Accept': 'application/json'
      }
    })
  }
}

export default systemService
