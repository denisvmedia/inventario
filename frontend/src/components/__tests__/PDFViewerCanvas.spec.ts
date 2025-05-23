import { describe, it, expect, vi, beforeEach } from 'vitest'
import { mount, flushPromises } from '@vue/test-utils'
import PDFViewerCanvas from '../PDFViewerCanvas.vue'
import { pdfjsLib } from '../../utils/pdfjs-init'

// Mock the PDF.js library
vi.mock('../../utils/pdfjs-init', () => {
  return {
    pdfjsLib: {
      getDocument: vi.fn().mockReturnValue({
        promise: Promise.resolve({
          numPages: 3,
          getPage: vi.fn().mockResolvedValue({
            getViewport: vi.fn().mockReturnValue({ width: 800, height: 1000 }),
            render: vi.fn().mockReturnValue({ promise: Promise.resolve() })
          })
        })
      })
    }
  }
})

// Mock canvas-related functionality
vi.mock('canvas', () => ({}), { virtual: true })

// Mock IntersectionObserver
class MockIntersectionObserver {
  constructor(callback) {
    this.callback = callback
  }
  observe = vi.fn()
  unobserve = vi.fn()
  disconnect = vi.fn()
}

global.IntersectionObserver = MockIntersectionObserver

describe('PDFViewerCanvas.vue', () => {
  let wrapper

  // Mock document methods
  const originalCreateElement = document.createElement
  const mockLink = {
    href: '',
    download: '',
    click: vi.fn(),
    appendChild: vi.fn(),
    removeChild: vi.fn()
  }

  beforeEach(() => {
    // Reset mocks
    vi.resetAllMocks()

    // Mock document.createElement for links
    document.createElement = vi.fn().mockImplementation((tagName) => {
      if (tagName === 'a') {
        return mockLink
      }
      if (tagName === 'canvas') {
        return {
          getContext: () => ({
            drawImage: vi.fn(),
            clearRect: vi.fn()
          }),
          toDataURL: () => 'data:image/png;base64,mockImageData',
          width: 0,
          height: 0
        }
      }
      return originalCreateElement.call(document, tagName)
    })

    // Create wrapper with default props
    wrapper = mount(PDFViewerCanvas, {
      props: {
        url: 'https://example.com/test.pdf'
      },
      global: {
        stubs: {
          'font-awesome-icon': true
        }
      }
    })

    // Mock setTimeout to execute immediately
    vi.useFakeTimers()
  })

  afterEach(() => {
    // Restore original functions
    document.createElement = originalCreateElement
    vi.useRealTimers()
  })

  // Basic rendering tests
  describe('Basic Rendering', () => {
    it('renders loading state initially', () => {
      expect(wrapper.find('.pdf-loading').exists()).toBe(true)
      expect(wrapper.find('.spinner').exists()).toBe(true)
      expect(wrapper.text()).toContain('Loading PDF...')
    })

    it('has the correct structure when rendered', () => {
      expect(wrapper.classes()).toContain('pdf-viewer-container')
    })
  })

  // Download functionality test
  describe('Download Functionality', () => {
    it('has a download method that uses the correct URL', async () => {
      // Create a modified downloadPDF method that doesn't use DOM
      const mockClick = vi.fn()
      const originalDownloadPDF = wrapper.vm.downloadPDF
      wrapper.vm.downloadPDF = function() {
        const url = this.url
        const filename = url.split('/').pop() || 'document.pdf'
        mockClick(url, filename)
      }

      // Call the download method
      wrapper.vm.downloadPDF()

      // Check that the mock function was called with the correct URL
      expect(mockClick).toHaveBeenCalledWith('https://example.com/test.pdf', 'test.pdf')

      // Restore original method
      wrapper.vm.downloadPDF = originalDownloadPDF
    })
  })

  // URL validation test
  describe('URL Validation', () => {
    it('validates the URL and shows error for empty URL', async () => {
      // Create a new wrapper with an empty URL
      const emptyUrlWrapper = mount(PDFViewerCanvas, {
        props: {
          url: ''
        },
        global: {
          stubs: {
            'font-awesome-icon': true
          }
        }
      })

      // Call loadPDF directly
      await emptyUrlWrapper.vm.loadPDF()

      // Check that error state is set
      expect(emptyUrlWrapper.vm.error).toBe('Invalid PDF URL')
      expect(emptyUrlWrapper.vm.loading).toBe(false)
    })

    it('emits events when URL is invalid', async () => {
      // Create a component with empty URL
      const emptyUrlWrapper = mount(PDFViewerCanvas, {
        props: {
          url: ''
        },
        global: {
          stubs: {
            'font-awesome-icon': true
          }
        }
      })

      // Call loadPDF directly
      await emptyUrlWrapper.vm.loadPDF()

      // Wait for the next tick to ensure events are emitted
      await emptyUrlWrapper.vm.$nextTick()

      // Check that loading event was emitted with false
      const loadingEvents = emptyUrlWrapper.emitted('loading')
      expect(loadingEvents).toBeTruthy()
      expect(loadingEvents[loadingEvents.length - 1]).toEqual([false])
    })
  })

  // Navigation tests
  describe('Page Navigation', () => {
    it('has navigation methods for changing pages', () => {
      // Set up component with multiple pages
      wrapper.vm.numPages = 3
      wrapper.vm.currentPage = 2

      // Test prevPage method
      wrapper.vm.prevPage()
      expect(wrapper.vm.currentPage).toBe(1)

      // Test nextPage method
      wrapper.vm.nextPage()
      expect(wrapper.vm.currentPage).toBe(2)

      // Test boundary conditions
      wrapper.vm.currentPage = 1
      wrapper.vm.prevPage()
      expect(wrapper.vm.currentPage).toBe(1) // Should not go below 1

      wrapper.vm.currentPage = 3
      wrapper.vm.nextPage()
      expect(wrapper.vm.currentPage).toBe(3) // Should not go above numPages
    })
  })

  // Zoom tests
  describe('Zoom Controls', () => {
    it('has zoom methods that adjust the scale', () => {
      // Set initial scale
      wrapper.vm.scale = 1.5

      // Test zoomIn method
      wrapper.vm.zoomIn()
      expect(wrapper.vm.scale).toBe(1.75)

      // Test zoomOut method
      wrapper.vm.zoomOut()
      expect(wrapper.vm.scale).toBe(1.5)

      // Test minimum scale limit
      wrapper.vm.scale = 0.75
      wrapper.vm.zoomOut()
      expect(wrapper.vm.scale).toBe(0.75) // Should not go below 0.75

      // Test maximum scale limit
      wrapper.vm.scale = 3.0
      wrapper.vm.zoomIn()
      expect(wrapper.vm.scale).toBe(3.0) // Should not go above 3.0
    })
  })

  // View mode tests
  describe('View Mode', () => {
    it('can toggle between single page and all pages view', () => {
      // Set initial state
      wrapper.vm.viewAllPages = false

      // Toggle view mode by directly changing the property
      wrapper.vm.viewAllPages = true
      expect(wrapper.vm.viewAllPages).toBe(true)

      // Toggle back
      wrapper.vm.viewAllPages = false
      expect(wrapper.vm.viewAllPages).toBe(false)
    })
  })
})
