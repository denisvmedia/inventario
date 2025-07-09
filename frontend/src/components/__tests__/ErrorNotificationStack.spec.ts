import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import ErrorNotificationStack from '../ErrorNotificationStack.vue'

describe('ErrorNotificationStack.vue', () => {
  const mockErrors = [
    {
      id: 'error-1',
      message: 'Cannot delete area because it contains commodities. Please remove all commodities first.',
      timestamp: Date.now(),
      context: 'area'
    },
    {
      id: 'error-2',
      message: 'Cannot delete location because it contains areas. Please remove all areas first.',
      timestamp: Date.now() + 1000,
      context: 'location'
    }
  ]

  describe('Rendering', () => {
    it('renders nothing when no errors are provided', () => {
      const wrapper = mount(ErrorNotificationStack, {
        props: {
          errors: []
        }
      })

      expect(wrapper.find('.error-notification').exists()).toBe(false)
    })

    it('renders error notifications when errors are provided', () => {
      const wrapper = mount(ErrorNotificationStack, {
        props: {
          errors: mockErrors
        }
      })

      const notifications = wrapper.findAll('.error-notification')
      expect(notifications).toHaveLength(2)
    })

    it('displays error messages correctly', () => {
      const wrapper = mount(ErrorNotificationStack, {
        props: {
          errors: mockErrors
        }
      })

      const messages = wrapper.findAll('.error-message')
      expect(messages[0].text()).toBe(mockErrors[0].message)
      expect(messages[1].text()).toBe(mockErrors[1].message)
    })

    it('displays context information when provided', () => {
      const wrapper = mount(ErrorNotificationStack, {
        props: {
          errors: mockErrors
        }
      })

      const contexts = wrapper.findAll('.error-context')
      expect(contexts[0].text()).toBe('Error in area operation')
      expect(contexts[1].text()).toBe('Error in location operation')
    })

    it('displays mobile hint for touch interactions', () => {
      const wrapper = mount(ErrorNotificationStack, {
        props: {
          errors: mockErrors
        }
      })

      const hints = wrapper.findAll('.error-hint')
      expect(hints).toHaveLength(2)
      expect(hints[0].text()).toBe('Tap × or swipe to dismiss')
    })

    it('does not display context when not provided', () => {
      const errorWithoutContext = [{
        id: 'error-1',
        message: 'Test error',
        timestamp: Date.now()
      }]

      const wrapper = mount(ErrorNotificationStack, {
        props: {
          errors: errorWithoutContext
        }
      })

      expect(wrapper.find('.error-context').exists()).toBe(false)
    })
  })

  describe('Interaction', () => {
    it('emits dismiss event when dismiss button is clicked', async () => {
      const wrapper = mount(ErrorNotificationStack, {
        props: {
          errors: mockErrors
        }
      })

      const dismissButton = wrapper.find('.error-dismiss')
      await dismissButton.trigger('click')

      expect(wrapper.emitted('dismiss')).toBeTruthy()
      expect(wrapper.emitted('dismiss')?.[0]).toEqual(['error-1'])
    })

    it('emits correct error ID when dismissing specific error', async () => {
      const wrapper = mount(ErrorNotificationStack, {
        props: {
          errors: mockErrors
        }
      })

      const dismissButtons = wrapper.findAll('.error-dismiss')
      await dismissButtons[1].trigger('click')

      expect(wrapper.emitted('dismiss')?.[0]).toEqual(['error-2'])
    })

    it('handles touch events for swipe-to-dismiss', async () => {
      const wrapper = mount(ErrorNotificationStack, {
        props: {
          errors: mockErrors
        }
      })

      const notification = wrapper.find('.error-notification')

      // Simulate swipe gesture
      await notification.trigger('touchstart', {
        touches: [{ clientX: 100, clientY: 100 }]
      })

      await notification.trigger('touchend', {
        changedTouches: [{ clientX: 200, clientY: 100 }]
      })

      expect(wrapper.emitted('dismiss')?.[0]).toEqual(['error-1'])
    })
  })

  describe('Accessibility', () => {
    it('has proper ARIA attributes', () => {
      const wrapper = mount(ErrorNotificationStack, {
        props: {
          errors: mockErrors
        }
      })

      const notifications = wrapper.findAll('.error-notification')
      notifications.forEach(notification => {
        expect(notification.attributes('role')).toBe('alert')
        expect(notification.attributes('aria-live')).toBe('assertive')
      })
    })

    it('has accessible dismiss buttons', () => {
      const wrapper = mount(ErrorNotificationStack, {
        props: {
          errors: mockErrors
        }
      })

      const dismissButtons = wrapper.findAll('.error-dismiss')
      dismissButtons.forEach(button => {
        expect(button.attributes('aria-label')).toBe('Dismiss error')
        expect(button.attributes('type')).toBe('button')
      })
    })
  })

  describe('Styling', () => {
    it('applies fixed positioning styles', () => {
      const wrapper = mount(ErrorNotificationStack, {
        props: {
          errors: mockErrors
        }
      })

      const stack = wrapper.find('.error-notification-stack')
      expect(stack.exists()).toBe(true)
    })

    it('applies error notification styles', () => {
      const wrapper = mount(ErrorNotificationStack, {
        props: {
          errors: mockErrors
        }
      })

      const notifications = wrapper.findAll('.error-notification')
      notifications.forEach(notification => {
        expect(notification.classes()).toContain('error-notification')
      })
    })
  })
})
