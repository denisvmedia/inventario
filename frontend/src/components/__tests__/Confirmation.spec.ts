import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount } from '@vue/test-utils'
import Confirmation from '../Confirmation.vue'

// Mock the isIconRegistered function
vi.mock('@/utils/faHelper.ts', () => ({
  isIconRegistered: vi.fn((icon) => {
    // Mock implementation that validates a few icons
    const validIcons = ['exclamation-triangle', 'exclamation-circle', 'check-circle', 'info-circle']
    return validIcons.includes(icon)
  })
}))

describe('Confirmation.vue', () => {
  // Default props for most tests
  const defaultProps = {
    title: 'Confirmation Title',
    message: 'Are you sure you want to proceed?',
    confirmLabel: 'Yes, Proceed',
    cancelLabel: 'Cancel',
    visible: true
  }

  // Helper function to create a wrapper with custom props
  const createWrapper = (props = {}) => {
    return mount(Confirmation, {
      props: { ...defaultProps, ...props },
      global: {
        stubs: {
          Dialog: true,
          FontAwesomeIcon: true
        }
      }
    })
  }

  beforeEach(() => {
    vi.clearAllMocks()
  })

  // Rendering tests
  describe('Rendering', () => {
    it('renders with the correct props', () => {
      const wrapper = createWrapper()
      expect(wrapper.props('title')).toBe('Confirmation Title')
      expect(wrapper.props('message')).toBe('Are you sure you want to proceed?')
      expect(wrapper.props('confirmLabel')).toBe('Yes, Proceed')
      expect(wrapper.props('cancelLabel')).toBe('Cancel')
      expect(wrapper.props('visible')).toBe(true)
    })

    it('uses the provided confirmButtonClass', () => {
      const wrapper = createWrapper({ confirmButtonClass: 'danger' })
      expect(wrapper.props('confirmButtonClass')).toBe('danger')
    })

    it('uses the provided confirmationIcon', () => {
      const wrapper = createWrapper({ confirmationIcon: 'exclamation-triangle' })
      expect(wrapper.props('confirmationIcon')).toBe('exclamation-triangle')
    })
  })

  // Event tests
  describe('Events', () => {
    it('emits cancel and update:visible events when cancel method is called', async () => {
      const wrapper = createWrapper()

      // Call the cancel method directly
      await wrapper.vm.cancel()

      // Check that the cancel event was emitted
      expect(wrapper.emitted('cancel')).toBeTruthy()
      expect(wrapper.emitted('cancel')!.length).toBe(1)

      // Check that the update:visible event was emitted with false
      expect(wrapper.emitted('update:visible')).toBeTruthy()
      expect(wrapper.emitted('update:visible')!.length).toBe(1)
      expect(wrapper.emitted('update:visible')![0][0]).toBe(false)
    })

    it('emits confirm event when confirm method is called', async () => {
      const wrapper = createWrapper()

      // Call the confirm method directly
      await wrapper.vm.confirm()

      // Check that the confirm event was emitted
      expect(wrapper.emitted('confirm')).toBeTruthy()
      expect(wrapper.emitted('confirm')!.length).toBe(1)
    })
  })

  // Computed properties tests
  describe('Computed Properties', () => {
    it('dialogVisible getter returns the visible prop value', () => {
      const wrapper = createWrapper({ visible: true })
      expect(wrapper.vm.dialogVisible).toBe(true)

      const wrapper2 = createWrapper({ visible: false })
      expect(wrapper2.vm.dialogVisible).toBe(false)
    })

    it('dialogVisible setter emits update:visible event', async () => {
      const wrapper = createWrapper()

      // Call the setter directly
      wrapper.vm.dialogVisible = false

      // Check that the update:visible event was emitted with false
      expect(wrapper.emitted('update:visible')).toBeTruthy()
      expect(wrapper.emitted('update:visible')![0][0]).toBe(false)
    })
  })

  // Prop validation tests
  describe('Prop Validation', () => {
    it('validates confirmButtonClass prop', () => {
      // Valid values should not throw errors
      expect(() => createWrapper({ confirmButtonClass: 'primary' })).not.toThrow()
      expect(() => createWrapper({ confirmButtonClass: 'danger' })).not.toThrow()
      expect(() => createWrapper({ confirmButtonClass: 'warning' })).not.toThrow()
      expect(() => createWrapper({ confirmButtonClass: 'success' })).not.toThrow()
      expect(() => createWrapper({ confirmButtonClass: 'secondary' })).not.toThrow()

      // Empty string should be valid (will use default)
      expect(() => createWrapper({ confirmButtonClass: '' })).not.toThrow()

      // Invalid value should log a warning (but not throw in non-production)
      const consoleWarnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      createWrapper({ confirmButtonClass: 'invalid-class' })
      expect(consoleWarnSpy).toHaveBeenCalled()
      consoleWarnSpy.mockRestore()
    })

    it('validates confirmationIcon prop using isIconRegistered', () => {
      // Valid icons should not throw errors
      expect(() => createWrapper({ confirmationIcon: 'exclamation-triangle' })).not.toThrow()
      expect(() => createWrapper({ confirmationIcon: 'exclamation-circle' })).not.toThrow()

      // Empty string should be valid (will not show icon)
      expect(() => createWrapper({ confirmationIcon: '' })).not.toThrow()

      // Invalid icon should log a warning (but not throw in non-production)
      const consoleWarnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
      createWrapper({ confirmationIcon: 'invalid-icon' })
      expect(consoleWarnSpy).toHaveBeenCalled()
      consoleWarnSpy.mockRestore()
    })
  })
})
