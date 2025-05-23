import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest'
import { mount } from '@vue/test-utils'
import CommodityForm from '../CommodityForm.vue'
import { COMMODITY_STATUS_IN_USE } from '@/constants/commodityStatuses'
import { CURRENCY_CZK } from '@/constants/currencies'

// Mock PrimeVue components
vi.mock('primevue/select', () => ({
  default: {
    name: 'Select',
    render() { return null },
    props: ['modelValue', 'options', 'optionLabel', 'optionValue', 'placeholder', 'disabled'],
    emits: ['update:modelValue']
  }
}))

describe('CommodityForm.vue', () => {
  // Test data
  const mockAreas = [
    {
      label: 'Location 1',
      items: [
        { id: 'area1', attributes: { name: 'Area 1' } },
        { id: 'area2', attributes: { name: 'Area 2' } }
      ]
    },
    {
      label: 'Location 2',
      items: [
        { id: 'area3', attributes: { name: 'Area 3' } }
      ]
    }
  ]

  const mockCurrencies = [
    { code: 'USD', label: 'US Dollar' },
    { code: 'EUR', label: 'Euro' },
    { code: 'CZK', label: 'Czech Koruna' }
  ]

  const defaultProps = {
    areas: mockAreas,
    currencies: mockCurrencies,
    mainCurrency: CURRENCY_CZK,
    isSubmitting: false,
    submitButtonText: 'Save',
    submitButtonLoadingText: 'Saving...'
  }

  // Helper function to create a wrapper with custom props
  const createWrapper = (props = {}) => {
    return mount(CommodityForm, {
      props: { ...defaultProps, ...props },
      global: {
        stubs: {
          Select: true
        }
      },
      attachTo: document.body
    })
  }

  beforeEach(() => {
    vi.clearAllMocks()
    document.body.innerHTML = ''
    // Use a fixed date for testing
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2023-01-01'))

    // Mock scrollIntoView which is not available in the test environment
    Element.prototype.scrollIntoView = vi.fn()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  // Basic rendering tests
  describe('Rendering', () => {
    it('renders the form with required fields', () => {
      const wrapper = createWrapper()

      // Check for basic form elements
      expect(wrapper.find('form').exists()).toBe(true)
      expect(wrapper.find('input[id="name"]').exists()).toBe(true)
      expect(wrapper.find('input[id="shortName"]').exists()).toBe(true)
      expect(wrapper.find('input[id="count"]').exists()).toBe(true)
      expect(wrapper.find('input[id="originalPrice"]').exists()).toBe(true)
      expect(wrapper.find('input[id="currentPrice"]').exists()).toBe(true)
      expect(wrapper.find('input[id="serialNumber"]').exists()).toBe(true)
      expect(wrapper.find('input[id="purchaseDate"]').exists()).toBe(true)
      expect(wrapper.find('textarea[id="comments"]').exists()).toBe(true)

      // Check for section headings
      const headings = wrapper.findAll('h2')
      expect(headings.length).toBe(5) // 5 sections in the form
      expect(headings[0].text()).toBe('Basic Information')
      expect(headings[1].text()).toBe('Price Information')

      // Check for submit button
      const submitButton = wrapper.find('button[type="submit"]')
      expect(submitButton.exists()).toBe(true)
      expect(submitButton.text()).toBe('Save')
    })

    it('disables the area select when areaFromUrl is provided', () => {
      const wrapper = createWrapper({
        areaFromUrl: 'area1'
      })

      const areaSelect = wrapper.find('#areaId')
      expect(areaSelect.attributes('disabled')).toBeDefined()
    })

    it('shows loading state during form submission', async () => {
      const wrapper = createWrapper({
        isSubmitting: true
      })

      const submitButton = wrapper.find('button[type="submit"]')
      expect(submitButton.text()).toBe('Saving...')
      expect(submitButton.attributes('disabled')).toBeDefined()
    })
  })

  // Initialization tests
  describe('Initialization', () => {
    it('initializes with default values when no initialData is provided', () => {
      const wrapper = createWrapper()

      // Check default values
      expect(wrapper.vm.formData.name).toBe('')
      expect(wrapper.vm.formData.count).toBe(1)
      expect(wrapper.vm.formData.originalPriceCurrency).toBe(CURRENCY_CZK)
      expect(wrapper.vm.formData.status).toBe(COMMODITY_STATUS_IN_USE)
      expect(wrapper.vm.formData.draft).toBe(false)
    })

    it('initializes with provided initialData', () => {
      const initialData = {
        name: 'Test Commodity',
        shortName: 'TC',
        type: 'electronics',
        areaId: 'area1',
        count: 2,
        originalPrice: 100,
        originalPriceCurrency: 'USD',
        currentPrice: 120,
        status: 'sold',
        draft: true
      }

      const wrapper = createWrapper({
        initialData
      })

      // Check that initialData values are used
      expect(wrapper.vm.formData.name).toBe('Test Commodity')
      expect(wrapper.vm.formData.shortName).toBe('TC')
      expect(wrapper.vm.formData.count).toBe(2)
      expect(wrapper.vm.formData.originalPrice).toBe(100)
      expect(wrapper.vm.formData.originalPriceCurrency).toBe('USD')
      expect(wrapper.vm.formData.draft).toBe(true)
    })

    it('sets areaId from areaFromUrl when provided', () => {
      const wrapper = createWrapper({
        areaFromUrl: 'area2'
      })

      expect(wrapper.vm.formData.areaId).toBe('area2')
    })
  })

  // Validation tests
  describe('Validation', () => {
    it('validates name field is required', async () => {
      const wrapper = createWrapper()

      // Set the name field to empty
      const nameInput = wrapper.find('input[id="name"]')
      await nameInput.setValue('')

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check validation error
      expect(wrapper.vm.formErrors.name).toBe('Name is required')
    })

    it('validates count must be at least 1', async () => {
      const wrapper = createWrapper()

      // Set count to 0
      const countInput = wrapper.find('input[id="count"]')
      await countInput.setValue(0)

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check validation error
      expect(wrapper.vm.formErrors.count).toBe('Count must be at least 1')
    })

    it('validates prices cannot be negative', async () => {
      const wrapper = createWrapper()

      // Set negative price
      const priceInput = wrapper.find('input[id="originalPrice"]')
      await priceInput.setValue(-10)

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check validation error
      expect(wrapper.vm.formErrors.originalPrice).toBe('Original Price cannot be negative')
    })

    it('validates comments length', async () => {
      const wrapper = createWrapper()

      // Set comments that exceed the maximum length
      const longComments = 'a'.repeat(1001)
      const commentsInput = wrapper.find('textarea[id="comments"]')
      await commentsInput.setValue(longComments)

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check validation error
      expect(wrapper.vm.formErrors.comments).toBe('Comments cannot exceed 1000 characters')
    })
  })

  // Event tests
  describe('Events', () => {
    it('has a cancel functionality', () => {
      const wrapper = createWrapper()

      // Find a button that can cancel the form
      const buttons = wrapper.findAll('button')
      const cancelButton = buttons.find(button => button.text() === 'Cancel' || button.classes('btn-secondary'))
      expect(cancelButton).toBeDefined()
    })

    it('emits validate event when form is submitted', async () => {
      const wrapper = createWrapper()

      // Fill required fields
      const nameInput = wrapper.find('input[id="name"]')
      await nameInput.setValue('Test Commodity')

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check emitted event
      const validateEvents = wrapper.emitted('validate')
      expect(validateEvents).toBeTruthy()
    })
  })

  // Methods tests
  describe('Methods', () => {
    it('has a setErrors method that sets form errors', () => {
      const wrapper = createWrapper()

      // Call setErrors method
      wrapper.vm.setErrors({
        name: 'Name error',
        short_name: 'Short name error'
      })

      // Check errors are set
      expect(wrapper.vm.formErrors.name).toBe('Name error')
      expect(wrapper.vm.formErrors.shortName).toBe('Short name error')
    })
  })

  // Props tests
  describe('Props', () => {
    it('uses the provided submitButtonText', () => {
      const wrapper = createWrapper({
        submitButtonText: 'Custom Submit Text'
      })

      const submitButton = wrapper.find('button[type="submit"]')
      expect(submitButton.text()).toBe('Custom Submit Text')
    })

    it('uses the provided submitButtonLoadingText when isSubmitting is true', () => {
      const wrapper = createWrapper({
        isSubmitting: true,
        submitButtonLoadingText: 'Custom Loading Text'
      })

      const submitButton = wrapper.find('button[type="submit"]')
      expect(submitButton.text()).toBe('Custom Loading Text')
    })

    it('accepts initialData prop', () => {
      const initialData = {
        name: 'Initial Commodity',
        shortName: 'IC'
      }

      const wrapper = createWrapper({
        initialData
      })

      // formData should be initialized with initialData
      expect(wrapper.vm.formData.name).toBe('Initial Commodity')
      expect(wrapper.vm.formData.shortName).toBe('IC')
    })
  })
})
