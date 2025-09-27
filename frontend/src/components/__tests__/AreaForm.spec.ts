import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import AreaForm from '../AreaForm.vue'

// Mock the areaService
vi.mock('../../services/areaService', () => ({
  default: {
    createArea: vi.fn().mockImplementation(() => Promise.resolve())
  }
}))

// Import the mocked service
import areaService from '../../services/areaService'
// eslint-disable-next-line no-unused-vars
type MockedFunction<T> = T & { mockImplementation: (fn: () => unknown) => void }
const mockedCreateArea = areaService.createArea as MockedFunction<typeof areaService.createArea>

describe('AreaForm.vue', () => {
  const locationId = '123'

  beforeEach(() => {
    vi.resetAllMocks()
  })

  // Rendering tests
  describe('Rendering', () => {
    it('renders correctly with provided locationId', () => {
      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      expect(wrapper.find('form').exists()).toBe(true)
      expect(wrapper.find('input[id="name"]').exists()).toBe(true)
      expect(wrapper.find('button[type="submit"]').text()).toContain('Create Area')
    })

    it('shows loading state during form submission', async () => {
      // Mock a delayed response
      mockedCreateArea.mockImplementation(() => new Promise(resolve => setTimeout(resolve, 100)))

      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      // Fill out the form
      await wrapper.find('input[id="name"]').setValue('Test Area')

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check loading state
      expect(wrapper.find('button[type="submit"]').text()).toContain('Creating...')
      expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeDefined()

      // Wait for the promise to resolve
      await flushPromises()
    })
  })

  // Validation tests
  describe('Validation', () => {
    it('validates empty form fields', async () => {
      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      // Submit the form with empty fields
      await wrapper.find('form').trigger('submit')

      // Validation errors should be displayed
      expect(wrapper.find('.error-message').exists()).toBe(true)
      expect(wrapper.find('.error-message').text()).toContain('Name is required')
    })

    it('validates whitespace-only input', async () => {
      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      // Fill the form with whitespace
      await wrapper.find('input[id="name"]').setValue('   ')

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Validation errors should be displayed
      expect(wrapper.find('.error-message').exists()).toBe(true)
      expect(wrapper.find('.error-message').text()).toContain('Name is required')
    })

    it('clears validation errors when input becomes valid', async () => {
      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      // Submit with empty field to trigger validation
      await wrapper.find('form').trigger('submit')
      expect(wrapper.find('.error-message').exists()).toBe(true)

      // Now enter a valid value
      await wrapper.find('input[id="name"]').setValue('Valid Area Name')

      // Submit again
      await wrapper.find('form').trigger('submit')

      // Error should be gone
      expect(wrapper.find('.error-message').exists()).toBe(false)
    })
  })

  // Event tests
  describe('Events', () => {
    it('emits cancel event when cancel button is clicked', async () => {
      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      await wrapper.find('button.btn-secondary').trigger('click')

      const emittedCancel = wrapper.emitted('cancel')
      expect(emittedCancel).toBeTruthy()
      expect(emittedCancel!.length).toBe(1)
    })
  })

  // Form submission tests
  describe('Form Submission', () => {
    it('submits the form successfully', async () => {
      // Mock successful response
      const mockResponse = {
        data: {
          data: {
            id: '456',
            type: 'areas',
            attributes: {
              name: 'Test Area',
              location_id: locationId
            }
          }
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as unknown
      }
      vi.mocked(areaService.createArea).mockResolvedValue(mockResponse)

      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      // Fill out the form
      await wrapper.find('input[id="name"]').setValue('Test Area')

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Wait for async operations to complete
      await flushPromises()

      // Verify service was called with correct data
      expect(areaService.createArea).toHaveBeenCalledTimes(1)
      expect(areaService.createArea).toHaveBeenCalledWith({
        data: {
          type: 'areas',
          attributes: {
            name: 'Test Area',
            location_id: locationId
          }
        }
      })

      // Verify created event was emitted with response data
      const emitted = wrapper.emitted('created')
      expect(emitted).toBeTruthy()
      expect(emitted![0][0]).toEqual(mockResponse.data.data)

      // Form should be reset
      expect((wrapper.find('input[id="name"]').element as HTMLInputElement).value).toBe('')
    })

    it('trims whitespace from input before submission', async () => {
      // Mock successful response
      const mockResponse = {
        data: {
          data: {
            id: '456',
            type: 'areas',
            attributes: {
              name: 'Test Area',
              location_id: locationId
            }
          }
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as unknown
      }
      vi.mocked(areaService.createArea).mockResolvedValue(mockResponse)

      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      // Fill out the form with whitespace
      await wrapper.find('input[id="name"]').setValue('  Test Area  ')

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Wait for async operations to complete
      await flushPromises()

      // Verify service was called with trimmed data
      expect(areaService.createArea).toHaveBeenCalledWith({
        data: {
          type: 'areas',
          attributes: {
            name: 'Test Area', // Whitespace should be trimmed
            location_id: locationId
          }
        }
      })
    })

    it('resets form after successful submission', async () => {
      // Mock successful response
      const mockResponse = {
        data: {
          data: {
            id: '456',
            type: 'areas',
            attributes: {
              name: 'Test Area',
              location_id: locationId
            }
          }
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as unknown
      }
      vi.mocked(areaService.createArea).mockResolvedValue(mockResponse)

      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      // Fill out the form
      await wrapper.find('input[id="name"]').setValue('Test Area')

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Wait for async operations to complete
      await flushPromises()

      // Form should be reset
      expect((wrapper.find('input[id="name"]').element as HTMLInputElement).value).toBe('')
      expect(wrapper.find('.error-message').exists()).toBe(false)
      expect(wrapper.find('.form-error').exists()).toBe(false)
    })
  })

  // Error handling tests
  describe('Error Handling', () => {
    it('handles API validation errors correctly', async () => {
      // Mock API error response with validation errors
      const errorResponse = {
        response: {
          status: 422,
          data: {
            errors: [
              { source: { pointer: '/data/attributes/name' }, detail: 'Name is already taken' }
            ]
          }
        }
      }
      vi.mocked(areaService.createArea).mockRejectedValue(errorResponse)

      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      // Fill out the form
      await wrapper.find('input[id="name"]').setValue('Test Area')

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Wait for async operations to complete
      await flushPromises()

      // Verify error message is displayed
      expect(wrapper.find('.error-message').text()).toContain('Name is already taken')
      expect(wrapper.find('.form-error').exists()).toBe(true)
      expect(wrapper.find('.form-error').text()).toContain('Please correct the errors above.')
    })

    it('handles generic API errors correctly', async () => {
      // Mock generic API error response
      const errorResponse = {
        response: {
          status: 500,
          data: { message: 'Internal server error' }
        }
      }
      vi.mocked(areaService.createArea).mockRejectedValue(errorResponse)

      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      // Fill out the form
      await wrapper.find('input[id="name"]').setValue('Test Area')

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Wait for async operations to complete
      await flushPromises()

      // Verify generic error message is displayed
      expect(wrapper.find('.form-error').exists()).toBe(true)
      expect(wrapper.find('.form-error').text()).toContain('Failed to create area')
    })

    it('handles network errors correctly', async () => {
      // Mock network error
      const networkError = new Error('Network Error')
      vi.mocked(areaService.createArea).mockRejectedValue(networkError)

      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      // Fill out the form
      await wrapper.find('input[id="name"]').setValue('Test Area')

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Wait for async operations to complete
      await flushPromises()

      // Verify network error message is displayed
      expect(wrapper.find('.form-error').exists()).toBe(true)
      expect(wrapper.find('.form-error').text()).toContain('Failed to create area: Network Error')
    })
  })

  // Props tests
  describe('Props', () => {
    it('initializes form with correct locationId', () => {
      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      // Initial locationId should be set
      const vm = wrapper.vm as { form: { locationId: string } }
      expect(vm.form.locationId).toBe(locationId)
    })

    it('uses the locationId prop in the API payload', async () => {
      // Mock successful response
      const mockResponse = {
        data: {
          data: {
            id: '456',
            type: 'areas',
            attributes: {
              name: 'Test Area',
              location_id: locationId
            }
          }
        },
        status: 200,
        statusText: 'OK',
        headers: {},
        config: {} as unknown
      }
      vi.mocked(areaService.createArea).mockResolvedValue(mockResponse)

      const wrapper = mount(AreaForm, {
        props: { locationId }
      })

      // Fill out the form
      await wrapper.find('input[id="name"]').setValue('Test Area')

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Wait for async operations to complete
      await flushPromises()

      // Verify service was called with correct locationId
      expect(areaService.createArea).toHaveBeenCalledWith({
        data: {
          type: 'areas',
          attributes: {
            name: 'Test Area',
            location_id: locationId
          }
        }
      })
    })
  })
})
