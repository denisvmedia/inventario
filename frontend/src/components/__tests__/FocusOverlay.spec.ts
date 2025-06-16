import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import FocusOverlay from '../FocusOverlay.vue'

// Mock FontAwesome
vi.mock('@fortawesome/vue-fontawesome', () => ({
  FontAwesomeIcon: {
    name: 'FontAwesomeIcon',
    template: '<i></i>'
  }
}))

// Create a div for teleport target
const createTeleportTarget = () => {
  const div = document.createElement('div')
  div.id = 'teleport-target'
  document.body.appendChild(div)
  return div
}

describe('FocusOverlay', () => {
  let wrapper: any
  let targetElement: HTMLElement
  let teleportTarget: HTMLElement

  beforeEach(() => {
    // Create teleport target
    teleportTarget = createTeleportTarget()

    // Create a mock target element
    targetElement = document.createElement('button')
    targetElement.textContent = 'Upload Files'
    targetElement.style.position = 'absolute'
    targetElement.style.top = '100px'
    targetElement.style.left = '100px'
    targetElement.style.width = '120px'
    targetElement.style.height = '40px'
    document.body.appendChild(targetElement)

    // Mock getBoundingClientRect
    vi.spyOn(targetElement, 'getBoundingClientRect').mockReturnValue({
      top: 100,
      left: 100,
      width: 120,
      height: 40,
      right: 220,
      bottom: 140,
      x: 100,
      y: 100,
      toJSON: () => ({})
    } as DOMRect)
  })

  afterEach(() => {
    if (wrapper) {
      wrapper.unmount()
    }
    if (targetElement && targetElement.parentNode) {
      targetElement.parentNode.removeChild(targetElement)
    }
    if (teleportTarget && teleportTarget.parentNode) {
      teleportTarget.parentNode.removeChild(teleportTarget)
    }
  })

  it('renders when show is true', async () => {
    wrapper = mount(FocusOverlay, {
      props: {
        show: true,
        targetElement: targetElement,
        message: 'Test message'
      },
      attachTo: teleportTarget
    })

    await wrapper.vm.$nextTick()

    // Check if the overlay is rendered in the body (due to Teleport)
    const overlay = document.querySelector('.focus-overlay')
    expect(overlay).toBeTruthy()
  })

  it('does not render when show is false', async () => {
    wrapper = mount(FocusOverlay, {
      props: {
        show: false,
        targetElement: targetElement
      },
      attachTo: teleportTarget
    })

    await wrapper.vm.$nextTick()

    // Check that no overlay is rendered in the body
    const overlay = document.querySelector('.focus-overlay')
    expect(overlay).toBeFalsy()
  })

  it('has correct props interface', () => {
    wrapper = mount(FocusOverlay, {
      props: {
        show: true,
        targetElement: targetElement,
        message: 'Test message',
        allowClickThrough: true
      },
      attachTo: teleportTarget
    })

    expect(wrapper.props('show')).toBe(true)
    expect(wrapper.props('targetElement')).toBe(targetElement)
    expect(wrapper.props('message')).toBe('Test message')
    expect(wrapper.props('allowClickThrough')).toBe(true)
  })

  it('uses default message when none provided', () => {
    wrapper = mount(FocusOverlay, {
      props: {
        show: true,
        targetElement: targetElement
      },
      attachTo: teleportTarget
    })

    expect(wrapper.props('message')).toBe('Don\'t forget to upload your files!')
  })

  it('emits close event when handleOverlayClick is called', async () => {
    wrapper = mount(FocusOverlay, {
      props: {
        show: true,
        targetElement: targetElement
      },
      attachTo: teleportTarget
    })

    // Simulate overlay click by calling the method directly
    await wrapper.vm.handleOverlayClick({ clientX: 0, clientY: 0 })
    expect(wrapper.emitted('close')).toBeTruthy()
  })
})
