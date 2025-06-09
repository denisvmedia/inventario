import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import CommodityListView from '../commodities/CommodityListView.vue'
import { COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'

// Mock services
vi.mock('@/services/commodityService', () => ({
  default: {
    getCommodities: vi.fn(() => Promise.resolve({ data: { data: [] } }))
  }
}))

vi.mock('@/services/areaService', () => ({
  default: {
    getAreas: vi.fn(() => Promise.resolve({ data: { data: [] } }))
  }
}))

vi.mock('@/services/locationService', () => ({
  default: {
    getLocations: vi.fn(() => Promise.resolve({ data: { data: [] } }))
  }
}))

vi.mock('@/services/valueService', () => ({
  default: {
    getValues: vi.fn(() => Promise.resolve({ data: { data: { attributes: { global_total: '0' } } } }))
  }
}))

vi.mock('@/services/currencyService', () => ({
  formatPrice: vi.fn((price, currency) => `${price.toFixed(2)} ${currency || 'USD'}`)
}))

// Mock stores
vi.mock('@/stores/settingsStore', () => ({
  useSettingsStore: vi.fn(() => ({
    mainCurrency: 'USD',
    fetchMainCurrency: vi.fn()
  }))
}))

// Mock router
vi.mock('vue-router', () => ({
  useRouter: vi.fn(() => ({
    push: vi.fn()
  })),
  useRoute: vi.fn(() => ({
    query: {}
  }))
}))

// Mock components
vi.mock('@/components/CommodityListItem.vue', () => ({
  default: {
    name: 'CommodityListItem',
    props: ['commodity'],
    template: '<div class="mock-commodity-item">{{ commodity.attributes.name }}</div>'
  }
}))

vi.mock('@/components/Confirmation.vue', () => ({
  default: {
    name: 'Confirmation',
    template: '<div class="mock-confirmation"></div>'
  }
}))

describe('CommodityListView.vue', () => {
  describe('Commodity Sorting', () => {
    it('sorts commodities by purchase date in descending order', async () => {
      const wrapper = mount(CommodityListView, {
        global: {
          stubs: {
            CommodityListItem: true,
            Confirmation: true,
            'router-link': true,
            'font-awesome-icon': true,
            ToggleSwitch: true
          }
        }
      })

      // Wait for initial mount to complete
      await nextTick()

      // Get the component instance to access computed properties and methods
      const vm = wrapper.vm as any

      // Set up test commodities with different purchase dates
      vm.commodities = [
        {
          id: 'commodity-1',
          attributes: {
            name: 'Oldest Commodity',
            purchase_date: '2022-01-01',
            status: COMMODITY_STATUS_IN_USE,
            draft: false
          }
        },
        {
          id: 'commodity-2',
          attributes: {
            name: 'Newest Commodity',
            purchase_date: '2024-01-01',
            status: COMMODITY_STATUS_IN_USE,
            draft: false
          }
        },
        {
          id: 'commodity-3',
          attributes: {
            name: 'Middle Commodity',
            purchase_date: '2023-01-01',
            status: COMMODITY_STATUS_IN_USE,
            draft: false
          }
        },
        {
          id: 'commodity-4',
          attributes: {
            name: 'No Date Commodity',
            purchase_date: null,
            status: COMMODITY_STATUS_IN_USE,
            draft: false
          }
        }
      ]

      await nextTick()

      // Get the filtered (and sorted) commodities
      const filtered = vm.filteredCommodities

      // Check that commodities are sorted by purchase date descending
      expect(filtered).toHaveLength(4)
      expect(filtered[0].attributes.name).toBe('Newest Commodity') // 2024-01-01
      expect(filtered[1].attributes.name).toBe('Middle Commodity') // 2023-01-01
      expect(filtered[2].attributes.name).toBe('Oldest Commodity') // 2022-01-01
      expect(filtered[3].attributes.name).toBe('No Date Commodity') // null date (should be last)
    })

    it('handles commodities with null or undefined purchase dates', async () => {
      const wrapper = mount(CommodityListView, {
        global: {
          stubs: {
            CommodityListItem: true,
            Confirmation: true,
            'router-link': true,
            'font-awesome-icon': true,
            ToggleSwitch: true
          }
        }
      })

      await nextTick()

      const vm = wrapper.vm as any

      vm.commodities = [
        {
          id: 'commodity-1',
          attributes: {
            name: 'With Date',
            purchase_date: '2023-01-01',
            status: COMMODITY_STATUS_IN_USE,
            draft: false
          }
        },
        {
          id: 'commodity-2',
          attributes: {
            name: 'Null Date',
            purchase_date: null,
            status: COMMODITY_STATUS_IN_USE,
            draft: false
          }
        },
        {
          id: 'commodity-3',
          attributes: {
            name: 'Undefined Date',
            status: COMMODITY_STATUS_IN_USE,
            draft: false
          }
        }
      ]

      await nextTick()

      const filtered = vm.filteredCommodities

      expect(filtered).toHaveLength(3)
      expect(filtered[0].attributes.name).toBe('With Date') // Should be first (has actual date)
      // The null and undefined date commodities should be last (order between them doesn't matter much)
      expect([filtered[1].attributes.name, filtered[2].attributes.name]).toEqual(
        expect.arrayContaining(['Null Date', 'Undefined Date'])
      )
    })

    it('maintains sorting when filtering is applied', async () => {
      const wrapper = mount(CommodityListView, {
        global: {
          stubs: {
            CommodityListItem: true,
            Confirmation: true,
            'router-link': true,
            'font-awesome-icon': true,
            ToggleSwitch: true
          }
        }
      })

      await nextTick()

      const vm = wrapper.vm as any

      vm.commodities = [
        {
          id: 'commodity-1',
          attributes: {
            name: 'Draft Commodity',
            purchase_date: '2024-01-01',
            status: COMMODITY_STATUS_IN_USE,
            draft: true // Should be filtered out when showInactiveItems is false
          }
        },
        {
          id: 'commodity-2',
          attributes: {
            name: 'Active New',
            purchase_date: '2023-06-01',
            status: COMMODITY_STATUS_IN_USE,
            draft: false
          }
        },
        {
          id: 'commodity-3',
          attributes: {
            name: 'Active Old',
            purchase_date: '2023-01-01',
            status: COMMODITY_STATUS_IN_USE,
            draft: false
          }
        }
      ]

      // Test with filtering enabled (showInactiveItems = false, which is default)
      await nextTick()

      let filtered = vm.filteredCommodities

      expect(filtered).toHaveLength(2) // Draft should be filtered out
      expect(filtered[0].attributes.name).toBe('Active New') // More recent date first
      expect(filtered[1].attributes.name).toBe('Active Old') // Older date second

      // Test with filtering disabled
      vm.showInactiveItems = true
      await nextTick()

      filtered = vm.filteredCommodities

      expect(filtered).toHaveLength(3) // All items included
      expect(filtered[0].attributes.name).toBe('Draft Commodity') // Most recent date first
      expect(filtered[1].attributes.name).toBe('Active New') // Next most recent
      expect(filtered[2].attributes.name).toBe('Active Old') // Oldest
    })
  })
})
