import { beforeEach, describe, expect, it, vi } from 'vitest'
import settingsService from '../settingsService'
import api from '../api'

vi.mock('../api', () => ({
  default: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    patch: vi.fn(),
    delete: vi.fn()
  }
}))

const mockedApi = vi.mocked(api)

describe('settingsService.updateMainCurrency', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('sends the legacy raw currency payload when no exchange rate is provided', async () => {
    mockedApi.patch.mockResolvedValue({ data: {} })

    await settingsService.updateMainCurrency('EUR')

    expect(mockedApi.patch).toHaveBeenCalledWith('/api/v1/settings/system.main_currency', 'EUR', {
      headers: {
        'Content-Type': 'application/json'
      }
    })
  })

  it('sends the exchange-rate envelope when a custom rate is provided', async () => {
    mockedApi.patch.mockResolvedValue({ data: {} })

    await settingsService.updateMainCurrency('EUR', '0.95')

    expect(mockedApi.patch).toHaveBeenCalledWith('/api/v1/settings/system.main_currency', {
      value: 'EUR',
      exchange_rate: 0.95
    }, {
      headers: {
        'Content-Type': 'application/json'
      }
    })
  })
})