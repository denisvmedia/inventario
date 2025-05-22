import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { mount } from '@vue/test-utils'
import FileUploader from '../FileUploader.vue'

// Mock Vue's nextTick function to prevent focus errors
vi.mock('vue', async () => {
  const actual = await vi.importActual('vue')
  return {
    ...actual,
    nextTick: vi.fn().mockImplementation(callback => {
      // Skip the callback to avoid focus errors
      return Promise.resolve()
    })
  }
})

// Create mock for FontAwesomeIcon
vi.mock('@fortawesome/vue-fontawesome', () => ({
  FontAwesomeIcon: {
    name: 'FontAwesomeIcon',
    template: '<span class="icon" :data-icon="icon" :data-size="size" />',
    props: ['icon', 'size']
  }
}))

describe('FileUploader.vue', () => {
  // Mock console.error
  const originalConsoleError = console.error

  beforeEach(() => {
    console.error = vi.fn()
    vi.resetAllMocks()
  })

  afterEach(() => {
    console.error = originalConsoleError
  })

  // Helper function to create a wrapper with custom props
  const createWrapper = (props = {}) => {
    return mount(FileUploader, {
      props,
      attachTo: document.body
    })
  }

  // Helper function to create a mock file
  const createMockFile = (name = 'test-file.jpg', type = 'image/jpeg', size = 1024) => {
    const file = new File(
      [new ArrayBuffer(size)],
      name,
      { type }
    )
    return file
  }

  // Helper function to create a mock drag event
  const createDragEvent = (eventType: string, files: File[] = []) => {
    const event = new Event(eventType, { bubbles: true })
    Object.defineProperty(event, 'dataTransfer', {
      value: {
        files,
        items: files.map(file => ({
          kind: 'file',
          type: file.type,
          getAsFile: () => file
        })),
        types: ['Files']
      }
    })
    return event
  }

  // Rendering tests
  describe('Rendering', () => {
    it('renders correctly with default props', () => {
      const wrapper = createWrapper()

      expect(wrapper.find('.file-uploader').exists()).toBe(true)
      expect(wrapper.find('.upload-area').exists()).toBe(true)
      expect(wrapper.find('.file-input').exists()).toBe(true)
      expect(wrapper.find('.upload-prompt').text()).toBe('Drag and drop files here')
      expect(wrapper.find('.browse-button').text()).toBe('Browse Files')
      expect(wrapper.find('.selected-files').exists()).toBe(false)
    })

    it('renders with custom upload prompt', () => {
      const wrapper = createWrapper({
        uploadPrompt: 'Custom upload message'
      })

      expect(wrapper.find('.upload-prompt').text()).toBe('Custom upload message')
    })

    it('sets the correct accept attribute on file input', () => {
      const wrapper = createWrapper({
        accept: 'image/*'
      })

      expect(wrapper.find('.file-input').attributes('accept')).toBe('image/*')
    })

    it('sets the correct multiple attribute on file input', () => {
      const wrapper = createWrapper({
        multiple: false
      })

      expect(wrapper.find('.file-input').attributes('multiple')).toBeUndefined()

      const multipleWrapper = createWrapper({
        multiple: true
      })

      expect(multipleWrapper.find('.file-input').attributes('multiple')).toBe('')
    })
  })

  // Drag and drop tests
  describe('Drag and Drop', () => {
    it('updates isDragOver when dragging over the upload area', async () => {
      const wrapper = createWrapper()

      await wrapper.find('.upload-area').trigger('dragover')

      expect(wrapper.find('.upload-area').classes()).toContain('drag-over')
    })

    it('removes isDragOver when dragging leaves the upload area', async () => {
      const wrapper = createWrapper()

      // First dragover to set isDragOver to true
      await wrapper.find('.upload-area').trigger('dragover')
      expect(wrapper.find('.upload-area').classes()).toContain('drag-over')

      // Then dragleave to set it back to false
      await wrapper.find('.upload-area').trigger('dragleave')
      expect(wrapper.find('.upload-area').classes()).not.toContain('drag-over')
    })

    it('handles file drop correctly', async () => {
      const wrapper = createWrapper()
      const mockFile = createMockFile()

      // Create a mock drop event with files
      const dropEvent = createDragEvent('drop', [mockFile])

      // Trigger the drop event
      await wrapper.find('.upload-area').element.dispatchEvent(dropEvent)

      // Check if the file was added to selectedFiles
      expect(wrapper.vm.selectedFiles.length).toBe(1)
      expect(wrapper.vm.selectedFiles[0].name).toBe('test-file.jpg')

      // Check if the selected file is displayed
      expect(wrapper.find('.selected-files').exists()).toBe(true)
      expect(wrapper.find('.file-name').text()).toBe('test-file.jpg')
    })

    it('removes isDragOver when files are dropped', async () => {
      const wrapper = createWrapper()
      const mockFile = createMockFile()

      // First dragover to set isDragOver to true
      await wrapper.find('.upload-area').trigger('dragover')
      expect(wrapper.find('.upload-area').classes()).toContain('drag-over')

      // Create a mock drop event with files
      const dropEvent = createDragEvent('drop', [mockFile])

      // Trigger the drop event
      await wrapper.find('.upload-area').element.dispatchEvent(dropEvent)

      // Check if isDragOver is reset
      expect(wrapper.find('.upload-area').classes()).not.toContain('drag-over')
    })
  })

  // File selection tests
  describe('File Selection', () => {
    it('handles file selection via input correctly', async () => {
      const wrapper = createWrapper()
      const mockFile = createMockFile()

      // Create a mock change event
      const input = wrapper.find('.file-input').element as HTMLInputElement
      Object.defineProperty(input, 'files', {
        value: [mockFile]
      })

      await wrapper.find('.file-input').trigger('change')

      // Check if the file was added to selectedFiles
      expect(wrapper.vm.selectedFiles.length).toBe(1)
      expect(wrapper.vm.selectedFiles[0].name).toBe('test-file.jpg')

      // Check if the selected file is displayed
      expect(wrapper.find('.selected-files').exists()).toBe(true)
      expect(wrapper.find('.file-name').text()).toBe('test-file.jpg')
    })

    it('triggers file input when browse button is clicked', async () => {
      const wrapper = createWrapper()
      const clickSpy = vi.spyOn(wrapper.vm.fileInput, 'click')

      await wrapper.find('.browse-button').trigger('click')

      expect(clickSpy).toHaveBeenCalled()
    })

    it('allows removing selected files', async () => {
      const wrapper = createWrapper()
      const mockFile = createMockFile()

      // Add a file
      wrapper.vm.addFiles([mockFile])
      await wrapper.vm.$nextTick()

      // Check if the file was added
      expect(wrapper.vm.selectedFiles.length).toBe(1)
      expect(wrapper.find('.selected-files').exists()).toBe(true)

      // Remove the file
      await wrapper.find('.remove-file').trigger('click')

      // Check if the file was removed
      expect(wrapper.vm.selectedFiles.length).toBe(0)
      expect(wrapper.find('.selected-files').exists()).toBe(false)
    })

    it('replaces files when multiple is false', () => {
      const wrapper = createWrapper({ multiple: false })
      const mockFile1 = createMockFile('file1.jpg')
      const mockFile2 = createMockFile('file2.jpg')

      // Add first file
      wrapper.vm.addFiles([mockFile1])
      expect(wrapper.vm.selectedFiles.length).toBe(1)
      expect(wrapper.vm.selectedFiles[0].name).toBe('file1.jpg')

      // Add second file
      wrapper.vm.addFiles([mockFile2])
      expect(wrapper.vm.selectedFiles.length).toBe(1)
      expect(wrapper.vm.selectedFiles[0].name).toBe('file2.jpg')
    })

    it('appends files when multiple is true', () => {
      const wrapper = createWrapper({ multiple: true })
      const mockFile1 = createMockFile('file1.jpg')
      const mockFile2 = createMockFile('file2.jpg')

      // Add first file
      wrapper.vm.addFiles([mockFile1])
      expect(wrapper.vm.selectedFiles.length).toBe(1)
      expect(wrapper.vm.selectedFiles[0].name).toBe('file1.jpg')

      // Add second file
      wrapper.vm.addFiles([mockFile2])
      expect(wrapper.vm.selectedFiles.length).toBe(2)
      expect(wrapper.vm.selectedFiles[0].name).toBe('file1.jpg')
      expect(wrapper.vm.selectedFiles[1].name).toBe('file2.jpg')
    })

    it('adds files when input changes', async () => {
      const wrapper = createWrapper()
      const mockFile = createMockFile()

      // Create a mock change event
      const input = wrapper.find('.file-input').element as HTMLInputElement
      Object.defineProperty(input, 'files', {
        value: [mockFile]
      })

      // Spy on the addFiles method
      const addFilesSpy = vi.spyOn(wrapper.vm, 'addFiles')

      await wrapper.find('.file-input').trigger('change')

      // Check if files were added
      expect(wrapper.vm.selectedFiles.length).toBeGreaterThan(0)
    })
  })

  // Upload tests
  describe('File Upload', () => {
    it('emits upload event with selected files', async () => {
      const wrapper = createWrapper()
      const mockFile = createMockFile()

      // Add a file
      wrapper.vm.addFiles([mockFile])
      await wrapper.vm.$nextTick()

      // Click upload button
      await wrapper.find('.btn-primary').trigger('click')

      // Check if upload event was emitted with the file
      expect(wrapper.emitted('upload')).toBeTruthy()
      expect(wrapper.emitted('upload')![0][0]).toEqual([mockFile])
    })

    it('clears selected files after upload', async () => {
      const wrapper = createWrapper()
      const mockFile = createMockFile()

      // Add a file
      wrapper.vm.addFiles([mockFile])
      await wrapper.vm.$nextTick()

      // Check if the file was added
      expect(wrapper.vm.selectedFiles.length).toBe(1)

      // Click upload button
      await wrapper.find('.btn-primary').trigger('click')

      // Check if selectedFiles was cleared
      expect(wrapper.vm.selectedFiles.length).toBe(0)
      expect(wrapper.find('.selected-files').exists()).toBe(false)
    })

    it('emits upload event when upload button is clicked', async () => {
      const wrapper = createWrapper()
      const mockFile = createMockFile()

      // Add a file
      wrapper.vm.addFiles([mockFile])
      await wrapper.vm.$nextTick()

      // Start upload
      await wrapper.find('.btn-primary').trigger('click')

      // Check that upload event was emitted
      expect(wrapper.emitted('upload')).toBeTruthy()
      expect(wrapper.emitted('upload')![0][0]).toEqual([mockFile])

      // Check that selectedFiles was cleared
      expect(wrapper.vm.selectedFiles.length).toBe(0)
    })

    it('resets isUploading after upload', async () => {
      const wrapper = createWrapper()
      const mockFile = createMockFile()

      // Add a file
      wrapper.vm.addFiles([mockFile])
      await wrapper.vm.$nextTick()

      // Set isUploading to true
      wrapper.vm.isUploading = true

      // Start upload
      await wrapper.find('.btn-primary').trigger('click')

      // Check if isUploading was reset
      expect(wrapper.vm.isUploading).toBe(false)
    })

    it('does nothing when trying to upload with no files', async () => {
      const wrapper = createWrapper()

      // Try to upload with no files
      await wrapper.vm.uploadFiles()

      // Check that nothing happened
      expect(wrapper.emitted('upload')).toBeFalsy()
    })
  })

  // Edge cases
  describe('Edge Cases', () => {
    it('handles drop event with no files', async () => {
      const wrapper = createWrapper()

      // Create a mock drop event with no files
      const dropEvent = createDragEvent('drop')

      // Trigger the drop event
      await wrapper.find('.upload-area').element.dispatchEvent(dropEvent)

      // Check that no files were added
      expect(wrapper.vm.selectedFiles.length).toBe(0)
    })

    it('handles change event with no files', async () => {
      const wrapper = createWrapper()

      // Create a mock change event with no files
      const input = wrapper.find('.file-input').element as HTMLInputElement
      Object.defineProperty(input, 'files', {
        value: []
      })

      await wrapper.find('.file-input').trigger('change')

      // Check that no files were added
      expect(wrapper.vm.selectedFiles.length).toBe(0)
    })

    it('handles null fileInput ref', () => {
      const wrapper = createWrapper()

      // Set fileInput to null
      wrapper.vm.fileInput = null

      // This should not throw an error
      expect(() => wrapper.vm.triggerFileInput()).not.toThrow()
    })
  })
})
