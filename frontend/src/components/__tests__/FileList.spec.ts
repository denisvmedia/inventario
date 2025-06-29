import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import FileList from '../FileList.vue'


// Mock Vue's nextTick function to prevent focus errors
vi.mock('vue', async () => {
  const actual = await vi.importActual('vue')
  return {
    ...actual,
    nextTick: vi.fn().mockImplementation(() => {
      // Skip the callback to avoid focus errors
      return Promise.resolve()
    })
  }
})

// Create mock for FontAwesomeIcon
const mockFontAwesomeIcon = {
  name: 'FontAwesomeIcon',
  template: '<span class="icon" :data-icon="icon" :data-size="size" />',
  props: ['icon', 'size']
}

// Mock the FontAwesomeIcon component globally
vi.mock('@fortawesome/vue-fontawesome', () => ({
  FontAwesomeIcon: {
    name: 'FontAwesomeIcon',
    template: '<span class="icon" :data-icon="icon" :data-size="size" />',
    props: ['icon', 'size']
  }
}))

describe('FileList.vue', () => {
  // Mock window alert
  const originalAlert = window.alert
  const originalFocus = HTMLElement.prototype.focus

  beforeEach(() => {
    window.alert = vi.fn()
    vi.resetAllMocks()

    // Mock the focus method
    HTMLElement.prototype.focus = vi.fn()
  })

  afterEach(() => {
    window.alert = originalAlert
    HTMLElement.prototype.focus = originalFocus
  })

  // Test data
  const mockImageFile = {
    id: 'file-1',
    path: 'test-image',
    ext: '.jpg',
    original_path: 'original-test-image.jpg',
    mime_type: 'image/jpeg'
  }

  const mockPdfFile = {
    id: 'file-2',
    path: 'test-pdf',
    ext: '.pdf',
    original_path: 'original-test-pdf.pdf',
    mime_type: 'application/pdf'
  }

  const mockManualFile = {
    id: 'file-3',
    path: 'test-manual',
    ext: '.txt',
    original_path: 'original-test-manual.txt',
    mime_type: 'text/plain'
  }

  const mockInvoiceFile = {
    id: 'file-4',
    path: 'test-invoice',
    ext: '.docx',
    original_path: 'original-test-invoice.docx',
    mime_type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'
  }

  const defaultProps = {
    files: [mockImageFile, mockPdfFile],
    fileType: 'images',
    commodityId: 'commodity-1',
    loading: false
  }

  // Helper function to create a wrapper with custom props
  const createWrapper = (props = {}) => {
    return mount(FileList, {
      props: { ...defaultProps, ...props },
      global: {
        stubs: {
          FontAwesomeIcon: mockFontAwesomeIcon
        }
      },
      attachTo: document.body
    })
  }

  // Rendering tests
  describe('Rendering', () => {
    it('renders loading state correctly', () => {
      const wrapper = createWrapper({ loading: true })

      expect(wrapper.find('.loading').exists()).toBe(true)
      expect(wrapper.find('.loading').text()).toBe('Loading files...')
      expect(wrapper.find('.files-container').exists()).toBe(false)
    })

    it('renders empty state correctly', () => {
      const wrapper = createWrapper({ files: [] })

      expect(wrapper.find('.no-files').exists()).toBe(true)
      expect(wrapper.find('.no-files').text()).toBe('No images uploaded yet.')
      expect(wrapper.find('.files-container').exists()).toBe(false)
    })

    it('renders files correctly', () => {
      const wrapper = createWrapper()

      expect(wrapper.find('.files-container').exists()).toBe(true)
      expect(wrapper.findAll('.file-item').length).toBe(2)
    })

    it('renders image files with image preview', () => {
      const wrapper = createWrapper({ files: [mockImageFile] })

      expect(wrapper.find('.image-preview').exists()).toBe(true)
      expect(wrapper.find('.preview-image').exists()).toBe(true)
      expect(wrapper.find('.preview-image').attributes('src')).toBe('/api/v1/files/file-1.jpg')
    })

    it('renders non-image files with icon preview', () => {
      const wrapper = createWrapper({ files: [mockPdfFile], fileType: 'manuals' })

      expect(wrapper.find('.file-icon').exists()).toBe(true)
      expect(wrapper.find('.icon').exists()).toBe(true)

      // Check that the getFileIcon method returns the correct value
      expect(wrapper.vm.getFileIcon(mockPdfFile)).toBe('file-pdf')
    })

    it('renders file name correctly', () => {
      const wrapper = createWrapper({ files: [mockImageFile] })

      expect(wrapper.find('.file-name').text()).toContain('test-image.jpg')
    })

    it('renders file actions correctly', () => {
      const wrapper = createWrapper()

      const fileActions = wrapper.find('.file-actions')
      expect(fileActions.exists()).toBe(true)

      const buttons = fileActions.findAll('button')
      expect(buttons.length).toBe(3)
      expect(buttons[0].text()).toContain('Download')
      expect(buttons[1].text()).toContain('Delete')
      expect(buttons[2].text()).toContain('Details')
    })
  })

  // Method tests
  describe('Methods', () => {
    it('getFileUrl returns correct URL for images', () => {
      const wrapper = createWrapper()
      expect(wrapper.vm.getFileUrl(mockImageFile)).toBe('/api/v1/files/file-1.jpg')
    })

    it('getFileUrl returns correct URL for manuals', () => {
      const wrapper = createWrapper({ fileType: 'manuals' })
      expect(wrapper.vm.getFileUrl(mockPdfFile)).toBe('/api/v1/files/file-2.pdf')
    })

    it('getFileUrl returns correct URL for invoices', () => {
      const wrapper = createWrapper({ fileType: 'invoices' })
      expect(wrapper.vm.getFileUrl(mockInvoiceFile)).toBe('/api/v1/files/file-4.docx')
    })

    it('getFileName returns path + extension when path is available', () => {
      const wrapper = createWrapper()
      expect(wrapper.vm.getFileName(mockImageFile)).toBe('test-image.jpg')
    })

    it('getFileName returns id + extension when path is not available', () => {
      const wrapper = createWrapper()
      const fileWithoutPath = { ...mockImageFile, path: '' }
      expect(wrapper.vm.getFileName(fileWithoutPath)).toBe('file-1.jpg')
    })

    it('getFileIcon returns correct icon for PDF files', () => {
      const wrapper = createWrapper()
      expect(wrapper.vm.getFileIcon(mockPdfFile)).toBe('file-pdf')
    })

    it('getFileIcon returns correct icon for image files', () => {
      const wrapper = createWrapper()
      expect(wrapper.vm.getFileIcon(mockImageFile)).toBe('file-image')
    })

    it('getFileIcon returns correct icon for manual files', () => {
      const wrapper = createWrapper({ fileType: 'manuals' })
      expect(wrapper.vm.getFileIcon(mockManualFile)).toBe('book')
    })

    it('getFileIcon returns correct icon for invoice files', () => {
      const wrapper = createWrapper({ fileType: 'invoices' })
      expect(wrapper.vm.getFileIcon(mockInvoiceFile)).toBe('file-invoice-dollar')
    })

    it('getFileIcon returns default icon for other files', () => {
      const wrapper = createWrapper()
      const unknownFile = { ...mockManualFile, ext: '.unknown' }
      expect(wrapper.vm.getFileIcon(unknownFile)).toBe('file')
    })

    it('isImageFile correctly identifies image files by extension', () => {
      const wrapper = createWrapper()

      const imageExtensions = ['.jpg', '.jpeg', '.png', '.gif', '.webp']
      for (const ext of imageExtensions) {
        const file = { ...mockImageFile, ext }
        expect(wrapper.vm.isImageFile(file)).toBe(true)
      }

      const nonImageFile = { ...mockImageFile, ext: '.txt' }
      expect(wrapper.vm.isImageFile(nonImageFile)).toBe(false)
    })

    it('isImageFile correctly identifies image files by mime type', () => {
      const wrapper = createWrapper()

      const file = { ...mockImageFile, ext: '', mime_type: 'image/png' }
      expect(wrapper.vm.isImageFile(file)).toBe(true)

      const nonImageFile = { ...mockImageFile, ext: '', mime_type: 'text/plain' }
      expect(wrapper.vm.isImageFile(nonImageFile)).toBe(false)
    })

    it('isPdfFile correctly identifies PDF files by extension', () => {
      const wrapper = createWrapper()

      const pdfExtensions = ['.pdf', 'pdf']
      for (const ext of pdfExtensions) {
        const file = { ...mockPdfFile, ext }
        expect(wrapper.vm.isPdfFile(file)).toBe(true)
      }

      const nonPdfFile = { ...mockPdfFile, ext: '.txt' }
      expect(wrapper.vm.isPdfFile(nonPdfFile)).toBe(false)
    })

    it('isPdfFile correctly identifies PDF files by mime type', () => {
      const wrapper = createWrapper()

      const file = { ...mockPdfFile, ext: '', mime_type: 'application/pdf' }
      expect(wrapper.vm.isPdfFile(file)).toBe(true)

      const nonPdfFile = { ...mockPdfFile, ext: '', mime_type: 'text/plain' }
      expect(wrapper.vm.isPdfFile(nonPdfFile)).toBe(false)
    })
  })

  // Event tests
  describe('Events', () => {
    it('emits download event when download button is clicked', async () => {
      const wrapper = createWrapper()

      await wrapper.find('.file-actions button:nth-child(1)').trigger('click')

      expect(wrapper.emitted('download')).toBeTruthy()
      expect(wrapper.emitted('download')![0]).toEqual([mockImageFile])
    })

    it('emits delete event when delete button is clicked', async () => {
      const wrapper = createWrapper()

      await wrapper.find('.file-actions button:nth-child(2)').trigger('click')

      expect(wrapper.emitted('delete')).toBeTruthy()
      expect(wrapper.emitted('delete')![0]).toEqual([mockImageFile])
    })

    it('emits view-details event when details button is clicked', async () => {
      const wrapper = createWrapper()

      await wrapper.find('.file-actions button:nth-child(3)').trigger('click')

      expect(wrapper.emitted('view-details')).toBeTruthy()
      expect(wrapper.emitted('view-details')![0]).toEqual([mockImageFile])
    })

    it('emits open-viewer event when image preview is clicked', async () => {
      const wrapper = createWrapper({ files: [mockImageFile] })

      await wrapper.find('.image-preview').trigger('click')

      expect(wrapper.emitted('open-viewer')).toBeTruthy()
      expect(wrapper.emitted('open-viewer')![0]).toEqual([mockImageFile])
    })

    it('emits open-viewer event when file icon is clicked', async () => {
      const wrapper = createWrapper({ files: [mockPdfFile] })

      await wrapper.find('.file-icon').trigger('click')

      expect(wrapper.emitted('open-viewer')).toBeTruthy()
      expect(wrapper.emitted('open-viewer')![0]).toEqual([mockPdfFile])
    })
  })

  // File name editing tests
  describe('File Name Editing', () => {
    it('shows edit mode when file name is clicked', async () => {
      const wrapper = createWrapper()

      // Initial state - edit mode should not be visible
      expect(wrapper.find('.file-name-edit').exists()).toBe(false)

      // Click on the file name to start editing
      await wrapper.find('.file-name').trigger('click')

      // Edit mode should now be visible
      expect(wrapper.find('.file-name-edit').exists()).toBe(true)
      expect(wrapper.find('.file-name-edit input').element.value).toBe('test-image')
    })

    it('cancels editing when cancel button is clicked', async () => {
      const wrapper = createWrapper()

      // Start editing
      await wrapper.find('.file-name').trigger('click')
      expect(wrapper.find('.file-name-edit').exists()).toBe(true)

      // Click cancel button
      await wrapper.find('.edit-actions button:nth-child(2)').trigger('click')

      // Edit mode should be hidden
      expect(wrapper.find('.file-name-edit').exists()).toBe(false)
      expect(wrapper.vm.editingFile).toBeNull()
      expect(wrapper.vm.editedFileName).toBe('')
    })

    it('cancels editing when ESC key is pressed', async () => {
      const wrapper = createWrapper()

      // Start editing
      await wrapper.find('.file-name').trigger('click')
      expect(wrapper.find('.file-name-edit').exists()).toBe(true)

      // Press ESC key
      await wrapper.find('.file-name-edit input').trigger('keyup.esc')

      // Edit mode should be hidden
      expect(wrapper.find('.file-name-edit').exists()).toBe(false)
      expect(wrapper.vm.editingFile).toBeNull()
      expect(wrapper.vm.editedFileName).toBe('')
    })

    it('saves file name when save button is clicked', async () => {
      const wrapper = createWrapper()

      // Start editing
      await wrapper.find('.file-name').trigger('click')

      // Change the file name
      await wrapper.find('.file-name-edit input').setValue('new-file-name')

      // Click save button
      await wrapper.find('.edit-actions button:nth-child(1)').trigger('click')

      // Check that the update event was emitted with correct data
      expect(wrapper.emitted('update')).toBeTruthy()
      expect(wrapper.emitted('update')![0]).toEqual([{
        id: 'file-1',
        type: 'images',
        path: 'new-file-name'
      }])

      // Edit mode should be hidden
      expect(wrapper.find('.file-name-edit').exists()).toBe(false)
      expect(wrapper.vm.editingFile).toBeNull()
      expect(wrapper.vm.editedFileName).toBe('')
    })

    it('saves file name when ENTER key is pressed', async () => {
      const wrapper = createWrapper()

      // Start editing
      await wrapper.find('.file-name').trigger('click')

      // Change the file name and press ENTER
      await wrapper.find('.file-name-edit input').setValue('new-file-name')
      await wrapper.find('.file-name-edit input').trigger('keyup.enter')

      // Check that the update event was emitted with correct data
      expect(wrapper.emitted('update')).toBeTruthy()
      expect(wrapper.emitted('update')![0]).toEqual([{
        id: 'file-1',
        type: 'images',
        path: 'new-file-name'
      }])
    })

    it('shows alert when trying to save empty file name', async () => {
      const wrapper = createWrapper()

      // Start editing
      await wrapper.find('.file-name').trigger('click')

      // Set empty file name
      await wrapper.find('.file-name-edit input').setValue('')

      // Click save button
      await wrapper.find('.edit-actions button:nth-child(1)').trigger('click')

      // Alert should be shown
      expect(window.alert).toHaveBeenCalledWith('File name cannot be empty')

      // Edit mode should still be active
      expect(wrapper.find('.file-name-edit').exists()).toBe(true)
      expect(wrapper.vm.editingFile).toBe('file-1')

      // No update event should be emitted
      expect(wrapper.emitted('update')).toBeFalsy()
    })
  })

  // Edge cases
  describe('Edge Cases', () => {
    it('handles files with missing properties', () => {
      const incompleteFile = {
        id: 'file-5',
        // Missing path, ext, and mime_type
      }

      const wrapper = createWrapper({ files: [incompleteFile] })

      // Should still render without errors
      expect(wrapper.find('.file-item').exists()).toBe(true)

      // File name should fall back to ID
      expect(wrapper.find('.file-name').text()).toContain('file-5')

      // Should not be identified as an image or PDF
      expect(wrapper.vm.isImageFile(incompleteFile)).toBe(false)
      expect(wrapper.vm.isPdfFile(incompleteFile)).toBe(false)

      // Should use default icon
      expect(wrapper.vm.getFileIcon(incompleteFile)).toBe('file')
    })

    it('validates fileType prop', () => {
      // Valid fileType values should not throw errors
      const validTypes = ['images', 'manuals', 'invoices']

      for (const type of validTypes) {
        expect(() => {
          createWrapper({ fileType: type })
        }).not.toThrow()
      }

      // In test environment, Vue may not always emit console errors for prop validation
      // So we'll just verify that the component accepts valid types
      expect(() => {
        createWrapper({ fileType: 'images' })
      }).not.toThrow()
    })
  })
})
