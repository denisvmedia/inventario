import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, VueWrapper } from '@vue/test-utils'
import LocationForm from '../LocationForm.vue'
import locationService from '@/services/locationService'

// Mock the locationService
vi.mock('@/services/locationService', () => ({
  default: {
    createLocation: vi.fn()
  }
}))

describe('LocationForm.vue', () => {
  let wrapper: VueWrapper<any>

  beforeEach(() => {
    // Reset mocks before each test
    vi.resetAllMocks()

    // Create a fresh wrapper
    wrapper = mount(LocationForm)
  })

  // Rendering tests
  describe('Rendering', () => {
    it('renders the form with correct fields', () => {
      // Check that the form exists
      expect(wrapper.find('form').exists()).toBe(true)

      // Check that name input exists
      const nameInput = wrapper.find('input#name')
      expect(nameInput.exists()).toBe(true)
      expect(nameInput.attributes('placeholder')).toBe('Enter location name')

      // Check that address input exists
      const addressInput = wrapper.find('input#address')
      expect(addressInput.exists()).toBe(true)
      expect(addressInput.attributes('placeholder')).toBe('Enter location address')

      // Check that buttons exist
      expect(wrapper.find('button[type="button"]').text()).toBe('Cancel')
      expect(wrapper.find('button[type="submit"]').text()).toBe('Create Location')
    })

    it('does not show error messages initially', () => {
      // Check that error messages are not displayed initially
      expect(wrapper.find('.error-message').exists()).toBe(false)
      expect(wrapper.find('.form-error').exists()).toBe(false)
    })
  })

  // Form validation tests
  describe('Form Validation', () => {
    it('shows validation errors when submitting empty form', async () => {
      // Submit the form without filling in any fields
      await wrapper.find('form').trigger('submit')

      // Check that error messages are displayed
      expect(wrapper.findAll('.error-message').length).toBe(2)
      expect(wrapper.find('.error-message').text()).toContain('Name is required')
    })

    it('validates name field', async () => {
      // Fill in only the address field
      await wrapper.find('input#address').setValue('123 Test Street')

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check that only name error is displayed
      const errorMessages = wrapper.findAll('.error-message')
      expect(errorMessages.length).toBe(1)
      expect(errorMessages[0].text()).toBe('Name is required')
    })

    it('validates address field', async () => {
      // Fill in only the name field
      await wrapper.find('input#name').setValue('Test Location')

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check that only address error is displayed
      const errorMessages = wrapper.findAll('.error-message')
      expect(errorMessages.length).toBe(1)
      expect(errorMessages[0].text()).toBe('Address is required')
    })

    it('passes validation with valid data', async () => {
      // Fill in both fields
      await wrapper.find('input#name').setValue('Test Location')
      await wrapper.find('input#address').setValue('123 Test Street')

      // Mock the createLocation method to resolve successfully
      const mockResponse = {
        data: {
          data: {
            id: 'location-123',
            type: 'locations',
            attributes: {
              name: 'Test Location',
              address: '123 Test Street'
            }
          }
        }
      }
      vi.mocked(locationService.createLocation).mockResolvedValue(mockResponse)

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check that no error messages are displayed
      expect(wrapper.find('.error-message').exists()).toBe(false)
    })
  })

  // Form submission tests
  describe('Form Submission', () => {
    it('calls locationService.createLocation with correct payload', async () => {
      // Fill in form fields
      await wrapper.find('input#name').setValue('Test Location')
      await wrapper.find('input#address').setValue('123 Test Street')

      // Mock the createLocation method to resolve successfully
      const mockResponse = {
        data: {
          data: {
            id: 'location-123',
            type: 'locations',
            attributes: {
              name: 'Test Location',
              address: '123 Test Street'
            }
          }
        }
      }
      vi.mocked(locationService.createLocation).mockResolvedValue(mockResponse)

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check that locationService.createLocation was called with the correct payload
      expect(locationService.createLocation).toHaveBeenCalledWith({
        data: {
          type: 'locations',
          attributes: {
            name: 'Test Location',
            address: '123 Test Street'
          }
        }
      })
    })

    it('emits created event with location data on successful submission', async () => {
      // Fill in form fields
      await wrapper.find('input#name').setValue('Test Location')
      await wrapper.find('input#address').setValue('123 Test Street')

      // Mock the createLocation method to resolve successfully
      const mockLocationData = {
        id: 'location-123',
        type: 'locations',
        attributes: {
          name: 'Test Location',
          address: '123 Test Street'
        }
      }
      const mockResponse = {
        data: {
          data: mockLocationData
        }
      }
      vi.mocked(locationService.createLocation).mockResolvedValue(mockResponse)

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check that created event was emitted with the location data
      expect(wrapper.emitted('created')).toBeTruthy()
      expect(wrapper.emitted('created')![0][0]).toEqual(mockLocationData)
    })

    it('resets form after successful submission', async () => {
      // Fill in form fields
      await wrapper.find('input#name').setValue('Test Location')
      await wrapper.find('input#address').setValue('123 Test Street')

      // Mock the createLocation method to resolve successfully
      const mockResponse = {
        data: {
          data: {
            id: 'location-123',
            type: 'locations',
            attributes: {
              name: 'Test Location',
              address: '123 Test Street'
            }
          }
        }
      }
      vi.mocked(locationService.createLocation).mockResolvedValue(mockResponse)

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check that form fields are reset
      expect((wrapper.find('input#name').element as HTMLInputElement).value).toBe('')
      expect((wrapper.find('input#address').element as HTMLInputElement).value).toBe('')
    })

    it('shows loading state during submission', async () => {
      // Fill in form fields
      await wrapper.find('input#name').setValue('Test Location')
      await wrapper.find('input#address').setValue('123 Test Street')

      // Create a promise that we can resolve later
      // eslint-disable-next-line no-unused-vars
      let resolvePromise: (value: any) => void
      const promise = new Promise((resolve) => {
        resolvePromise = resolve
      })

      // Mock the createLocation method to return our controlled promise
      vi.mocked(locationService.createLocation).mockReturnValue(promise as any)

      // Submit the form
      const submitPromise = wrapper.find('form').trigger('submit')

      // Check that the submit button shows loading state
      await wrapper.vm.$nextTick()
      expect(wrapper.find('button[type="submit"]').text()).toBe('Creating...')
      expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeDefined()

      // Resolve the promise
      resolvePromise!({
        data: {
          data: {
            id: 'location-123',
            type: 'locations',
            attributes: {
              name: 'Test Location',
              address: '123 Test Street'
            }
          }
        }
      })

      // Wait for the submit promise to resolve
      await submitPromise
      await wrapper.vm.$nextTick()
      await wrapper.vm.$nextTick() // Need multiple ticks for the UI to update

      // Check that the submit button is back to normal
      expect(wrapper.find('button[type="submit"]').text()).toBe('Create Location')
      expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeUndefined()
    })

    it('handles API errors correctly', async () => {
      // Fill in form fields
      await wrapper.find('input#name').setValue('Test Location')
      await wrapper.find('input#address').setValue('123 Test Street')

      // Mock the createLocation method to reject with an error
      const mockError = {
        response: {
          status: 422,
          data: {
            errors: [
              {
                source: {
                  pointer: '/data/attributes/name'
                },
                detail: 'Location name is already used'
              }
            ]
          }
        }
      }
      vi.mocked(locationService.createLocation).mockRejectedValue(mockError)

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check that error message is displayed
      expect(wrapper.find('.error-message').text()).toBe('Location name is already used')
      expect(wrapper.find('.form-error').text()).toBe('Please correct the errors above.')
    })

    it('handles generic API errors', async () => {
      // Fill in form fields
      await wrapper.find('input#name').setValue('Test Location')
      await wrapper.find('input#address').setValue('123 Test Street')

      // Mock the createLocation method to reject with a generic error
      const mockError = {
        message: 'Network Error'
      }
      vi.mocked(locationService.createLocation).mockRejectedValue(mockError)

      // Submit the form
      await wrapper.find('form').trigger('submit')

      // Check that error message is displayed
      expect(wrapper.find('.form-error').text()).toBe('Failed to create location: Network Error')
    })
  })

  // Cancel button tests
  describe('Cancel Button', () => {
    it('emits cancel event when cancel button is clicked', async () => {
      // Click the cancel button
      await wrapper.find('button[type="button"]').trigger('click')

      // Check that cancel event was emitted
      expect(wrapper.emitted('cancel')).toBeTruthy()
    })

    it('resets form when cancel button is clicked', async () => {
      // Fill in form fields
      await wrapper.find('input#name').setValue('Test Location')
      await wrapper.find('input#address').setValue('123 Test Street')

      // Click the cancel button
      await wrapper.find('button[type="button"]').trigger('click')

      // Check that form fields are reset
      expect((wrapper.find('input#name').element as HTMLInputElement).value).toBe('')
      expect((wrapper.find('input#address').element as HTMLInputElement).value).toBe('')
    })
  })
})
