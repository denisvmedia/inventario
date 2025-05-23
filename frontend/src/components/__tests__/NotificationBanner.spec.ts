import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import NotificationBanner from '../NotificationBanner.vue'

// Create mock for FontAwesomeIcon
const mockFontAwesomeIcon = {
  name: 'FontAwesomeIcon',
  template: '<span class="icon" :data-icon="icon" />',
  props: ['icon']
}

describe('NotificationBanner.vue', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('renders correctly with default props', () => {
    const wrapper = mount(NotificationBanner, {
      slots: {
        default: 'Test notification message'
      },
      global: {
        stubs: {
          FontAwesomeIcon: mockFontAwesomeIcon
        }
      }
    })

    expect(wrapper.text()).toContain('Test notification message')
    expect(wrapper.classes()).toContain('notification-banner')
    expect(wrapper.classes()).toContain('info') // Default type is info
    expect(wrapper.find('.notification-close').exists()).toBe(true) // Default is dismissible
  })

  it('applies the correct type class', () => {
    const types = ['info', 'warning', 'error', 'success']

    types.forEach(type => {
      const wrapper = mount(NotificationBanner, {
        props: { type },
        slots: { default: 'Message' },
        global: {
          stubs: {
            FontAwesomeIcon: mockFontAwesomeIcon
          }
        }
      })

      expect(wrapper.classes()).toContain(type)
    })
  })

  it('shows the correct icon based on type', async () => {
    const typeIconMap = {
      'info': 'info-circle',
      'warning': 'exclamation-triangle',
      'error': 'exclamation-circle',
      'success': 'check-circle'
    }

    for (const [type, expectedIcon] of Object.entries(typeIconMap)) {
      const wrapper = mount(NotificationBanner, {
        props: { type },
        slots: { default: 'Message' },
        global: {
          stubs: {
            FontAwesomeIcon: mockFontAwesomeIcon
          }
        }
      })

      const iconElement = wrapper.find('.icon')
      expect(iconElement.attributes('data-icon')).toBe(expectedIcon)
    }
  })

  it('can be dismissed when dismissible is true', async () => {
    const wrapper = mount(NotificationBanner, {
      props: { dismissible: true },
      slots: { default: 'Dismissible message' },
      global: {
        stubs: {
          FontAwesomeIcon: mockFontAwesomeIcon
        }
      }
    })

    expect(wrapper.isVisible()).toBe(true)

    await wrapper.find('.notification-close').trigger('click')

    // The component should no longer be visible
    expect(wrapper.isVisible()).toBe(false)
  })

  it('cannot be dismissed when dismissible is false', () => {
    const wrapper = mount(NotificationBanner, {
      props: { dismissible: false },
      slots: { default: 'Non-dismissible message' },
      global: {
        stubs: {
          FontAwesomeIcon: mockFontAwesomeIcon
        }
      }
    })

    // Close button should not exist
    expect(wrapper.find('.notification-close').exists()).toBe(false)
  })

  it('auto-closes after specified duration', async () => {
    vi.useFakeTimers()

    const wrapper = mount(NotificationBanner, {
      props: { autoClose: 500 }, // Auto-close after 500ms
      slots: { default: 'Auto-closing message' },
      global: {
        stubs: {
          FontAwesomeIcon: mockFontAwesomeIcon
        }
      }
    })

    expect(wrapper.isVisible()).toBe(true)

    // Fast-forward time by 600ms
    vi.advanceTimersByTime(600)
    await wrapper.vm.$nextTick()

    // The component should no longer be visible
    expect(wrapper.isVisible()).toBe(false)

    vi.useRealTimers()
  })

  it('does not auto-close when autoClose is 0', async () => {
    vi.useFakeTimers()

    const wrapper = mount(NotificationBanner, {
      props: { autoClose: 0 }, // No auto-close
      slots: { default: 'Non-auto-closing message' },
      global: {
        stubs: {
          FontAwesomeIcon: mockFontAwesomeIcon
        }
      }
    })

    expect(wrapper.isVisible()).toBe(true)

    // Fast-forward time by 1000ms
    vi.advanceTimersByTime(1000)
    await wrapper.vm.$nextTick()

    // The component should still be visible
    expect(wrapper.isVisible()).toBe(true)

    vi.useRealTimers()
  })

  it('renders with different types', () => {
    const validTypes = ['info', 'warning', 'error', 'success']

    validTypes.forEach(type => {
      const wrapper = mount(NotificationBanner, {
        props: { type },
        slots: { default: 'Test message' },
        global: {
          stubs: {
            FontAwesomeIcon: mockFontAwesomeIcon
          }
        }
      })

      expect(wrapper.classes()).toContain(type)
    })
  })

  it('renders slot content correctly', () => {
    const slotContent = '<strong>Important</strong> notification'
    const wrapper = mount(NotificationBanner, {
      slots: {
        default: slotContent
      },
      global: {
        stubs: {
          FontAwesomeIcon: mockFontAwesomeIcon
        }
      }
    })

    expect(wrapper.find('.notification-message').html()).toContain(slotContent)
  })
})
