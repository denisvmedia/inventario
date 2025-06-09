import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import AreaDetailView from '@/views/areas/AreaDetailView.vue'
import areaService from '@/services/areaService'
import locationService from '@/services/locationService'
import commodityService from '@/services/commodityService'
import valueService from '@/services/valueService'

// Mock the services
vi.mock('@/services/areaService')
vi.mock('@/services/locationService')
vi.mock('@/services/commodityService')
vi.mock('@/services/valueService')

// Mock font-awesome-icon component
vi.mock('@fortawesome/vue-fontawesome', () => ({
  FontAwesomeIcon: {
    name: 'FontAwesomeIcon',
    template: '<span class="fa-icon"></span>'
  }
}))

// Mock router
vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn()
  }),
  useRoute: () => ({
    params: { id: 'area-1' },
    query: {}
  })
}))

// Mock components
vi.mock('@/components/Confirmation.vue', () => ({
  default: {
    name: 'Confirmation',
    template: '<div class="confirmation-mock"></div>'
  }
}))

vi.mock('@/components/CommodityListItem.vue', () => ({
  default: {
    name: 'CommodityListItem',
    props: ['commodity'],
    template: '<div class="commodity-item">{{ commodity.attributes.name }}</div>'
  }
}))

vi.mock('@/components/ToggleSwitch.vue', () => ({
  default: {
    name: 'ToggleSwitch',
    template: '<input type="checkbox" class="toggle-switch" />'
  }
}))

describe('AreaDetailView.vue', () => {
  const mockArea = {
    id: 'area-1',
    attributes: {
      name: 'Test Area',
      location_id: 'location-1'
    }
  }

  const mockLocation = {
    id: 'location-1',
    attributes: {
      name: 'Test Location',
      address: '123 Test St'
    }
  }

  const mockCommodities = [
    {
      id: 'commodity-1',
      attributes: {
        name: 'Oldest Item',
        area_id: 'area-1',
        purchase_date: '2023-01-01',
        status: 'in_use',
        draft: false
      }
    },
    {
      id: 'commodity-2',
      attributes: {
        name: 'Middle Item',
        area_id: 'area-1',
        purchase_date: '2023-06-15',
        status: 'in_use',
        draft: false
      }
    },
    {
      id: 'commodity-3',
      attributes: {
        name: 'Newest Item',
        area_id: 'area-1',
        purchase_date: '2023-12-31',
        status: 'in_use',
        draft: false
      }
    },
    {
      id: 'commodity-4',
      attributes: {
        name: 'No Date Item',
        area_id: 'area-1',
        purchase_date: null,
        status: 'in_use',
        draft: false
      }
    }
  ]

  beforeEach(() => {
    vi.resetAllMocks()

    // Mock service responses
    vi.mocked(areaService.getArea).mockResolvedValue({
      data: { data: mockArea }
    })

    vi.mocked(locationService.getLocations).mockResolvedValue({
      data: { data: [mockLocation] }
    })

    vi.mocked(commodityService.getCommodities).mockResolvedValue({
      data: { data: mockCommodities }
    })

    vi.mocked(valueService.getValues).mockResolvedValue({
      data: { data: { attributes: { area_totals: [] } } }
    })
  })

  describe('Commodity Sorting', () => {
    it('sorts commodities by purchase date in descending order (newest first)', async () => {
      const wrapper = mount(AreaDetailView)

      // Wait for the component to load data
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0)) // Allow promises to resolve

      // Get the filtered commodities from the component
      const filteredCommodities = wrapper.vm.filteredCommodities

      // Verify the order: newest first, then by date descending, items without dates last
      expect(filteredCommodities).toHaveLength(4)
      expect(filteredCommodities[0].attributes.name).toBe('Newest Item') // 2023-12-31
      expect(filteredCommodities[1].attributes.name).toBe('Middle Item') // 2023-06-15
      expect(filteredCommodities[2].attributes.name).toBe('Oldest Item') // 2023-01-01
      expect(filteredCommodities[3].attributes.name).toBe('No Date Item') // null date (sorted to end)
    })

    it('handles commodities with null/undefined purchase dates by placing them at the end', async () => {
      const commoditiesWithNullDates = [
        {
          id: 'commodity-1',
          attributes: {
            name: 'Recent Item',
            area_id: 'area-1',
            purchase_date: '2023-12-01',
            status: 'in_use',
            draft: false
          }
        },
        {
          id: 'commodity-2',
          attributes: {
            name: 'No Date Item 1',
            area_id: 'area-1',
            purchase_date: null,
            status: 'in_use',
            draft: false
          }
        },
        {
          id: 'commodity-3',
          attributes: {
            name: 'No Date Item 2',
            area_id: 'area-1',
            purchase_date: undefined,
            status: 'in_use',
            draft: false
          }
        }
      ]

      vi.mocked(commodityService.getCommodities).mockResolvedValue({
        data: { data: commoditiesWithNullDates }
      })

      const wrapper = mount(AreaDetailView)

      // Wait for the component to load data
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0))

      const filteredCommodities = wrapper.vm.filteredCommodities

      expect(filteredCommodities).toHaveLength(3)
      expect(filteredCommodities[0].attributes.name).toBe('Recent Item')
      expect(filteredCommodities[1].attributes.name).toBe('No Date Item 1')
      expect(filteredCommodities[2].attributes.name).toBe('No Date Item 2')
    })

    it('maintains sort order when filtering is applied', async () => {
      const commoditiesWithDrafts = [
        ...mockCommodities,
        {
          id: 'commodity-5',
          attributes: {
            name: 'Draft Item',
            area_id: 'area-1',
            purchase_date: '2023-11-01',
            status: 'in_use',
            draft: true
          }
        }
      ]

      vi.mocked(commodityService.getCommodities).mockResolvedValue({
        data: { data: commoditiesWithDrafts }
      })

      const wrapper = mount(AreaDetailView)

      // Wait for the component to load data
      await wrapper.vm.$nextTick()
      await new Promise(resolve => setTimeout(resolve, 0))

      // With showInactiveItems = false (default), drafts should be filtered out
      let filteredCommodities = wrapper.vm.filteredCommodities
      expect(filteredCommodities).toHaveLength(4) // Excludes draft item
      expect(filteredCommodities[0].attributes.name).toBe('Newest Item')

      // Set showInactiveItems to true to include drafts
      wrapper.vm.showInactiveItems = true
      await wrapper.vm.$nextTick()

      filteredCommodities = wrapper.vm.filteredCommodities
      expect(filteredCommodities).toHaveLength(5) // Includes draft item
      expect(filteredCommodities[0].attributes.name).toBe('Newest Item') // 2023-12-31
      expect(filteredCommodities[1].attributes.name).toBe('Draft Item') // 2023-11-01
      expect(filteredCommodities[2].attributes.name).toBe('Middle Item') // 2023-06-15
    })
  })
})