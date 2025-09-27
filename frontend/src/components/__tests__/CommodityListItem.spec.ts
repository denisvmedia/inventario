import { describe, it, expect, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import CommodityListItem from '../CommodityListItem.vue'
import { COMMODITY_TYPES } from '@/constants/commodityTypes'
import { COMMODITY_STATUSES, COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'

// Mock the currency service
vi.mock('@/services/currencyService.ts', () => ({
  calculatePricePerUnit: vi.fn((commodity) => {
    const price = 100
    const count = commodity.attributes.count || 1
    return price / count
  }),
  formatPrice: vi.fn((price) => `${price.toFixed(2)} USD`),
  getDisplayPrice: vi.fn(() => 100)
}))

// Mock FontAwesomeIcon component
vi.mock('@fortawesome/vue-fontawesome', () => ({
  FontAwesomeIcon: {
    name: 'FontAwesomeIcon',
    props: ['icon'],
    template: '<span class="mock-icon">{{ icon }}</span>'
  }
}))

describe('CommodityListItem.vue', () => {
  // Test data
  const mockCommodity = {
    id: 'commodity-1',
    type: 'commodities',
    attributes: {
      name: 'Test Commodity',
      type: 'electronics',
      count: 2,
      status: COMMODITY_STATUS_IN_USE,
      area_id: 'area-1',
      draft: false
    }
  }

  const mockAreaMap = {
    'area-1': {
      name: 'Test Area',
      locationId: 'location-1'
    }
  }

  const mockLocationMap = {
    'location-1': {
      name: 'Test Location'
    }
  }

  const defaultProps = {
    commodity: mockCommodity,
    areaMap: mockAreaMap,
    locationMap: mockLocationMap,
    showLocation: true,
    highlightCommodityId: ''
  }

  // Helper function to create a wrapper with custom props
  const createWrapper = (props = {}) => {
    return mount(CommodityListItem, {
      props: { ...defaultProps, ...props },
      global: {
        stubs: {
          FontAwesomeIcon: true
        }
      }
    })
  }

  // Rendering tests
  describe('Rendering', () => {
    it('renders the commodity name', () => {
      const wrapper = createWrapper()
      expect(wrapper.find('h3').text()).toBe('Test Commodity')
    })

    it('renders the commodity type', () => {
      const wrapper = createWrapper()
      const typeName = COMMODITY_TYPES.find(t => t.id === 'electronics')?.name
      expect(wrapper.find('.type').text()).toContain(typeName)
    })

    it('renders the count when greater than 1', () => {
      const wrapper = createWrapper()
      expect(wrapper.find('.count').exists()).toBe(true)
      expect(wrapper.find('.count').text()).toBe('×2')
    })

    it('does not render the count when equal to 1', () => {
      const commodity = {
        ...mockCommodity,
        attributes: {
          ...mockCommodity.attributes,
          count: 1
        }
      }
      const wrapper = createWrapper({ commodity })
      expect(wrapper.find('.count').exists()).toBe(false)
    })

    it('renders the price', () => {
      const wrapper = createWrapper()
      expect(wrapper.find('.price').exists()).toBe(true)
    })

    it('renders the price per unit when count is greater than 1', () => {
      const wrapper = createWrapper()
      expect(wrapper.find('.price-per-unit').exists()).toBe(true)
      expect(wrapper.find('.price-per-unit').text()).toContain('per unit')
    })

    it('renders the location info when showLocation is true', () => {
      const wrapper = createWrapper({ showLocation: true })
      expect(wrapper.find('.commodity-location').exists()).toBe(true)
      expect(wrapper.find('.location-info').text()).toContain('Test Location / Test Area')
    })

    it('does not render the location info when showLocation is false', () => {
      const wrapper = createWrapper({ showLocation: false })
      expect(wrapper.find('.commodity-location').exists()).toBe(false)
    })

    it('renders the status', () => {
      const wrapper = createWrapper()
      const statusName = COMMODITY_STATUSES.find(s => s.id === COMMODITY_STATUS_IN_USE)?.name
      expect(wrapper.find('.status').text()).toBe(statusName)
    })

    it('renders the edit and delete buttons', () => {
      const wrapper = createWrapper()
      expect(wrapper.find('.commodity-actions').exists()).toBe(true)
      expect(wrapper.findAll('button').length).toBe(2)
    })
  })

  // Class tests
  describe('CSS Classes', () => {
    it('adds highlighted class when commodity id matches highlightCommodityId', () => {
      const wrapper = createWrapper({ highlightCommodityId: 'commodity-1' })
      expect(wrapper.classes()).toContain('highlighted')
    })

    it('adds draft class when commodity is a draft', () => {
      const commodity = {
        ...mockCommodity,
        attributes: {
          ...mockCommodity.attributes,
          draft: true
        }
      }
      const wrapper = createWrapper({ commodity })
      expect(wrapper.classes()).toContain('draft')
    })

    it('adds sold class when commodity status is sold', () => {
      const commodity = {
        ...mockCommodity,
        attributes: {
          ...mockCommodity.attributes,
          status: 'sold'
        }
      }
      const wrapper = createWrapper({ commodity })
      expect(wrapper.classes()).toContain('sold')
    })

    it('adds lost class when commodity status is lost', () => {
      const commodity = {
        ...mockCommodity,
        attributes: {
          ...mockCommodity.attributes,
          status: 'lost'
        }
      }
      const wrapper = createWrapper({ commodity })
      expect(wrapper.classes()).toContain('lost')
    })

    it('adds disposed class when commodity status is disposed', () => {
      const commodity = {
        ...mockCommodity,
        attributes: {
          ...mockCommodity.attributes,
          status: 'disposed'
        }
      }
      const wrapper = createWrapper({ commodity })
      expect(wrapper.classes()).toContain('disposed')
    })

    it('adds written-off class when commodity status is written_off', () => {
      const commodity = {
        ...mockCommodity,
        attributes: {
          ...mockCommodity.attributes,
          status: 'written_off'
        }
      }
      const wrapper = createWrapper({ commodity })
      expect(wrapper.classes()).toContain('written-off')
    })

    it('adds with-draft class to status when commodity is a draft', () => {
      const commodity = {
        ...mockCommodity,
        attributes: {
          ...mockCommodity.attributes,
          draft: true
        }
      }
      const wrapper = createWrapper({ commodity })
      expect(wrapper.find('.commodity-status').classes()).toContain('with-draft')
    })
  })

  // Event tests
  describe('Events', () => {
    it('emits view-commodity event when clicking on the commodity card', async () => {
      const wrapper = createWrapper()
      await wrapper.find('.commodity-card').trigger('click')

      expect(wrapper.emitted('view-commodity')).toBeTruthy()
      expect(wrapper.emitted('view-commodity')![0]).toEqual(['commodity-1'])
    })

    it('emits edit-commodity event when clicking on the edit button', async () => {
      const wrapper = createWrapper()
      await wrapper.findAll('button')[0].trigger('click')

      expect(wrapper.emitted('edit-commodity')).toBeTruthy()
      expect(wrapper.emitted('edit-commodity')![0]).toEqual(['commodity-1'])
    })

    it('emits confirm-delete-commodity event when clicking on the delete button', async () => {
      const wrapper = createWrapper()
      await wrapper.findAll('button')[1].trigger('click')

      expect(wrapper.emitted('confirm-delete-commodity')).toBeTruthy()
      expect(wrapper.emitted('confirm-delete-commodity')![0]).toEqual(['commodity-1'])
    })

    it('stops event propagation when clicking on action buttons', async () => {
      const wrapper = createWrapper()

      // Mock the stopPropagation method
      const stopPropagation = vi.fn()

      // Trigger click with mocked event
      await wrapper.findAll('button')[0].trigger('click.stop', {
        stopPropagation
      })

      // Check that stopPropagation was called
      expect(stopPropagation).toHaveBeenCalled()
    })
  })

  // Helper function tests
  describe('Helper Functions', () => {
    it('returns correct type icon based on commodity type', () => {
      const wrapper = createWrapper()

      // Access the component instance
      const vm = wrapper.vm as { getTypeIcon: (type: string) => string }

      expect(vm.getTypeIcon('white_goods')).toBe('blender')
      expect(vm.getTypeIcon('electronics')).toBe('laptop')
      expect(vm.getTypeIcon('equipment')).toBe('tools')
      expect(vm.getTypeIcon('furniture')).toBe('couch')
      expect(vm.getTypeIcon('clothes')).toBe('tshirt')
      expect(vm.getTypeIcon('other')).toBe('box')
      expect(vm.getTypeIcon('unknown')).toBe('box') // Default case
    })

    it('returns correct type name based on commodity type', () => {
      const wrapper = createWrapper()

      // Access the component instance
      const vm = wrapper.vm as { getTypeName: (type: string) => string }

      expect(vm.getTypeName('electronics')).toBe('Electronics')
      expect(vm.getTypeName('unknown')).toBe('unknown') // Fallback to type ID
    })

    it('returns correct status name based on commodity status', () => {
      const wrapper = createWrapper()

      // Access the component instance
      const vm = wrapper.vm as { getStatusName: (status: string) => string }

      expect(vm.getStatusName('in_use')).toBe('In Use')
      expect(vm.getStatusName('unknown')).toBe('unknown') // Fallback to status ID
    })

    it('returns correct area name based on area ID', () => {
      const wrapper = createWrapper()

      // Access the component instance
      const vm = wrapper.vm as { getAreaName: (areaId: string) => string }

      expect(vm.getAreaName('area-1')).toBe('Test Area')
      expect(vm.getAreaName('unknown')).toBe('Unknown Area') // Fallback
    })

    it('returns correct location name based on area ID', () => {
      const wrapper = createWrapper()

      // Access the component instance
      const vm = wrapper.vm as { getLocationName: (areaId: string) => string }

      expect(vm.getLocationName('area-1')).toBe('Test Location')
      expect(vm.getLocationName('unknown')).toBe('Unknown Location') // Fallback
    })
  })

  // Edge cases
  describe('Edge Cases', () => {
    it('handles missing area_id', () => {
      const commodity = {
        ...mockCommodity,
        attributes: {
          ...mockCommodity.attributes,
          area_id: undefined
        }
      }
      const wrapper = createWrapper({ commodity })

      // Should not render location info even if showLocation is true
      expect(wrapper.find('.commodity-location').exists()).toBe(false)
    })

    it('handles missing area in areaMap', () => {
      const wrapper = createWrapper({
        commodity: {
          ...mockCommodity,
          attributes: {
            ...mockCommodity.attributes,
            area_id: 'non-existent-area'
          }
        }
      })

      // Access the component instance
      const vm = wrapper.vm as { getAreaName: (areaId: string) => string, getLocationName: (areaId: string) => string }

      expect(vm.getAreaName('non-existent-area')).toBe('Unknown Area')
      expect(vm.getLocationName('non-existent-area')).toBe('Unknown Location')
    })

    it('handles missing location in locationMap', () => {
      const areaMap = {
        'area-1': {
          name: 'Test Area',
          locationId: 'non-existent-location'
        }
      }

      const wrapper = createWrapper({ areaMap })

      // Access the component instance
      const vm = wrapper.vm as { getLocationName: (areaId: string) => string }

      expect(vm.getLocationName('area-1')).toBe('Unknown Location')
    })
  })

  // Purchase Date Tests
  describe('Purchase Date Display', () => {
    it('displays purchase date when available', () => {
      const commodity = {
        ...mockCommodity,
        attributes: {
          ...mockCommodity.attributes,
          purchase_date: '2023-01-15'
        }
      }
      const wrapper = createWrapper({ commodity })

      const purchaseDateElement = wrapper.find('.commodity-purchase-date')
      expect(purchaseDateElement.exists()).toBe(true)
      expect(purchaseDateElement.text()).toContain('Jan 15, 2023')
    })

    it('does not display purchase date when not available', () => {
      const commodity = {
        ...mockCommodity,
        attributes: {
          ...mockCommodity.attributes,
          purchase_date: undefined
        }
      }
      const wrapper = createWrapper({ commodity })

      const purchaseDateElement = wrapper.find('.commodity-purchase-date')
      expect(purchaseDateElement.exists()).toBe(false)
    })

    it('formats purchase date correctly', () => {
      const wrapper = createWrapper()
      const vm = wrapper.vm as { formatPurchaseDate: (date: string) => string }

      // Test different date formats
      expect(vm.formatPurchaseDate('2023-01-01')).toBe('Jan 1, 2023')
      expect(vm.formatPurchaseDate('2023-12-25')).toBe('Dec 25, 2023')
      expect(vm.formatPurchaseDate('2024-06-15')).toBe('Jun 15, 2024')
    })
  })
})
